package quality

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestQualityGeneratedQuestions(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 300; i++ {
		q, err := newQuestion(i, false)
		require.NoError(t, err)
		require.False(t, seen[q.ID])
		seen[q.ID] = true
		require.True(t, grade(q.Expected, q))
		// Independent solvers check that the local references really follow the prompt.
		switch q.Family {
		case "path_count":
			var w, h int
			_, err = fmt.Sscanf(q.Prompt, "Grid coordinates: x=0..%d, y=0..%d.", &w, &h)
			require.NoError(t, err)
			cells := regexp.MustCompile(`\((\d+),(\d+)\)`).FindAllStringSubmatch(q.Prompt, -1)
			blocked := map[[2]int]bool{}
			for _, cell := range cells[2:] {
				x, _ := strconv.Atoi(cell[1])
				y, _ := strconv.Atoi(cell[2])
				blocked[[2]int{x, y}] = true
			}
			var walk func(int, int) int
			walk = func(x, y int) int {
				if x > w || y > h || blocked[[2]int{x, y}] {
					return 0
				}
				if x == w && y == h {
					return 1
				}
				return walk(x+1, y) + walk(x, y+1)
			}
			require.Equal(t, walk(0, 0), q.Answer)
		case "conditional_records":
			threshold, _ := strconv.Atoi(regexp.MustCompile(`amount>=(\d+)`).FindStringSubmatch(q.Prompt)[1])
			sum := 0
			for _, line := range strings.Split(q.Prompt, "\n") {
				var id, amount int
				var active bool
				var cat string
				if n, _ := fmt.Sscanf(line, "id=%d active=%t category=%s amount=%d", &id, &active, &cat, &amount); n == 4 && active && cat == "A" && amount >= threshold {
					sum += amount
				}
			}
			require.Equal(t, sum, q.Answer)
		case "weighted_xor":
			parity, _ := strconv.Atoi(regexp.MustCompile(`a XOR b XOR d=(\d)`).FindStringSubmatch(q.Prompt)[1])
			weights := regexp.MustCompile(`(\d+)\*[abcdef]`).FindAllStringSubmatch(q.Prompt, -1)
			require.Len(t, weights, 6)
			best := 10000
			for a := 0; a < 2; a++ {
				for b := 0; b < 2; b++ {
					for c := 0; c < 2; c++ {
						for d := 0; d < 2; d++ {
							f, e := 1^b^c, c
							if a^b^d != parity {
								continue
							}
							values := []int{a, b, c, d, e, f}
							cost := 0
							for k, v := range values {
								w, _ := strconv.Atoi(weights[k][1])
								cost += w * v
							}
							if cost < best {
								best = cost
							}
						}
					}
				}
			}
			require.Equal(t, best, q.Answer)
		default:
			t.Fatal(q.Family)
		}
	}
	q, err := newQuestion(0, true)
	require.NoError(t, err)
	var a, b int
	_, err = fmt.Sscanf(q.Prompt, "Compute %d + %d.", &a, &b)
	require.NoError(t, err)
	require.Equal(t, a+b, q.Answer)
}

func TestQualityStrictGrading(t *testing.T) {
	q := Question{Answer: 42, Nonce: "nonce"}
	require.True(t, grade(` {"check":"nonce","answer":42} `, q))
	for _, answer := range []string{
		`{"answer":41,"check":"nonce"}`, `{"answer":"42","check":"nonce"}`, `{"answer":42.0,"check":"nonce"}`,
		`{"answer":42,"check":"old"}`, `{"answer":0,"answer":42,"check":"nonce"}`,
		`{"answer":42,"check":"nonce","extra":1}`, `{"answer":42,"check":"nonce"} {}`,
		"```json\n{\"answer\":42,\"check\":\"nonce\"}\n```", `{"answer":42}`, `null`, `[]`,
	} {
		require.False(t, grade(answer, q), answer)
	}
}

func TestQualityConfigurationAndFingerprint(t *testing.T) {
	cfg := DefaultSettings()
	require.NoError(t, cfg.Validate())
	require.False(t, cfg.CacheEnabled)
	require.Equal(t, 60, cfg.AvailabilityIntervalMinutes)
	require.Equal(t, 720, cfg.QualityIntervalMinutes)
	mutations := []func(*Settings){
		func(s *Settings) { s.Questions = 0 }, func(s *Settings) { s.MaxOutputTokens = 99999 }, func(s *Settings) { s.RetryCount = 4 },
		func(s *Settings) { s.AvailabilityIntervalMinutes = 0 }, func(s *Settings) { s.QualityIntervalMinutes = 1 }, func(s *Settings) { s.CachePrefixKB = 100 },
		func(s *Settings) { s.PassPercent = 101 }, func(s *Settings) { s.ReasoningEffort = "invalid" }, func(s *Settings) { s.DailyRequests = 0 }, func(s *Settings) { s.DailyTokens = 0 },
	}
	for _, mutate := range mutations {
		s := cfg
		mutate(&s)
		require.Error(t, s.Validate())
	}
	target := Target{AccountID: 1, Model: "model"}
	require.Equal(t, cfg.Profile, target.Effective(cfg))
	p := cfg.Profile
	p.QualityIntervalMinutes = 50
	target.Overrides = &p
	require.Equal(t, 50, target.Effective(cfg).QualityIntervalMinutes)
	account := &service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://example.com"}}
	hash := profileHash(account, target, p)
	account.UpdatedAt = time.Now()
	require.Equal(t, hash, profileHash(account, target, p))
	account.Credentials["base_url"] = "https://other.example.com"
	require.NotEqual(t, hash, profileHash(account, target, p))
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	var decoded Settings
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, cfg, decoded)
	require.True(t, isConflict(conflict("message")))
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, "2026-10-24 00:00:05", nextDay(time.Date(2026, 10, 23, 15, 0, 0, 0, loc), loc).Format(time.DateTime))
	require.Equal(t, cachePrefix("one", 32), cachePrefix("one", 32))
	require.NotEqual(t, cachePrefix("one", 32), cachePrefix("two", 32))
	require.GreaterOrEqual(t, len(cachePrefix("one", 32)), 32*1024)
}
