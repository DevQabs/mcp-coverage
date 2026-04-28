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

var (
	constFieldRe = regexp.MustCompile(`\bfinal\s+String\s+(\w+)\s*=\s*"([^"]*)"`)
	constClassRe = regexp.MustCompile(`(?:class|interface|enum)\s+(\w+)`)
	identRefRe   = regexp.MustCompile(`^([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)`)
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
