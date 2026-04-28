package javasource

import (
	"regexp"
	"strings"
)

// ControllerDef holds all extracted information from one Spring controller file.
type ControllerDef struct {
	ClassName  string
	BasePaths  []string // from class-level @RequestMapping
	Methods    []MethodDef
	SourceFile string
}

// MethodDef is one handler method with its resolved HTTP method and relative paths.
type MethodDef struct {
	MethodName string
	HTTPMethod string
	Paths      []string // relative paths declared on the annotation
	LineNumber int
}

var (
	// controllerAnnotRe matches @RestController or @Controller (word boundary prevents
	// matching @ControllerAdvice or @RestControllerAdvice).
	controllerAnnotRe = regexp.MustCompile(`@(?:Rest)?Controller\b`)
	// adviceAnnotRe detects advice classes that should NOT be treated as controllers.
	adviceAnnotRe = regexp.MustCompile(`@(?:Rest)?ControllerAdvice\b`)
	// classRe finds the class declaration and captures the class name.
	classRe = regexp.MustCompile(`(?:public\s+|protected\s+|abstract\s+|final\s+)*class\s+(\w+)`)
	// methodNameRe finds the last non-keyword identifier before '(' in a method signature.
	methodNameRe = regexp.MustCompile(`(\w+)\s*\(`)
)

// ParseFile parses a single Java source file and returns a ControllerDef if the
// file is a Spring MVC controller. Returns nil if the file is not a controller.
func ParseFile(path, src string) *ControllerDef {
	clean := stripComments(src)

	if !isController(clean) {
		return nil
	}

	className, classPos := findClassDeclaration(clean)
	if classPos < 0 {
		return nil
	}

	basePaths := extractBasePaths(clean[:classPos])

	// Find the opening '{' of the class body.
	bodyStart := strings.Index(clean[classPos:], "{")
	if bodyStart < 0 {
		return nil
	}
	bodyStart += classPos + 1 // position just after '{'

	methods, lineOffset := extractMethods(clean, bodyStart, countLines(clean[:bodyStart]))

	_ = lineOffset
	return &ControllerDef{
		ClassName:  className,
		BasePaths:  basePaths,
		Methods:    methods,
		SourceFile: path,
	}
}

// ── controller detection ───────────────────────────────────────────────────

func isController(src string) bool {
	if adviceAnnotRe.MatchString(src) {
		return false
	}
	return controllerAnnotRe.MatchString(src)
}

// ── class declaration ──────────────────────────────────────────────────────

func findClassDeclaration(src string) (name string, pos int) {
	m := classRe.FindStringIndex(src)
	if m == nil {
		return "", -1
	}
	sub := classRe.FindStringSubmatch(src[m[0]:])
	if len(sub) < 2 {
		return "", -1
	}
	return sub[1], m[0]
}

// ── class-level @RequestMapping ────────────────────────────────────────────

func extractBasePaths(preClass string) []string {
	idx := strings.LastIndex(preClass, "@RequestMapping")
	if idx < 0 {
		return nil
	}
	a, _ := parseAnnotation(preClass, idx)
	if a == nil {
		return nil
	}
	paths := extractPaths(a.content)
	if len(paths) == 0 && a.content == "" {
		return nil
	}
	if len(paths) == 0 {
		// content present but no string literal — might be just whitespace/empty parens
		return nil
	}
	return paths
}

// ── method-level annotations ───────────────────────────────────────────────

type pendingAnnot struct {
	annot   *annotRaw
	lineNum int
}

// extractMethods scans the class body (starting at bodyStart, which is the
// position right after the class's opening '{') and collects all handler methods.
func extractMethods(src string, bodyStart int, classLineOffset int) ([]MethodDef, int) {
	var methods []MethodDef

	pos := bodyStart
	braceDepth := 1 // we are inside the class body (depth 1)
	var pending []pendingAnnot
	lineNum := classLineOffset

	for pos < len(src) {
		ch := src[pos]

		switch {
		// ── opening brace ──────────────────────────────────────────────
		case ch == '{':
			braceDepth++
			if braceDepth == 2 && len(pending) > 0 {
				// Entering a method body. Find method name in the text between
				// the last annotation end and this brace.
				lastAnnotEnd := pending[len(pending)-1].annot.endPos
				sig := src[lastAnnotEnd:pos]
				name := findMethodName(sig)
				if name != "" {
					firstAnnotLine := pending[0].lineNum
					m := buildMethodDefs(pending, name, firstAnnotLine)
					methods = append(methods, m...)
				}
				pending = nil
			} else if braceDepth == 2 {
				// Inner class, enum, or block without preceding mapping annotation.
				pending = nil
			}
			// Skip the entire method/inner-class body by jumping to its matching '}'.
			if braceDepth > 1 {
				pos = skipBody(src, pos+1, braceDepth-1)
				braceDepth = 1
			} else {
				pos++
			}

		// ── closing brace ──────────────────────────────────────────────
		case ch == '}':
			braceDepth--
			pos++
			if braceDepth == 0 {
				return methods, lineNum
			}

		// ── annotation ─────────────────────────────────────────────────
		case ch == '@' && braceDepth == 1:
			a, newPos := parseAnnotation(src, pos)
			if a != nil && isMappingName[a.name] {
				pending = append(pending, pendingAnnot{annot: a, lineNum: lineNum + countLines(src[bodyStart:pos])})
			} else if a == nil {
				newPos = pos + 1
			}
			// Non-mapping annotations (@Valid, @PreAuthorize, etc.) are skipped
			// but do NOT clear pending — they may appear between @GetMapping and
			// the actual method signature.
			pos = newPos

		// ── semicolon clears pending (field declaration) ────────────────
		case ch == ';' && braceDepth == 1:
			pending = nil
			pos++

		// ── newline: count for line number tracking ────────────────────
		case ch == '\n':
			lineNum++
			pos++

		default:
			pos++
		}
	}
	return methods, lineNum
}

