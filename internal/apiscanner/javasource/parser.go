package javasource

import (
	"regexp"
	"strings"
)

// ControllerDef holds all extracted information from one Spring controller file.
type ControllerDef struct {
	ClassName          string
	BasePaths          []string // from class-level @RequestMapping (may contain UNRESOLVED: prefix)
	BasePathUnresolved bool     // true when base path is a constant that could not be resolved
	BasePathRef        string   // the unresolved constant reference string
	Methods            []MethodDef
	SourceFile         string
	IsInterface        bool // true when parsed from an interface (not @RestController)
	IsAbstract         bool // true for abstract class controllers
}

// MethodDef is one handler method with its resolved HTTP method and relative paths.
type MethodDef struct {
	MethodName    string
	HTTPMethod    string
	Paths         []string // relative paths (may contain UNRESOLVED: prefix)
	LineNumber    int
	Unresolved    bool   // true when path is a constant that could not be resolved
	UnresolvedRef string // the unresolved constant reference
}

var (
	// controllerAnnotRe matches @RestController or @Controller, including fully
	// qualified forms (e.g. @org.springframework.web.bind.annotation.RestController).
	// The (?:\w+\.)* prefix allows any package path before the simple name.
	controllerAnnotRe = regexp.MustCompile(`@(?:\w+\.)*(?:Rest)?Controller\b`)
	// adviceAnnotRe detects advice classes that must NOT be treated as controllers.
	adviceAnnotRe = regexp.MustCompile(`@(?:\w+\.)*(?:Rest)?ControllerAdvice\b`)
	// classRe finds class or interface declarations and captures the name.
	classRe = regexp.MustCompile(`(?:public\s+|protected\s+|abstract\s+|final\s+)*(?:class|interface)\s+(\w+)`)
	// methodNameRe finds the last non-keyword identifier before '(' in a method signature.
	methodNameRe = regexp.MustCompile(`(\w+)\s*\(`)
	// interfaceWithMappingRe detects interfaces that declare Spring MVC mapping annotations.
	interfaceWithMappingRe = regexp.MustCompile(`\binterface\s+\w+`)
	// anyMappingAnnotRe matches any Spring MVC mapping annotation (incl. fully qualified).
	anyMappingAnnotRe = regexp.MustCompile(`@(?:\w+\.)*(?:Request|Get|Post|Put|Delete|Patch)Mapping\b`)
)

// ParseFile parses a single Java source file. Returns nil if not a controller.
// Backward-compatible wrapper — uses no constant registry.
func ParseFile(path, src string) *ControllerDef {
	return ParseFileWithRegistry(path, src, nil, nil)
}

// ParseFileWithRegistry parses a Java source file using reg to resolve path constants.
// reg may be nil (disables constant resolution; unresolvable paths are still included
// with an UNRESOLVED: prefix rather than being silently dropped).
func ParseFileWithRegistry(path, src string, reg ConstantRegistry, metaReg MetaAnnotationRegistry) *ControllerDef {
	clean := stripComments(src)

	if !isController(clean, metaReg) {
		return nil
	}

	className, classPos, isIface, isAbstract := findClassDeclaration(clean)
	if classPos < 0 {
		return nil
	}

	basePaths, baseUnresolved, baseRef := extractBasePaths(clean[:classPos], reg, metaReg)

	bodyStart := strings.Index(clean[classPos:], "{")
	if bodyStart < 0 {
		return nil
	}
	bodyStart += classPos + 1

	methods, _ := extractMethods(clean, bodyStart, countLines(clean[:bodyStart]), reg)

	return &ControllerDef{
		ClassName:          className,
		BasePaths:          basePaths,
		BasePathUnresolved: baseUnresolved,
		BasePathRef:        baseRef,
		Methods:            methods,
		SourceFile:         path,
		IsInterface:        isIface,
		IsAbstract:         isAbstract,
	}
}

// ── controller detection ───────────────────────────────────────────────────

func isController(src string, metaReg MetaAnnotationRegistry) bool {
	if adviceAnnotRe.MatchString(src) {
		return false
	}
	if controllerAnnotRe.MatchString(src) {
		return true
	}
	// Check custom meta-annotations (e.g. @HealthRestController).
	for annotName := range metaReg {
		if containsAnnotation(src, annotName) {
			return true
		}
	}
	// Interfaces (and abstract types) that carry Spring MVC mapping annotations
	// are treated as controller blueprints — they define real API routes even
	// without @RestController.
	if interfaceWithMappingRe.MatchString(src) && anyMappingAnnotRe.MatchString(src) {
		return true
	}
	return false
}

// containsAnnotation reports whether src contains @annotName as an annotation reference.
func containsAnnotation(src, annotName string) bool {
	idx := strings.Index(src, "@"+annotName)
	if idx < 0 {
		return false
	}
	// Verify the character after the name is not a word character (prevents partial matches).
	end := idx + 1 + len(annotName)
	if end < len(src) {
		ch := src[end]
		if isWordChar(ch) {
			return false
		}
	}
	return true
}

// ── class/interface declaration ────────────────────────────────────────────

func findClassDeclaration(src string) (name string, pos int, isInterface bool, isAbstract bool) {
	m := classRe.FindStringIndex(src)
	if m == nil {
		return "", -1, false, false
	}
	sub := classRe.FindStringSubmatch(src[m[0]:])
	if len(sub) < 2 {
		return "", -1, false, false
	}
	ctx := src[m[0] : m[0]+len(sub[0])]
	isInterface = strings.Contains(ctx, "interface")
	isAbstract = strings.Contains(ctx, "abstract")
	return sub[1], m[0], isInterface, isAbstract
}

