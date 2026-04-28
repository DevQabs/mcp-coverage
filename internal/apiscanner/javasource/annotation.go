package javasource

import (
	"regexp"
	"strings"
)

// annotRaw holds a parsed annotation's name, raw content, and its end position.
type annotRaw struct {
	name    string
	content string // text between the parens, empty if no parens present
	lineNum int
	endPos  int // position in source after this annotation (including closing paren)
}

// httpMethodForAnnotation returns the fixed HTTP method for shorthand mapping
// annotations, or "" for @RequestMapping (method extracted from content).
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

	// Read annotation name (may contain dots for fully-qualified, we only care about simple name)
	start := pos
	for pos < len(src) && (src[pos] == '_' || src[pos] == '.' || isLetter(src[pos]) || isDigit(src[pos])) {
		pos++
	}
	fullName := src[start:pos]
	// Use simple name (after last dot)
	parts := strings.Split(fullName, ".")
	name := parts[len(parts)-1]

	// Skip horizontal whitespace only (preserve newlines for line counting)
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
// pos must point at the opening '('. Returns the inner content and the
// position after the closing ')'.
func extractBalancedParens(src string, pos int) (string, int) {
	if pos >= len(src) || src[pos] != '(' {
		return "", pos
	}
	pos++ // skip '('
	start := pos
	depth := 1
	for pos < len(src) && depth > 0 {
		switch src[pos] {
		case '(':
			depth++
		case ')':
			depth--
		case '"':
			pos++ // skip opening quote
			for pos < len(src) {
				if src[pos] == '\\' {
					pos++ // skip escape char
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
		pos++ // skip ')'
	}
	return content, pos
}

// extractPaths parses the annotation content and returns all path string literals.
// Handles:
//
//	@GetMapping("/path")           → ["/path"]
//	@GetMapping(value="/path")     → ["/path"]
//	@GetMapping({"/a","/b"})       → ["/a","/b"]
//	@GetMapping(value={"/a","/b"}) → ["/a","/b"]
//	@GetMapping                    → []  (no content)
func extractPaths(content string) []string {
	if content == "" {
		return nil
	}
	var paths []string
	inStr := false
	var cur strings.Builder

	for i := 0; i < len(content); i++ {
		ch := content[i]
		if inStr {
			if ch == '\\' && i+1 < len(content) {
				cur.WriteByte(ch)
				i++
				cur.WriteByte(content[i])
			} else if ch == '"' {
				paths = append(paths, cur.String())
				cur.Reset()
				inStr = false
			} else {
				cur.WriteByte(ch)
			}
		} else if ch == '"' {
			inStr = true
		}
	}
	return paths
}

var requestMethodRe = regexp.MustCompile(`(?i)method\s*=\s*(?:\{\s*)?(?:RequestMethod\.)?(\w+)`)

// extractHTTPMethod extracts method=RequestMethod.GET from @RequestMapping content.
// Defaults to GET if not specified.
func extractHTTPMethod(content string) string {
	m := requestMethodRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return "GET"
	}
	method := strings.ToUpper(m[1])
	// Validate it's a real HTTP method
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return method
	}
	return "GET"
}

// ── character helpers ──────────────────────────────────────────────────────

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '$'
}

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func isWordChar(ch byte) bool { return isLetter(ch) || isDigit(ch) }

func isWhitespace(ch byte) bool { return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' }
