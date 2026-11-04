package service

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// Only reached with the private quality context. No gateway payload is changed.
func applyChannelInspectionPayload(payload map[string]any, protocol string, o QualityProbeOptions) {
	payload["stream"] = !o.NonStream
	if o.NonStream {
		delete(payload, "stream_options")
	}
	schema := map[string]any{"type": "object", "properties": map[string]any{"nonce": map[string]any{"type": "string"}, "value": map[string]any{"type": "integer"}}, "required": []string{"nonce", "value"}, "additionalProperties": false}
	if o.ImageDataURL != "" {
		switch protocol {
		case "chat":
			payload["messages"] = []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": o.Prompt}, map[string]any{"type": "image_url", "image_url": map[string]string{"url": o.ImageDataURL}}}}}
		case "responses":
			payload["input"] = []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": o.Prompt}, map[string]any{"type": "input_image", "image_url": o.ImageDataURL}}}}
		case "anthropic":
			payload["messages"] = []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "image", "source": map[string]string{"type": "base64", "media_type": "image/png", "data": strings.TrimPrefix(o.ImageDataURL, "data:image/png;base64,")}}, map[string]any{"type": "text", "text": o.Prompt}}}}
		}
	}
	if o.Structured {
		format := map[string]any{"type": "json_schema", "name": "channel_probe", "strict": true, "schema": schema}
		switch protocol {
		case "chat":
			payload["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "channel_probe", "strict": true, "schema": schema}}
		case "responses":
			payload["text"] = map[string]any{"format": format}
		case "anthropic":
			payload["output_config"] = map[string]any{"format": map[string]any{"type": "json_schema", "schema": schema}}
		}
	}
	if !o.Tools && o.ToolCallID == "" {
		return
	}
	switch protocol {
	case "anthropic":
		payload["tools"] = []any{map[string]any{"name": "channel_probe", "description": "A local inspection tool; returns an opaque receipt.", "input_schema": schema}}
		if o.ToolCallID == "" {
			payload["tool_choice"] = map[string]any{"type": "tool", "name": "channel_probe"}
		} else {
			var args any
			_ = json.Unmarshal([]byte(o.ToolArguments), &args)
			payload["messages"] = []any{map[string]any{"role": "user", "content": o.Prompt}, map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": o.ToolCallID, "name": "channel_probe", "input": args}}}, map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": o.ToolCallID, "content": o.ToolResult}}}}
		}
	case "chat":
		function := map[string]any{"name": "channel_probe", "description": "A local inspection tool; returns an opaque receipt.", "parameters": schema}
		payload["tools"] = []any{map[string]any{"type": "function", "function": function}}
		if o.ToolCallID == "" {
			payload["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": "channel_probe"}}
		} else {
			payload["messages"] = []any{map[string]any{"role": "user", "content": o.Prompt}, map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"id": o.ToolCallID, "type": "function", "function": map[string]any{"name": "channel_probe", "arguments": o.ToolArguments}}}}, map[string]any{"role": "tool", "tool_call_id": o.ToolCallID, "content": o.ToolResult}}
		}
	case "responses":
		payload["tools"] = []any{map[string]any{"type": "function", "name": "channel_probe", "description": "A local inspection tool; returns an opaque receipt.", "parameters": schema, "strict": true}}
		if o.ToolCallID == "" {
			payload["tool_choice"] = map[string]any{"type": "function", "name": "channel_probe"}
		} else {
			payload["input"] = []any{map[string]any{"role": "user", "content": o.Prompt}, map[string]any{"type": "function_call", "call_id": o.ToolCallID, "name": "channel_probe", "arguments": o.ToolArguments}, map[string]any{"type": "function_call_output", "call_id": o.ToolCallID, "output": o.ToolResult}}
		}
	}
}