// skipBody advances pos past a block body, starting at the position AFTER
// the opening brace. depth is the current depth (caller has already seen one '{').
func skipBody(src string, pos int, depth int) int {
	for pos < len(src) && depth > 0 {
		switch src[pos] {
		case '{':
			depth++
		case '}':
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
		pos++
	}
	return pos
}

// buildMethodDefs creates one MethodDef per HTTP method × path combination.
func buildMethodDefs(pending []pendingAnnot, methodName string, lineNum int) []MethodDef {
	var defs []MethodDef
	for _, pa := range pending {
		a := pa.annot
		httpMethod, ok := httpMethodForAnnotation[a.name]
		if !ok {
			httpMethod = extractHTTPMethod(a.content)
		}
		paths := extractPaths(a.content)
		defs = append(defs, MethodDef{
			MethodName: methodName,
			HTTPMethod: httpMethod,
			Paths:      paths,
			LineNumber: lineNum,
		})
	}
	return defs
}

// findMethodName finds the method name from the text between an annotation and
// the opening '{' of the method body. It looks for the last non-keyword
// identifier immediately before a '(' in the signature text.
func findMethodName(sig string) string {
	matches := methodNameRe.FindAllStringSubmatchIndex(sig, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		word := sig[m[2]:m[3]]
		if !javaKeywords[word] {
			return word
		}
	}
	return ""
}

// ── comment stripping ──────────────────────────────────────────────────────

type commentState int

const (
	csNormal       commentState = iota
	csString                    // inside "..."
	csChar                      // inside '.'
	csLineComment               // after //
	csBlockComment              // inside /* ... */
)

// stripComments removes Java // and /* */ comments while preserving newlines
// for accurate line number tracking.
func stripComments(src string) string {
	var buf strings.Builder
	buf.Grow(len(src))
	state := csNormal
	i := 0
	for i < len(src) {
		ch := src[i]
		switch state {
		case csNormal:
			switch {
			case ch == '"':
				state = csString
				buf.WriteByte(ch)
			case ch == '\'':
				state = csChar
				buf.WriteByte(ch)
			case ch == '/' && i+1 < len(src) && src[i+1] == '/':
				state = csLineComment
				i += 2
				continue
			case ch == '/' && i+1 < len(src) && src[i+1] == '*':
				state = csBlockComment
				i += 2
				continue
			default:
				buf.WriteByte(ch)
			}
		case csString:
			buf.WriteByte(ch)
			if ch == '\\' && i+1 < len(src) {
				i++
				buf.WriteByte(src[i])
			} else if ch == '"' {
				state = csNormal
			}
		case csChar:
			buf.WriteByte(ch)
			if ch == '\\' && i+1 < len(src) {
				i++
				buf.WriteByte(src[i])
			} else if ch == '\'' {
				state = csNormal
			}
		case csLineComment:
			if ch == '\n' {
				buf.WriteByte('\n')
				state = csNormal
			}
		case csBlockComment:
			if ch == '*' && i+1 < len(src) && src[i+1] == '/' {
				i += 2
				state = csNormal
				continue
			}
			if ch == '\n' {
				buf.WriteByte('\n') // preserve newlines for line counting
			}
		}
		i++
	}
	return buf.String()
}

// countLines returns the number of newlines in s (= 1-based line of the last char).
func countLines(s string) int {
	return strings.Count(s, "\n")
}

// ── Java keywords ──────────────────────────────────────────────────────────

var javaKeywords = map[string]bool{
	"abstract": true, "assert": true, "boolean": true, "break": true,
	"byte": true, "case": true, "catch": true, "char": true, "class": true,
	"const": true, "continue": true, "default": true, "do": true,
	"double": true, "else": true, "enum": true, "extends": true,
	"final": true, "finally": true, "float": true, "for": true, "goto": true,
	"if": true, "implements": true, "import": true, "instanceof": true,
	"int": true, "interface": true, "long": true, "native": true, "new": true,
	"null": true, "package": true, "private": true, "protected": true,
	"public": true, "return": true, "short": true, "static": true,
	"strictfp": true, "super": true, "switch": true, "synchronized": true,
	"this": true, "throw": true, "throws": true, "transient": true,
	"try": true, "var": true, "void": true, "volatile": true, "while": true,
	// common return types that appear before method name
	"String": true, "Object": true, "List": true, "Map": true,
	"Optional": true, "Mono": true, "Flux": true,
	"Future": true, "CompletableFuture": true,
}
