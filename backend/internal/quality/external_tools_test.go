package quality

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExternalToolsURLValidation(t *testing.T) {
	tool := ExternalTool{ID: "example", Name: "Example", URL: "https://example.com/ranking", Kind: "ranking", Families: []string{"gpt", "claude"}, Description: "Public ranking"}
	require.NoError(t, (ExternalTools{Items: []ExternalTool{tool}, Version: 1}).validate())
	for _, kind := range []string{"status", "pricing", "upstream", "payment", "email", "infrastructure", "operations", "marketplace", "creative"} {
		resource := tool
		resource.Kind = kind
		require.NoError(t, (ExternalTools{Items: []ExternalTool{resource}, Version: 1}).validate())
	}
	for _, value := range []string{"javascript:alert(1)", "data:text/html,unsafe", "//example.com", "https://user:password@example.com", "https://", "https://example.com\n"} {
		invalidTool := tool
		invalidTool.URL = value
		require.Error(t, (ExternalTools{Items: []ExternalTool{invalidTool}, Version: 1}).validate(), value)
	}
	require.Error(t, (ExternalTools{Items: []ExternalTool{tool, tool}, Version: 1}).validate())
	tool.Families = []string{"gpt", "gpt"}
	require.Error(t, (ExternalTools{Items: []ExternalTool{tool}, Version: 1}).validate())
}

func TestExternalResourceMetadataValidation(t *testing.T) {
	tool := ExternalTool{ID: "email", Name: "Email", URL: "https://example.com/", Kind: "email", Families: []string{}, Tags: []string{"SMTP"}, Notes: "Original note", Usage: "trial", Origin: "user", Review: "public", ReviewedAt: "2026-11-04", ReferenceURL: "https://example.com/docs"}
	validate := func(resource ExternalTool) error {
		return (ExternalTools{Items: []ExternalTool{resource}, Version: 1}).validate()
	}
	require.NoError(t, validate(tool))
	for name, mutate := range map[string]func(*ExternalTool){
		"unknown kind":             func(r *ExternalTool) { r.Kind = "unknown" },
		"unknown family":           func(r *ExternalTool) { r.Families = []string{"unknown"} },
		"unsafe reference":         func(r *ExternalTool) { r.ReferenceURL = "javascript:alert(1)" },
		"credentials in reference": func(r *ExternalTool) { r.ReferenceURL = "https://user:password@example.com" },
		"invalid review date":      func(r *ExternalTool) { r.ReviewedAt = "2026-02-30" },
		"unknown review":           func(r *ExternalTool) { r.Review = "certified" },
		"unknown origin":           func(r *ExternalTool) { r.Origin = "endorsed" },
		"unknown usage":            func(r *ExternalTool) { r.Usage = "unknown" },
		"duplicate tags":           func(r *ExternalTool) { r.Tags = []string{"SMTP", "SMTP"} },
		"blank tag":                func(r *ExternalTool) { r.Tags = []string{" "} },
		"too many tags":            func(r *ExternalTool) { r.Tags = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"} },
	} {
		t.Run(name, func(t *testing.T) { candidate := tool; mutate(&candidate); require.Error(t, validate(candidate)) })
	}
}
