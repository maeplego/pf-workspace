package domain

import (
	"strings"
)

func ParseSearchTypes(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		out := make([]string, len(DefaultSearchTypes))
		copy(out, DefaultSearchTypes)
		return out, nil
	}
	allowed := map[string]bool{"page": true, "document": true, "board": true, "card": true, "channel": true, "message": true}
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		t := strings.ToLower(strings.TrimSpace(part))
		if t == "" {
			continue
		}
		if !allowed[t] {
			return nil, ErrInvalid
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, ErrInvalid
	}
	return out, nil
}

func ContainsFold(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func Snippet(text, needle string, width int) string {
	if width < 16 {
		width = 80
	}
	hay := []rune(text)
	if len(hay) == 0 {
		return ""
	}
	ned := []rune(strings.ToLower(needle))
	low := []rune(strings.ToLower(text))
	at := indexRunes(low, ned)
	if at < 0 {
		if len(hay) <= width {
			return text
		}
		return string(hay[:width]) + "…"
	}
	start := at - 20
	if start < 0 {
		start = 0
	}
	end := at + len(ned) + 40
	if end > len(hay) {
		end = len(hay)
	}
	s := string(hay[start:end])
	if start > 0 {
		s = "…" + s
	}
	if end < len(hay) {
		s = s + "…"
	}
	return s
}

func indexRunes(haystack, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
