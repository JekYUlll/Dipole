package ai

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// DetectAssistantMention reports whether content contains an @-mention of the
// assistant. Group messages have no structured mention field, so Route A · A2
// treats the nickname (and optional extra tokens such as the assistant UUID)
// as the trigger. Matching is case-insensitive, collapses internal whitespace
// in both the haystack and the token, accepts a compact form of multi-word
// nicknames (`@DipoleAI` for `Dipole AI`), and requires a non-word boundary
// after the token so `@Dipole` does not fire for nickname `Dipole AI`.
func DetectAssistantMention(content, nickname string, extraTokens ...string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	folded := foldMentionText(content)
	for _, token := range mentionTokens(nickname, extraTokens...) {
		if token != "" && hasBoundedAtMention(folded, token) {
			return true
		}
	}
	return false
}

func mentionTokens(nickname string, extraTokens ...string) []string {
	seen := make(map[string]struct{})
	var tokens []string
	add := func(raw string) {
		for _, candidate := range expandMentionToken(raw) {
			folded := foldMentionText(candidate)
			if folded == "" {
				continue
			}
			if _, exists := seen[folded]; exists {
				continue
			}
			seen[folded] = struct{}{}
			tokens = append(tokens, folded)
		}
	}
	add(nickname)
	for _, extra := range extraTokens {
		add(extra)
	}
	return tokens
}

func expandMentionToken(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.Fields(raw)
	out := []string{raw, strings.Join(fields, " "), strings.Join(fields, "")}
	return out
}

func foldMentionText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func hasBoundedAtMention(foldedContent, foldedToken string) bool {
	start := 0
	for start < len(foldedContent) {
		rel := strings.IndexByte(foldedContent[start:], '@')
		if rel < 0 {
			return false
		}
		at := start + rel
		if at > 0 {
			prev, _ := utf8.DecodeLastRuneInString(foldedContent[:at])
			if isMentionBody(prev) {
				start = at + 1
				continue
			}
		}
		rest := strings.TrimLeft(foldedContent[at+1:], " ")
		if strings.HasPrefix(rest, foldedToken) {
			after := rest[len(foldedToken):]
			if after == "" {
				return true
			}
			next, _ := utf8.DecodeRuneInString(after)
			if !isMentionBody(next) {
				return true
			}
		}
		start = at + 1
	}
	return false
}

func isMentionBody(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
