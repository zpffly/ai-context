package aictx

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	serviceRe = regexp.MustCompile(`^\s*service\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
	methodRe  = regexp.MustCompile(`^\s*(?:oneway\s+)?([A-Za-z_][A-Za-z0-9_\.<>]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)`)
	reqRe     = regexp.MustCompile(`\d+\s*:\s*(?:required\s+|optional\s+)?([A-Za-z_][A-Za-z0-9_\.]*)\s+`)
)

func loadRPCIndex(cfg Config, repoRoot string) (map[string]RPCDefinition, error) {
	idlRoot := resolvePath(repoRoot, cfg.IDL.Root)
	files, err := thriftFiles(idlRoot, cfg.IDL.Include, cfg.IDL.Exclude)
	if err != nil {
		return nil, err
	}
	index := map[string]RPCDefinition{}
	for _, file := range files {
		defs, err := parseThriftFile(file)
		if err != nil {
			return nil, err
		}
		for _, def := range defs {
			key := def.FullName()
			if _, exists := index[key]; exists {
				return nil, fmt.Errorf("duplicate RPC %s in %s", key, file)
			}
			index[key] = def
		}
	}
	return index, nil
}

func thriftFiles(root string, include, exclude []string) ([]string, error) {
	var files []string
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("idl root %s: %w", root, err)
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			for _, pattern := range exclude {
				if ok, _ := filepath.Match(pattern, name); ok {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(path) != ".thrift" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if matchesAny(rel, include) && (len(exclude) == 0 || !matchesAny(rel, exclude)) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func matchesAny(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	path = filepath.ToSlash(path)
	for _, p := range patterns {
		p = filepath.ToSlash(p)
		if p == "**/*.thrift" && strings.HasSuffix(path, ".thrift") {
			return true
		}
		if p == "*" {
			return true
		}
		if ok, _ := filepath.Match(p, path); ok {
			return true
		}
		if strings.HasPrefix(p, "**/") {
			tail := strings.TrimPrefix(p, "**/")
			if ok, _ := filepath.Match(tail, filepath.Base(path)); ok {
				return true
			}
		}
	}
	return false
}

func parseThriftFile(path string) ([]RPCDefinition, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var defs []RPCDefinition
	scanner := bufio.NewScanner(f)
	var currentService string
	var serviceComment string
	var pendingComments []string
	var inBlockComment bool
	var blockLines []string
	braceDepth := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if inBlockComment {
			clean := strings.TrimSpace(trimmed)
			if end := strings.Index(clean, "*/"); end >= 0 {
				clean = clean[:end]
				inBlockComment = false
				blockLines = appendCleanComment(blockLines, clean)
				pendingComments = append(pendingComments, strings.Join(blockLines, "\n"))
				blockLines = nil
				continue
			}
			blockLines = appendCleanComment(blockLines, clean)
			continue
		}

		if strings.HasPrefix(trimmed, "/*") {
			clean := strings.TrimPrefix(trimmed, "/*")
			if strings.HasPrefix(clean, "*") {
				clean = strings.TrimPrefix(clean, "*")
			}
			if end := strings.Index(clean, "*/"); end >= 0 {
				clean = clean[:end]
				pendingComments = append(pendingComments, strings.Join(appendCleanComment(nil, clean), "\n"))
			} else {
				inBlockComment = true
				blockLines = appendCleanComment(blockLines, clean)
			}
			continue
		}

		if strings.HasPrefix(trimmed, "//") {
			pendingComments = append(pendingComments, strings.TrimSpace(strings.TrimPrefix(trimmed, "//")))
			continue
		}

		if trimmed == "" {
			continue
		}

		if m := serviceRe.FindStringSubmatch(line); len(m) == 2 {
			currentService = m[1]
			serviceComment = strings.Join(pendingComments, "\n")
			pendingComments = nil
			braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
			continue
		}

		if currentService == "" {
			pendingComments = nil
			continue
		}

		if m := methodRe.FindStringSubmatch(stripInlineComment(line)); len(m) == 4 {
			defs = append(defs, RPCDefinition{
				Service:        currentService,
				Method:         m[2],
				Request:        firstRequestType(m[3]),
				Response:       m[1],
				ThriftFile:     path,
				ServiceComment: serviceComment,
				MethodComment:  strings.Join(pendingComments, "\n"),
			})
			pendingComments = nil
		} else {
			pendingComments = nil
		}

		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
		if braceDepth <= 0 && strings.Contains(line, "}") {
			currentService = ""
			serviceComment = ""
			braceDepth = 0
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return defs, nil
}

func appendCleanComment(lines []string, line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "*")
	line = strings.TrimSpace(line)
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func stripInlineComment(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func firstRequestType(params string) string {
	if m := reqRe.FindStringSubmatch(params); len(m) == 2 {
		return m[1]
	}
	return ""
}
