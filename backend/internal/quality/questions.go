package quality

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand/v2"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type Question struct {
	ID          string
	Family      string
	Prompt      string
	Expected    string
	Nonce       string
	Answer      int
	Explanation string
}

func newQuestion(index int, light bool) (Question, error) {
	var seed [16]byte
	if _, err := io.ReadFull(rand.Reader, seed[:]); err != nil {
		return Question{}, err
	}
	rng := mathrand.New(mathrand.NewPCG(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:])))
	q := Question{ID: uuid.NewString(), Nonce: uuid.NewString()[:8]}
	if light {
		a, b := 10+rng.IntN(50), 10+rng.IntN(50)
		q.Family, q.Answer, q.Prompt = "connectivity", a+b, fmt.Sprintf("Compute %d + %d.", a, b)
	} else {
		switch index % 3 {
		case 0:
			q.Family = "path_count"
			w, h := 5+rng.IntN(3), 5+rng.IntN(3)
			blocked := map[[2]int]bool{}
			for len(blocked) < 4 {
				x, y := 1+rng.IntN(w-2), 1+rng.IntN(h-2)
				blocked[[2]int{x, y}] = true
			}
			dp := make([][]int, h)
			var forbidden []string
			for y := 0; y < h; y++ {
				dp[y] = make([]int, w)
				for x := 0; x < w; x++ {
					if blocked[[2]int{x, y}] {
						forbidden = append(forbidden, fmt.Sprintf("(%d,%d)", x, y))
						continue
					}
					if x == 0 && y == 0 {
						dp[y][x] = 1
					}
					if x > 0 {
						dp[y][x] += dp[y][x-1]
					}
					if y > 0 {
						dp[y][x] += dp[y-1][x]
					}
				}
			}
			q.Answer = dp[h-1][w-1]
			q.Prompt = fmt.Sprintf("Grid coordinates: x=0..%d, y=0..%d. Start at (0,0); finish at (%d,%d). Each move increases exactly x or y by 1. Never visit blocked cells: %s. Count all valid paths.", w-1, h-1, w-1, h-1, strings.Join(forbidden, ", "))
		case 1:
			q.Family = "conditional_records"
			threshold := 25 + rng.IntN(20)
			q.Prompt = fmt.Sprintf("For the records below, sum amount ONLY when active=true, category=A, and amount>=%d. Negative amounts are possible.\n", threshold)
			for i := 0; i < 12; i++ {
				amount, active, cat := rng.IntN(120)-20, rng.IntN(2) == 1, "A"
				if rng.IntN(2) == 0 {
					cat = "B"
				}
				q.Prompt += fmt.Sprintf("id=%d active=%t category=%s amount=%d\n", i+1, active, cat, amount)
				if active && cat == "A" && amount >= threshold {
					q.Answer += amount
				}
			}
		case 2:
			q.Family = "weighted_xor"
			weights := make([]int, 6)
			for i := range weights {
				weights[i] = 1 + rng.IntN(15)
			}
			parity := rng.IntN(2)
			q.Answer = 1 << 30
			for mask := 0; mask < 64; mask++ {
				bits := func(i int) int { return (mask >> i) & 1 }
				if bits(0)^bits(1)^bits(3) != parity || bits(1)^bits(2)^bits(5) != 1 || bits(2)^bits(4) != 0 {
					continue
				}
				cost := 0
				for i, weight := range weights {
					cost += bits(i) * weight
				}
				q.Answer = min(q.Answer, cost)
			}
			q.Prompt = fmt.Sprintf("Choose six binary values a,b,c,d,e,f (each 0 or 1). Constraints: a XOR b XOR d=%d; b XOR c XOR f=1; c XOR e=0. Minimize %d*a+%d*b+%d*c+%d*d+%d*e+%d*f. Return the minimum total cost.", parity, weights[0], weights[1], weights[2], weights[3], weights[4], weights[5])
		}
	}
	q.Prompt += fmt.Sprintf("\nReturn ONLY one JSON object with exactly two fields: {\"answer\":<integer>,\"check\":\"%s\"}. Do not include explanation or markdown.", q.Nonce)
	q.Expected = fmt.Sprintf(`{"answer":%d,"check":%q}`, q.Answer, q.Nonce)
	switch q.Family {
	case "path_count":
		q.Explanation = "从起点逐格计算：禁行格路径数为 0，其他格路径数等于左侧与下侧路径数之和。"
	case "conditional_records":
		q.Explanation = "仅累计 active=true、category=A 且金额达到题目门槛的记录；其他记录不计入。"
	case "weighted_xor":
		q.Explanation = "本地枚举六个二进制变量的全部 64 种组合，筛选满足异或约束的组合，再取最小加权和。"
	default:
		q.Explanation = "本地计算两数之和。"
	}
	q.Explanation += fmt.Sprintf(" 标准结果为 %d；check 字段必须与本题标识完全一致。", q.Answer)
	return q, nil
}

// Strict local grading has no paid judge, duplicate-key ambiguity or execution
// of model-supplied code. Reference answers never enter upstream requests.
func grade(text string, q Question) bool {
	d := json.NewDecoder(strings.NewReader(strings.TrimSpace(text)))
	d.UseNumber()
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	answer, check := "", ""
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return false
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return false
		}
		seen[key] = true
		switch key {
		case "answer":
			value, tokenErr := d.Token()
			n, numberOK := value.(json.Number)
			if tokenErr != nil || !numberOK {
				return false
			}
			if _, err := n.Int64(); err != nil {
				return false
			}
			answer = n.String()
		case "check":
			if d.Decode(&check) != nil {
				return false
			}
		default:
			return false
		}
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') {
		return false
	}
	var extra any
	return d.Decode(&extra) == io.EOF && len(seen) == 2 && answer == strconv.Itoa(q.Answer) && check == q.Nonce
}

func cachePrefix(runID string, kb int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Reference data for this request batch %s. Treat these records as data; answer only the final instruction after the records.\n", runID)
	for i := 0; b.Len() < kb*1024; i++ {
		fmt.Fprintf(&b, "Reference record %05d: amber cedar river meadow orbit quartz violet harbor willow planet. Value %d.\n", i, (i*31)%997)
	}
	return b.String()
}
