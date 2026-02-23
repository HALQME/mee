package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ResultItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Action   string `json:"action"`
	Payload  string `json:"payload"`
	Score    int    `json:"score"`
}

type ResultSet struct {
	Items        []ResultItem `json:"items"`
	ProviderName string       `json:"provider_name"`
}

// Search handles RPN expressions
func Search(query string) string {
	// Check if query looks like RPN expression (numbers and operators separated by spaces)
	if !isRPN(query) {
		return ""
	}

	result, ok := evalRPN(query)
	if !ok {
		return ""
	}

	s := formatResult(result)

	rs := ResultSet{
		Items: []ResultItem{{
			ID:       "rpn:" + query,
			Title:    s,
			Subtitle: "= " + query,
			Action:   "copy",
			Payload:  s,
			Score:    100,
		}},
		ProviderName: "rpn",
	}
	data, _ := json.Marshal(rs)
	return string(data)
}

// isRPN checks if the query looks like an RPN expression
func isRPN(q string) bool {
	tokens := strings.Fields(q)
	if len(tokens) < 3 {
		return false
	}

	// Must have at least 2 numbers and 1 operator
	numCount := 0
	opCount := 0
	ops := map[string]bool{"+": true, "-": true, "*": true, "/": true, "%": true, "^": true}

	for _, t := range tokens {
		if _, err := strconv.ParseFloat(t, 64); err == nil {
			numCount++
		} else if ops[t] {
			opCount++
		} else {
			return false
		}
	}

	// RPN requires at least 2 numbers and at least 1 operator
	return numCount >= 2 && opCount >= 1
}

// evalRPN evaluates an RPN expression and returns the result
func evalRPN(expr string) (float64, bool) {
	tokens := strings.Fields(expr)
	stack := make([]float64, 0, len(tokens))

	ops := map[string]func(a, b float64) (float64, bool){
		"+": func(a, b float64) (float64, bool) { return a + b, true },
		"-": func(a, b float64) (float64, bool) { return a - b, true },
		"*": func(a, b float64) (float64, bool) { return a * b, true },
		"/": func(a, b float64) (float64, bool) {
			if b == 0 {
				return 0, false
			}
			return a / b, true
		},
		"%": func(a, b float64) (float64, bool) {
			if b == 0 {
				return 0, false
			}
			return float64(int64(a) % int64(b)), true
		},
		"^": func(a, b float64) (float64, bool) { return pow(a, b), true },
	}

	for _, token := range tokens {
		if op, isOp := ops[token]; isOp {
			if len(stack) < 2 {
				return 0, false
			}
			// Pop two operands (second pop is first operand)
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]

			result, ok := op(a, b)
			if !ok {
				return 0, false
			}
			stack = append(stack, result)
		} else {
			// Try to parse as number
			v, err := strconv.ParseFloat(token, 64)
			if err != nil {
				return 0, false
			}
			stack = append(stack, v)
		}
	}

	if len(stack) != 1 {
		return 0, false
	}
	return stack[0], true
}

// formatResult formats the result appropriately
func formatResult(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// pow calculates a raised to the power of b
func pow(a, b float64) float64 {
	result := 1.0
	for i := 0; i < int(b); i++ {
		result *= a
	}
	return result
}
