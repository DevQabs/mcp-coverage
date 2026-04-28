package mapping

import (
	"strings"
	"unicode"
)

// similarity returns a score 0.0–1.0 between two identifiers.
// Uses token overlap after camelCase / snake_case splitting.
func similarity(a, b string) float64 {
	ta := tokenize(a)
	tb := tokenize(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(ta))
	for _, t := range ta {
		set[t] = struct{}{}
	}
	var shared int
	for _, t := range tb {
		if _, ok := set[t]; ok {
			shared++
		}
	}
	// Jaccard-like: shared / union
	union := len(ta) + len(tb) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

// tokenize splits an identifier into lowercase tokens.
// Handles: camelCase, PascalCase, snake_case, kebab-case, path segments.
func tokenize(s string) []string {
	// Replace separators with space.
	s = strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r == '/' || r == '.' {
			return ' '
		}
		return r
	}, s)

	// Split camelCase.
	var buf strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(runes[i-1]) {
			buf.WriteRune(' ')
		}
		buf.WriteRune(unicode.ToLower(r))
	}

	parts := strings.Fields(buf.String())
	// Remove stop words that add noise.
	stop := map[string]bool{
		"controller": true, "service": true, "handler": true,
		"api": true, "the": true, "a": true, "an": true,
	}
	var tokens []string
	for _, p := range parts {
		if p != "" && !stop[p] {
			tokens = append(tokens, p)
		}
	}
	return tokens
}

// bestToolMatch finds the MCP tool name with the highest similarity to the
// given API controller+methodName pair. Returns ("", 0) if no candidates.
func bestToolMatch(controller, method string, toolNames []string) (string, float64) {
	query := controller + " " + method
	best := ""
	var bestScore float64
	for _, name := range toolNames {
		s := similarity(query, name)
		if s > bestScore {
			bestScore = s
			best = name
		}
	}
	return best, bestScore
}
