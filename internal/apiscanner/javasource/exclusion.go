package javasource

import (
	"strings"
)

// matchPattern returns true if name matches the given glob-like pattern.
// Supported wildcards:
//
//	*  matches any sequence of characters (but not '/')
//	** matches any sequence including '/'
//
// Patterns without wildcards are tested as exact matches.
// Leading '*' on controller patterns matches any suffix (e.g. *HealthCheckController).
func matchPattern(pattern, name string) bool {
	if pattern == "" {
		return false
	}
	// Exact match
	if !strings.Contains(pattern, "*") {
		return pattern == name
	}
	return globMatch(pattern, name)
}

// globMatch implements simple glob matching with * and **.
func globMatch(pattern, str string) bool {
	p, s := 0, 0
	starP := -1
	starS := 0

	for s < len(str) {
		if p < len(pattern) && (pattern[p] == str[s] || pattern[p] == '?') {
			p++
			s++
		} else if p < len(pattern) && pattern[p] == '*' {
			// Is it **?
			if p+1 < len(pattern) && pattern[p+1] == '*' {
				// ** matches everything including /
				starP = p
				starS = s
				p += 2
				if p < len(pattern) && pattern[p] == '/' {
					p++
				}
			} else {
				// * matches anything except /
				starP = p
				starS = s
				p++
			}
		} else if starP >= 0 {
			// Backtrack
			starS++
			s = starS
			p = starP + 1
		} else {
			return false
		}
	}

	// Consume trailing * or **
	for p < len(pattern) && (pattern[p] == '*') {
		p++
	}
	return p == len(pattern)
}

// MatchPatternExported is the exported wrapper for matchPattern (used in tests).
func MatchPatternExported(pattern, name string) bool { return matchPattern(pattern, name) }

// matchAny returns true if name matches any pattern in the list.
func matchAny(patterns []string, name string) bool {
	for _, pat := range patterns {
		if matchPattern(pat, name) {
			return true
		}
	}
	return false
}
