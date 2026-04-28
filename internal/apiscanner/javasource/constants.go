package javasource

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ConstantRegistry maps String constant identifiers to their resolved values.
// Keys: "ClassName.FIELD" (qualified) and "FIELD" (unqualified fallback).
type ConstantRegistry map[string]string

// MetaAnnotationRegistry is the set of custom annotation simple-names (e.g. "HealthRestController")
// that act as meta-annotations composing @RestController (or @Controller).
// Controllers annotated with any name in this set are treated as Spring controllers,
// and their annotation value() is treated as the base @RequestMapping path.
type MetaAnnotationRegistry map[string]bool

var (
	constFieldRe     = regexp.MustCompile(`\bfinal\s+String\s+(\w+)\s*=\s*"([^"]*)"`)
	constClassRe     = regexp.MustCompile(`(?:class|interface|enum)\s+(\w+)`)
	identRefRe       = regexp.MustCompile(`^([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)`)
	metaInterfaceRe  = regexp.MustCompile(`(?:public\s+)?@interface\s+(\w+)`)
)

// BuildConstantRegistry walks projectPath and collects all `final String FIELD = "..."` declarations.
func BuildConstantRegistry(projectPath string) ConstantRegistry {
	reg := make(ConstantRegistry)
	_ = filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "build" ||
				name == "target" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".java") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		src := stripComments(string(data))
		className := ""
		if m := constClassRe.FindStringSubmatch(src); len(m) >= 2 {
			className = m[1]
		}
		for _, m := range constFieldRe.FindAllStringSubmatch(src, -1) {
			field, val := m[1], m[2]
			reg[field] = val
			if className != "" {
				reg[className+"."+field] = val
			}
		}
		return nil
	})
	return reg
}

// BuildMetaAnnotationRegistry walks projectPath and discovers custom annotation types
// (i.e. @interface declarations) that are themselves meta-annotated with @RestController
// or @Controller. These custom annotations can then be used to detect controller classes
// and extract their base request-mapping paths.
//
// Example: given
//
//	@RestController
//	@RequestMapping(...)
//	public @interface HealthRestController { String[] value() default {}; }
//
// The registry will contain {"HealthRestController": true}.
func BuildMetaAnnotationRegistry(projectPath string) MetaAnnotationRegistry {
	reg := make(MetaAnnotationRegistry)
	_ = filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "build" ||
				name == "target" || name == "node_modules" || name == "out" ||
				name == ".gradle" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".java") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		src := stripComments(string(data))

		// Fast pre-check: must contain both "@interface" and a controller annotation keyword.
		if !strings.Contains(src, "@interface") {
			return nil
		}
		if !strings.Contains(src, "RestController") && !strings.Contains(src, "@Controller") {
			return nil
		}

		// Find the @interface declaration position.
		ifaceIdx := metaInterfaceRe.FindStringIndex(src)
		if ifaceIdx == nil {
			return nil
		}
		m := metaInterfaceRe.FindStringSubmatch(src[ifaceIdx[0]:])
		if len(m) < 2 {
			return nil
		}
		annotName := m[1]

		// The annotations above the @interface declaration appear in the text before it.
		preInterface := src[:ifaceIdx[0]]

		// If @RestController or @Controller appears before the @interface, it's a meta-annotation.
		if controllerAnnotRe.MatchString(preInterface) {
			reg[annotName] = true
		}
		return nil
	})
	return reg
}

// extractConstantRef extracts the first identifier or qualified name from annotation
// content after stripping attribute prefixes (value=, path=) and array braces.
// Returns "" if content starts with a string literal or has no recognisable identifier.
func extractConstantRef(content string) string {
	c := strings.TrimSpace(content)
	for _, prefix := range []string{"value=", "path=", "value =", "path ="} {
		if strings.HasPrefix(c, prefix) {
			c = strings.TrimSpace(c[len(prefix):])
			break
		}
	}
	c = strings.TrimLeft(c, "{ \t")
	if strings.HasPrefix(c, "\"") {
		return "" // string literal, not a constant
	}
	return identRefRe.FindString(c)
}

// resolvePathsOrMark resolves path strings from annotation content via:
//  1. String literals (direct)
//  2. ConstantRegistry lookup
//  3. UNRESOLVED:<ref> marker (never silently drops the API)
//
// Returns (paths, unresolved, constRef).
// Empty content → (nil, false, "") meaning no path override on this annotation.
func resolvePathsOrMark(content string, reg ConstantRegistry) (paths []string, unresolved bool, constRef string) {
	if strings.TrimSpace(content) == "" {
		return nil, false, ""
	}
	paths = extractPaths(content)
	if len(paths) > 0 {
		return paths, false, ""
	}
	ref := extractConstantRef(content)
	if ref == "" {
		return []string{"UNRESOLVED:" + strings.TrimSpace(content)}, true, content
	}
	if reg != nil {
		if val, ok := reg[ref]; ok {
			return []string{val}, false, ""
		}
		if idx := strings.LastIndex(ref, "."); idx >= 0 {
			if val, ok := reg[ref[idx+1:]]; ok {
				return []string{val}, false, ""
			}
		}
	}
	return []string{"UNRESOLVED:" + ref}, true, ref
}
