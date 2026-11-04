package quality

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQualityInspectionOptionsAndStrictAnswers(t *testing.T) {
	for _, o := range []InspectionOptions{{Mode: "automatic", Checks: []string{"tools"}}, {Mode: "quick"}, {Mode: "full", Checks: []string{"tools", "tools"}}, {Mode: "quick", Checks: []string{"execute_code"}}} {
		require.Error(t, o.validate())
	}
	o := InspectionOptions{Mode: "full", Checks: []string{"context", "tools", "speed"}}
	require.NoError(t, o.validate())
	require.Equal(t, []string{"tools", "speed", "context"}, o.Checks)
	require.Equal(t, 5, inspectionCount("speed", "full"))
	require.Equal(t, 2, inspectionCount("speed", "quick"))
	require.True(t, inspectionObjectMatches(`{"value":7,"nonce":"x"}`, "x", 7))
	for _, raw := range []string{`{"value":7,"nonce":"x","extra":1}`, `{"value":"7","nonce":"x"}`, `{"value":7,"nonce":"y"}`, "```json\n{\"nonce\":\"x\",\"value\":7}\n```"} {
		require.False(t, inspectionObjectMatches(raw, "x", 7))
	}
	a := &service.Account{Credentials: map[string]any{"api_key": "one"}}
	old := inspectionIdentity(a)
	a.Credentials["api_key"] = "two"
	require.NotEqual(t, old, inspectionIdentity(a))
	require.NotContains(t, old, "one")
}

func TestQualityInspectionImageReferenceMatchesPixels(t *testing.T) {
	data, expected, err := inspectionImage()
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, "data:image/png;base64,"))
	require.NoError(t, err)
	img, err := png.Decode(bytes.NewReader(raw))
	require.NoError(t, err)
	red := 0
	for i := 0; i < 16; i++ {
		r, g, b, _ := img.At(30+(i%4)*76, 30+(i/4)*76).RGBA()
		if r > g && r > b {
			red++
		}
	}
	require.Equal(t, strconv.Itoa(red), expected)
	require.Greater(t, red, 0)
	require.Less(t, red, 16)
}

func TestQualityCustomInspectionValidationAndGrading(t *testing.T) {
	questions := []InspectionQuestion{
		{Name: "Exact text", Prompt: "Answer the question", Match: "exact", Expected: "Answer"},
		{Name: "Keywords", Prompt: "Explain a topic", Match: "contains", Expected: "alpha\n\nbeta"},
		{Name: "Open question", Prompt: "Write an essay", Match: "manual"},
	}
	o := InspectionOptions{Mode: "quick", Checks: []string{"custom"}, Questions: questions}
	require.NoError(t, o.validate())
	require.Equal(t, 3, o.count("custom"))
	for _, tc := range []struct {
		index  int
		answer string
		pass   bool
	}{
		{0, " Answer \n", true}, {0, "answer", false}, {0, "The Answer is", false},
		{1, "beta is linked to alpha", true}, {1, "alpha only", false}, {1, "ALPHA beta", false},
	} {
		pass, reason := gradeInspectionQuestion(questions[tc.index], tc.answer)
		require.NotNil(t, pass)
		require.Equal(t, tc.pass, *pass)
		require.Empty(t, reason)
	}
	pass, reason := gradeInspectionQuestion(questions[2], "Any essay")
	require.Nil(t, pass)
	require.Equal(t, "manual_review", reason)
	for _, edit := range []func(*InspectionOptions){
		func(o *InspectionOptions) { o.Questions = nil },
		func(o *InspectionOptions) { o.Checks = []string{"speed"} },
		func(o *InspectionOptions) { o.Questions[0].Name = " " },
		func(o *InspectionOptions) { o.Questions[0].Prompt = " " },
		func(o *InspectionOptions) { o.Questions[0].Match = "execute" },
		func(o *InspectionOptions) { o.Questions[0].Expected = " " },
		func(o *InspectionOptions) { o.Questions[0].Prompt = strings.Repeat("x", 4001) },
		func(o *InspectionOptions) { o.Questions[0].Expected = strings.Repeat("x", 1001) },
		func(o *InspectionOptions) { o.Questions = make([]InspectionQuestion, 9) },
		func(o *InspectionOptions) {
			o.Questions = []InspectionQuestion{{Name: "a", Prompt: strings.Repeat("中", 4000), Match: "manual", Expected: strings.Repeat("中", 1000)}, {Name: "b", Prompt: strings.Repeat("中", 4000), Match: "manual"}}
		},
	} {
		copy := o
		copy.Checks = append([]string{}, o.Checks...)
		copy.Questions = append([]InspectionQuestion{}, o.Questions...)
		edit(&copy)
		require.Error(t, copy.validate())
	}
	cfg := DefaultSettings()
	cfg.InspectionQuestions = questions
	require.NoError(t, cfg.Validate())
	a := &service.Account{}
	before := inspectionProfileHash(a, Target{}, cfg.Profile, nil, o)
	o.Questions[0].Expected = "Changed answer"
	require.NotEqual(t, before, inspectionProfileHash(a, Target{}, cfg.Profile, nil, o))
}
