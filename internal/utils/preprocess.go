package utils

import (
	"strings"
	"unicode"
)

// PreprocessLite performs lightweight, loss-minimizing cleanup for RP context.
// It removes standalone filler lines, trims whitespace, collapses repeats, and
// keeps the original content order without summarization.
func PreprocessLite(text string) string {
	if text == "" {
		return ""
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	prevLine := ""
	lastWasBlank := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !lastWasBlank && len(out) > 0 {
				out = append(out, "")
				lastWasBlank = true
			}
			continue
		}

		if isMeaninglessLine(line) {
			continue
		}

		if line == prevLine {
			continue
		}

		out = append(out, line)
		prevLine = line
		lastWasBlank = false
	}

	// Trim trailing blank line.
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}

	return strings.Join(out, "\n")
}

func isMeaninglessLine(line string) bool {
	s := strings.TrimSpace(strings.ToLower(line))
	if s == "" {
		return true
	}

	compact := stripNoise(s)
	if compact == "" {
		return true
	}

	fillers := map[string]bool{
		"嗯": true, "嗯嗯": true, "啊": true, "哦": true, "呃": true, "唔": true,
		"哈哈": true, "哈哈哈": true, "呵呵": true, "嘿嘿": true, "嘻嘻": true,
		"ok": true, "okay": true, "okey": true, "yes": true, "no": true, "yeah": true,
		"yep": true, "nope": true, "thanks": true, "thankyou": true, "thx": true,
		"👍": true, "👌": true, "😊": true, "😄": true,
	}
	if fillers[compact] {
		return true
	}

	if isRepeatedRune(compact, "哈") || isRepeatedRune(compact, "呵") || isRepeatedRune(compact, "嗯") ||
		isRepeatedRune(compact, "啊") || isRepeatedRune(compact, "哦") || isRepeatedRune(compact, "呃") {
		return true
	}

	if isOnlyRunes(compact, "ha") && len(compact) >= 4 {
		return true
	}
	if isOnlyRunes(compact, "lo") && len(compact) >= 3 {
		return true
	}

	return false
}

func stripNoise(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isRepeatedRune(s string, r string) bool {
	if s == "" {
		return false
	}
	rs := []rune(s)
	if len(rs) < 2 || string(rs[0]) != r {
		return false
	}
	for _, rr := range rs[1:] {
		if string(rr) != r {
			return false
		}
	}
	return true
}

func isOnlyRunes(s, allowed string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(allowed, r) {
			return false
		}
	}
	return true
}
