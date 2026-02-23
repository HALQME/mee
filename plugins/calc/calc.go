package main

import (
	"encoding/json"
	"fmt"
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

func Search(query string) string {
	if !isMath(query) {
		return ""
	}

	result, ok := eval(query)
	if !ok {
		return ""
	}

	s := fmt.Sprintf("%v", result)
	if result == float64(int64(result)) {
		s = fmt.Sprintf("%d", int64(result))
	}

	rs := ResultSet{
		Items: []ResultItem{{
			ID:       "calc:" + query,
			Title:    s,
			Subtitle: "= " + query,
			Action:   "copy",
			Payload:  s,
			Score:    100,
		}},
		ProviderName: "calc",
	}
	data, _ := json.Marshal(rs)
	return string(data)
}

func isMath(q string) bool {
	hasDigit, hasOp := false, false
	for _, c := range q {
		switch {
		case c >= '0' && c <= '9':
			hasDigit = true
		case c == '+' || c == '-' || c == '*' || c == '/' || c == '%' || c == '^' || c == '(':
			hasOp = true
		case c == ' ' || c == '\t' || c == ')' || c == '.':
		default:
			return false
		}
	}
	return hasDigit && hasOp
}

type parser struct {
	in []rune
	p  int
}

func eval(s string) (float64, bool) {
	p := &parser{in: []rune(s)}
	v, ok := p.expr()
	if !ok || p.p < len(p.in) {
		return 0, false
	}
	return v, true
}

func (p *parser) expr() (float64, bool) {
	v, ok := p.term()
	if !ok {
		return 0, false
	}
	for {
		p.skip()
		switch p.peek() {
		case '+':
			p.p++
			r, ok := p.term()
			if !ok {
				return 0, false
			}
			v += r
		case '-':
			p.p++
			r, ok := p.term()
			if !ok {
				return 0, false
			}
			v -= r
		default:
			return v, true
		}
	}
}

func (p *parser) term() (float64, bool) {
	v, ok := p.factor()
	if !ok {
		return 0, false
	}
	for {
		p.skip()
		switch p.peek() {
		case '*':
			p.p++
			r, ok := p.factor()
			if !ok {
				return 0, false
			}
			v *= r
		case '/':
			p.p++
			r, ok := p.factor()
			if !ok || r == 0 {
				return 0, false
			}
			v /= r
		case '%':
			p.p++
			r, ok := p.factor()
			if !ok || r == 0 {
				return 0, false
			}
			v = float64(int64(v) % int64(r))
		default:
			return v, true
		}
	}
}

func (p *parser) factor() (float64, bool) {
	v, ok := p.unary()
	if !ok {
		return 0, false
	}
	for {
		p.skip()
		if p.peek() == '^' {
			p.p++
			r, ok := p.unary()
			if !ok {
				return 0, false
			}
			v = pow(v, r)
		} else {
			return v, true
		}
	}
}

func (p *parser) unary() (float64, bool) {
	p.skip()
	if p.peek() == '+' {
		p.p++
		return p.primary()
	}
	if p.peek() == '-' {
		p.p++
		v, ok := p.primary()
		if !ok {
			return 0, false
		}
		return -v, true
	}
	return p.primary()
}

func (p *parser) primary() (float64, bool) {
	p.skip()
	if p.peek() == '(' {
		p.p++
		v, ok := p.expr()
		if !ok {
			return 0, false
		}
		p.skip()
		if p.peek() != ')' {
			return 0, false
		}
		p.p++
		return v, true
	}
	return p.number()
}

func (p *parser) number() (float64, bool) {
	p.skip()
	var buf []rune
	for p.p < len(p.in) {
		c := p.in[p.p]
		if c >= '0' && c <= '9' || c == '.' {
			buf = append(buf, c)
			p.p++
		} else {
			break
		}
	}
	if len(buf) == 0 {
		return 0, false
	}
	var v float64
	fmt.Sscanf(string(buf), "%f", &v)
	return v, true
}

func (p *parser) peek() rune {
	if p.p >= len(p.in) {
		return 0
	}
	return p.in[p.p]
}

func (p *parser) skip() {
	for p.p < len(p.in) && (p.in[p.p] == ' ' || p.in[p.p] == '\t') {
		p.p++
	}
}

func pow(b, e float64) float64 {
	v := 1.0
	for i := 0; i < int(e); i++ {
		v *= b
	}
	return v
}