// ── class-level @RequestMapping ────────────────────────────────────────────

func extractBasePaths(preClass string, reg ConstantRegistry, metaReg MetaAnnotationRegistry) ([]string, bool, string) {
	// Scan all annotations in preClass: match @RequestMapping and any custom
	// controller meta-annotations (e.g. @HealthRestController) which provide
	// the base path via their value() attribute.
	var last *annotRaw
	pos := 0
	for pos < len(preClass) {
		idx := strings.Index(preClass[pos:], "@")
		if idx < 0 {
			break
		}
		abs := pos + idx
		a, newPos := parseAnnotation(preClass, abs)
		if a != nil {
			if a.name == "RequestMapping" || metaReg[a.name] {
				last = a
			}
		}
		if newPos > abs {
			pos = newPos
		} else {
			pos = abs + 1
		}
	}
	if last == nil {
		return nil, false, ""
	}
	return resolvePathsOrMark(last.content, reg)
}

// ── method-level annotations ───────────────────────────────────────────────

type pendingAnnot struct {
	annot   *annotRaw
	lineNum int
}

// extractMethods scans the class body starting at bodyStart (right after the opening '{')
// and collects all handler methods, including abstract and interface methods (terminated by ';').
func extractMethods(src string, bodyStart int, classLineOffset int, reg ConstantRegistry) ([]MethodDef, int) {
	var methods []MethodDef

	pos := bodyStart
	braceDepth := 1
	var pending []pendingAnnot
	lineNum := classLineOffset

	for pos < len(src) {
		ch := src[pos]

		switch {
		// ── opening brace ──────────────────────────────────────────────
		case ch == '{':
			braceDepth++
			if braceDepth == 2 && len(pending) > 0 {
				lastAnnotEnd := pending[len(pending)-1].annot.endPos
				sig := src[lastAnnotEnd:pos]
				name := findMethodName(sig)
				if name != "" {
					firstAnnotLine := pending[0].lineNum
					m := buildMethodDefs(pending, name, firstAnnotLine, reg)
					methods = append(methods, m...)
				}
				pending = nil
			} else if braceDepth == 2 {
				pending = nil
			}
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
				pending = append(pending, pendingAnnot{
					annot:   a,
					lineNum: lineNum + countLines(src[bodyStart:pos]),
				})
			} else if a == nil {
				newPos = pos + 1
			}
			pos = newPos

		// ── semicolon: interface/abstract method or field declaration ──
		case ch == ';' && braceDepth == 1:
			if len(pending) > 0 {
				// Could be an interface method or abstract method declaration.
				// Extract the method name from text between last annotation and ';'.
				lastAnnotEnd := pending[len(pending)-1].annot.endPos
				sig := src[lastAnnotEnd:pos]
				name := findMethodName(sig)
				if name != "" {
					firstAnnotLine := pending[0].lineNum
					m := buildMethodDefs(pending, name, firstAnnotLine, reg)
					methods = append(methods, m...)
				}
			}
			pending = nil
			pos++

		// ── newline: line tracking ─────────────────────────────────────
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

// buildMethodDefs creates MethodDef entries, expanding one entry per
// HTTP method × path combination (handles @RequestMapping with multiple methods).
func buildMethodDefs(pending []pendingAnnot, methodName string, lineNum int, reg ConstantRegistry) []MethodDef {
	var defs []MethodDef
	for _, pa := range pending {
		a := pa.annot

		// HTTP methods
		var httpMethods []string
		if fixed, ok := httpMethodForAnnotation[a.name]; ok {
			httpMethods = []string{fixed}
		} else {
			// @RequestMapping — extract method= attribute, default GET
			httpMethods = extractHTTPMethods(a.content)
		}

		// Paths (with constant resolution and UNRESOLVED marker)
		paths, unresolved, ref := resolvePathsOrMark(a.content, reg)

		for _, httpMethod := range httpMethods {
			defs = append(defs, MethodDef{
				MethodName:    methodName,
				HTTPMethod:    httpMethod,
				Paths:         paths,
				LineNumber:    lineNum,
				Unresolved:    unresolved,
				UnresolvedRef: ref,
			})
		}
	}
	return defs
}

// findMethodName finds the method name from the text between an annotation and
// the opening '{' or ';'. Looks for the last non-keyword identifier before '('
// that is NOT itself an annotation name (i.e. not preceded by '@').
func findMethodName(sig string) string {
	matches := methodNameRe.FindAllStringSubmatchIndex(sig, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		word := sig[m[2]:m[3]]
		if javaKeywords[word] {
			continue
		}
		// Skip annotation names: the character immediately before the identifier
		// (ignoring whitespace) must not be '@'.
		pre := strings.TrimRight(sig[:m[0]], " \t\n\r")
		if len(pre) > 0 && pre[len(pre)-1] == '@' {
			continue
		}
		return word
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

// stripComments removes Java // and /* */ comments while preserving newlines.
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
				buf.WriteByte('\n')
			}
		}
		i++
	}
	return buf.String()
}

// countLines returns the number of newlines in s.
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
	"Future": true, "CompletableFuture": true, "ResponseEntity": true,
}
