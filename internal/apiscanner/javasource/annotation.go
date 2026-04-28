package javasource

import (
	"regexp"
	"strings"
)

// annotRaw holds a parsed annotation's name, raw content, and end position.
type annotRaw struct {
	name    string
	content string // text between the parens, empty if no parens present
	lineNum int
	endPos  int // position in source after this annotation (including closing paren)
}

// httpMethodForAnnotation maps shorthand mapping annotations to their fixed HTTP method.
var httpMethodForAnnotation = map[string]string{
	"GetMapping":    "GET",
	"PostMapping":   "POST",
	"PutMapping":    "PUT",
	"DeleteMapping": "DELETE",
	"PatchMapping":  "PATCH",
}

var isMappingName = map[string]bool{
	"GetMapping": true, "PostMapping": true, "PutMapping": true,
	"DeleteMapping": true, "PatchMapping": true, "RequestMapping": true,
}

// parseAnnotation parses one annotation starting at pos (which points at '@').
// Returns the annotRaw and the next position after the annotation.
func parseAnnotation(src string, pos int) (*annotRaw, int) {
	if pos >= len(src) || src[pos] != '@' {
		return nil, pos
	}
	pos++ // skip '@'

	start := pos
	for pos < len(src) && (src[pos] == '_' || src[pos] == '.' || isLetter(src[pos]) || isDigit(src[pos])) {
		pos++
	}
	fullName := src[start:pos]
	parts := strings.Split(fullName, ".")
	name := parts[len(parts)-1]

	for pos < len(src) && (src[pos] == ' ' || src[pos] == '\t') {
		pos++
	}

	content := ""
	if pos < len(src) && src[pos] == '(' {
		content, pos = extractBalancedParens(src, pos)
	}

	return &annotRaw{name: name, content: content, endPos: pos}, pos
}

// extractBalancedParens extracts the content inside matching parentheses.
// pos must point at '('. Returns inner content and position after ')'.
func extractBalancedParens(src string, pos int) (string, int) {
	if pos >= len(src) || src[pos] != '(' {
		return "", pos
	}
	pos++
	start := pos
	depth := 1
	for pos < len(src) && depth > 0 {
		switch src[pos] {
		case '(':
			depth++
		case ')':
			depth--
		case '"':
			pos++
			for pos < len(src) {
				if src[pos] == '\\' {
					pos++
				} else if src[pos] == '"' {
					break
				}
				pos++
			}
		}
		if depth > 0 {
			pos++
		}
	}
	content := src[start:pos]
	if pos < len(src) {
		pos++
	}
	return content, pos
}

// ── path extraction ────────────────────────────────────────────────────────

// extractPaths parses annotation content and returns all path string literals.
// Handles:
//
//	@GetMapping("/path")                           → ["/path"]
//	@GetMapping(value="/path")                     → ["/path"]
//	@GetMapping(path="/path")                      → ["/path"]
//	@GetMapping({"/a","/b"})                       → ["/a","/b"]
//	@GetMapping(value={"/a","/b"})                 → ["/a","/b"]
//	@GetMapping(value="/a", produces="text/plain") → ["/a"] only
//
// Returns nil when content is empty or contains no string literals
// (e.g. constant reference — caller should use resolvePathsOrMark instead).
func extractPaths(content string) []string {
	if content == "" {
		return nil
	}
	if hasTopLevelEquals(content) {
		// Named attributes — extract value= or path= only, ignore produces/consumes/etc.
		if paths := extractNamedAttrStrings(content, "value"); len(paths) > 0 {
			return paths
		}
		return extractNamedAttrStrings(content, "path")
	}
	// Purely positional — all string literals are paths.
	return extractAllStrings(content)
}

// hasTopLevelEquals reports whether content contains '=' outside strings/braces.
func hasTopLevelEquals(content string) bool {
	inStr, depth := false, 0
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if inStr {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
		case '=':
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

// extractNamedAttrStrings finds `attr = <value>` in content and extracts string
// literals from just the value token (not subsequent attributes).
func extractNamedAttrStrings(content, attr string) []string {
	// Find attr= (with optional whitespace before =) outside strings.
	found := -1
	inStr := false
	for i := 0; i <= len(content)-len(attr); i++ {
		ch := content[i]
		if inStr {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inStr = false
			}
			continue
		}
		if ch == '"' {
			inStr = true
			continue
		}
		if content[i:i+len(attr)] == attr {
			j := i + len(attr)
			for j < len(content) && (content[j] == ' ' || content[j] == '\t') {
				j++
			}
			if j < len(content) && content[j] == '=' {
				found = j + 1
				break
			}
		}
	}
	if found < 0 {
		return nil
	}
	rest := strings.TrimSpace(content[found:])
	token := extractValueToken(rest)
	return extractAllStrings(token)
}

// extractValueToken returns just the first value token from s:
// a quoted string "...", an array {...}, or a bare token (identifier/constant).
// Stops at the next comma or ')' at depth 0.
func extractValueToken(s string) string {
	if s == "" {
		return ""
	}
	switch s[0] {
	case '"':
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				return s[:i+1]
			}
		}
		return s
	case '{':
		depth, inStr := 0, false
		for i := 0; i < len(s); i++ {
			ch := s[i]
			if inStr {
				if ch == '\\' {
					i++
					continue
				}
				if ch == '"' {
					inStr = false
				}
				continue
			}
			switch ch {
			case '"':
				inStr = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return s[:i+1]
				}
			}
		}
		return s
	default:
		for i := 0; i < len(s); i++ {
			if s[i] == ',' || s[i] == ')' {
				return s[:i]
			}
		}
		return s
	}
}

// extractAllStrings extracts all double-quoted string values from s.
func extractAllStrings(s string) []string {
	var result []string
	inStr := false
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inStr {
			if ch == '\\' && i+1 < len(s) {
				cur.WriteByte(ch)
				i++
				cur.WriteByte(s[i])
			} else if ch == '"' {
				result = append(result, cur.String())
				cur.Reset()
				inStr = false
			} else {
				cur.WriteByte(ch)
			}
		} else if ch == '"' {
			inStr = true
		}
	}
	return result
}

// ── HTTP method extraction ─────────────────────────────────────────────────

// requestMethodsRe matches method= with a single value or {}-delimited list.
var requestMethodsRe = regexp.MustCompile(
	`\bmethod\s*=\s*(?:\{([^}]*)\}|([A-Za-z][A-Za-z0-9._]*))`)

// extractHTTPMethods returns the HTTP methods declared in @RequestMapping content.
// Handles:
//
//	method = RequestMethod.GET              → ["GET"]
//	method = {RequestMethod.GET, RequestMethod.POST} → ["GET","POST"]
//
// Defaults to ["GET"] when no method= attribute is present.
func extractHTTPMethods(content string) []string {
	m := requestMethodsRe.FindStringSubmatch(content)
	if m == nil {
		return []string{"GET"}
	}
	raw := m[1]
	if raw == "" {
		raw = m[2]
	}
	var methods []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if idx := strings.LastIndex(part, "."); idx >= 0 {
			part = part[idx+1:]
		}
		part = strings.ToUpper(strings.TrimSpace(part))
		switch part {
		case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
			methods = append(methods, part)
		}
	}
	if len(methods) == 0 {
		return []string{"GET"}
	}
	return methods
}

// ── character helpers ──────────────────────────────────────────────────────

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '$'
}

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func isWordChar(ch byte) bool { return isLetter(ch) || isDigit(ch) }

func isWhitespace(ch byte) bool { return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' }