// Consume the original response using the same account transport and auth. This
// supports JSON and tool-only replies without weakening ordinary text probes.
func consumeChannelInspectionResponse(ctx context.Context, resp *http.Response) (bool, error) {
	p := qualityCapture(ctx)
	if p == nil || !p.options.Inspection {
		return false, nil
	}
	if p.options.NonStream {
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return true, err
		}
		if !gjson.ValidBytes(raw) {
			p.wireInvalid = true
			p.result.Reason = "protocol_invalid"
			return true, nil
		}
		v := gjson.ParseBytes(raw)
		switch p.result.Protocol {
		case "responses":
			p.observe(`data: {"type":"response.completed","response":` + string(raw) + `}`)
		case "chat":
			p.observe("data: " + string(raw))
		case "anthropic":
			p.observe(`data: {"type":"message_start","message":` + string(raw) + `}`)
			for _, block := range v.Get("content").Array() {
				if block.Get("type").String() == "text" {
					p.result.Text += block.Get("text").String()
				}
			}
			stop := v.Get("stop_reason").String()
			if stop != "" {
				delta, _ := json.Marshal(map[string]any{"type": "message_delta", "delta": map[string]string{"stop_reason": stop}})
				p.observe("data: " + string(delta))
				p.ended = true
			}
		}
		p.observeInspection(v)
		if !p.ended {
			p.result.Reason = "protocol_invalid"
		}
		// JSON response time is not a streaming first-token measurement.
		p.result.FirstTokenMs = nil
		return true, nil
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	// qualityProbeBody observes SSE while it is read, preserving token timing.
	for scanner.Scan() {
	}
	return true, scanner.Err()
}

func (p *qualityProbeCapture) observeInspection(v gjson.Result) {
	protocol := p.result.Protocol
	event := v.Get("type").String()
	known := false
	switch protocol {
	case "chat":
		known = v.Get("choices").IsArray()
	case "anthropic":
		known = strings.HasPrefix(event, "message_") || strings.HasPrefix(event, "content_block_") || event == "ping" || event == "message"
	case "responses":
		known = strings.HasPrefix(event, "response.") || v.Get("object").String() == "response"
	}
	if !p.options.NonStream {
		switch protocol {
		case "anthropic":
			if event == "message_start" {
				if p.inspectionStarted {
					p.wireInvalid = true
				}
				p.inspectionStarted = true
			}
			if strings.HasPrefix(event, "content_block_") || event == "message_delta" || event == "message_stop" {
				if !p.inspectionStarted || p.inspectionTerminal {
					p.wireInvalid = true
				}
			}
			if event == "message_stop" {
				p.inspectionTerminal = true
			}
		case "responses":
			if event == "response.created" || event == "response.in_progress" {
				p.inspectionStarted = true
			}
			if event == "response.output_text.delta" || event == "response.completed" || event == "response.done" {
				if !p.inspectionStarted || p.inspectionTerminal {
					p.wireInvalid = true
				}
			}
			if event == "response.completed" || event == "response.done" {
				p.inspectionTerminal = true
			}
		case "chat":
			for _, choice := range v.Get("choices").Array() {
				if p.inspectionTerminal && choice.Get("delta.content").String() != "" {
					p.wireInvalid = true
				}
				if choice.Get("finish_reason").String() != "" {
					p.inspectionTerminal = true
				}
			}
		}
	}
	if known {
		p.observedEvents++
	} else {
		p.wireInvalid = true
	}
	// Final nonstreaming responses carry complete tool arguments. Streaming
	// tool probes are not used; ordinary SSE checks still inspect framing.
	add := func(id, name, args string) {
		if id == "" || name == "" {
			return
		}
		for _, t := range p.result.ToolCalls {
			if t.ID == id {
				return
			}
		}
		p.result.ToolCalls = append(p.result.ToolCalls, QualityToolCall{ID: id, Name: name, Arguments: args})
	}
	for _, choice := range v.Get("choices").Array() {
		for _, tool := range choice.Get("message.tool_calls").Array() {
			add(tool.Get("id").String(), tool.Get("function.name").String(), tool.Get("function.arguments").String())
		}
	}
	for _, block := range v.Get("content").Array() {
		if block.Get("type").String() == "tool_use" {
			add(block.Get("id").String(), block.Get("name").String(), block.Get("input").Raw)
		}
	}
	for _, path := range []string{"output", "response.output"} {
		for _, tool := range v.Get(path).Array() {
			if tool.Get("type").String() == "function_call" {
				add(tool.Get("call_id").String(), tool.Get("name").String(), tool.Get("arguments").String())
			}
		}
	}
}
