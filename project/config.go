// Package project owns workspace configuration, document overlays, and immutable
// snapshots. Like typescript-go, it sits above compiler and below the server.
package project

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/compiler"
)

type Config struct {
	Root           string
	ProjectFile    string
	Name           string
	SourceEncoding string
	Sources        []compiler.SourceFileInput
	// References retain manifest metadata only; no type library is loaded yet.
	References []string
}

type LoadOptions struct {
	Root        string
	ProjectFile string
	// Auto honors Unicode BOMs, prefers UTF-8, then falls back to Windows-1252.
	// Windows1252 forces that code page for BOM-less files; UTF8 is strict.
	SourceEncoding string
}

// LoadConfig loads one VB project. Without an explicit manifest it selects the
// sole .vbp in root, or infers a project from .bas/.cls/.frm files recursively.
// Multiple manifests require a selection instead of mixing their namespaces.
func LoadConfig(ctx context.Context, root, projectFile string) (Config, error) {
	return LoadConfigWithOptions(ctx, LoadOptions{Root: root, ProjectFile: projectFile})
}

func LoadConfigWithOptions(ctx context.Context, options LoadOptions) (Config, error) {
	c := Config{}
	if err := ctx.Err(); err != nil {
		return c, err
	}
	encoding, err := sourceEncoding(options.SourceEncoding)
	if err != nil {
		return c, err
	}
	c.SourceEncoding = encoding
	root, projectFile := options.Root, options.ProjectFile
	if root == "" && projectFile == "" {
		return c, nil
	}
	if root == "" {
		projectFile, err = filepath.Abs(projectFile)
		if err != nil {
			return c, err
		}
		root = filepath.Dir(projectFile)
	}
	c.Root, err = filepath.Abs(root)
	if err != nil {
		return c, err
	}
	if projectFile == "" {
		entries, err := os.ReadDir(c.Root)
		if err != nil {
			return c, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".vbp") {
				if projectFile != "" {
					return c, fmt.Errorf("multiple .vbp files: select initializationOptions.projectFile")
				}
				projectFile = entry.Name()
			}
		}
	}
	c.Name = filepath.Base(c.Root)
	if projectFile != "" {
		if !filepath.IsAbs(projectFile) {
			projectFile = filepath.Join(c.Root, projectFile)
		}
		c.ProjectFile = filepath.Clean(projectFile)
		data, err := os.ReadFile(c.ProjectFile)
		if err != nil {
			return c, err
		}
		c.Name = strings.TrimSuffix(filepath.Base(projectFile), filepath.Ext(projectFile))
		text, err := decodeSource(data, encoding)
		if err != nil {
			return c, fmt.Errorf("%s: %w", c.ProjectFile, err)
		}
		scan := bufio.NewScanner(strings.NewReader(text))
		for scan.Scan() {
			if err := ctx.Err(); err != nil {
				return c, err
			}
			key, value, ok := strings.Cut(scan.Text(), "=")
			if !ok {
				continue
			}
			key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
			kind, module := ast.ModuleKindUnspecified, ""
			switch key {
			case "name":
				c.Name = unquote(value)
				continue
			case "reference", "object":
				c.References = append(c.References, value)
				continue
			case "module", "class":
				left, right, ok := strings.Cut(value, ";")
				if !ok {
					return c, fmt.Errorf("invalid %s entry in %s", key, projectFile)
				}
				module, value = strings.TrimSpace(left), strings.TrimSpace(right)
				kind = ast.ModuleKindStandard
				if key == "class" {
					kind = ast.ModuleKindClass
				}
			case "form":
				kind = ast.ModuleKindUserForm
			default:
				continue
			}
			name := filepath.FromSlash(strings.ReplaceAll(unquote(value), `\`, "/"))
			if !filepath.IsAbs(name) {
				name = filepath.Join(filepath.Dir(c.ProjectFile), name)
			}
			c.Sources = append(c.Sources, compiler.SourceFileInput{FileName: filepath.Clean(name), ModuleName: module, ModuleKind: kind})
		}
		if err := scan.Err(); err != nil {
			return c, err
		}
	} else {
		err := filepath.WalkDir(c.Root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() {
				if name != c.Root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || entry.Name() == "dist" || entry.Name() == "build") {
					return filepath.SkipDir
				}
				return nil
			}
			switch strings.ToLower(filepath.Ext(name)) {
			case ".bas", ".cls", ".frm":
				c.Sources = append(c.Sources, compiler.SourceFileInput{FileName: name})
			}
			return nil
		})
		if err != nil {
			return c, err
		}
	}
	sort.SliceStable(c.Sources, func(i, j int) bool { return c.Sources[i].FileName < c.Sources[j].FileName })
	for i := range c.Sources {
		if err := ctx.Err(); err != nil {
			return c, err
		}
		data, err := os.ReadFile(c.Sources[i].FileName)
		if err != nil {
			return c, err
		}
		text, err := decodeSource(data, encoding)
		if err != nil {
			return c, fmt.Errorf("%s: %w", c.Sources[i].FileName, err)
		}
		c.Sources[i].Text = text
	}
	return c, nil
}

func unquote(s string) string { return strings.Trim(strings.TrimSpace(s), `"`) }

// codeText masks exported designer preambles rather than deleting them. Both
// byte offsets and line/UTF-16 positions still address the original document.
// Attributes and all executable VBA remain intact. This is an import boundary,
// not an extension of the VBA grammar with VB6's designer serialization syntax.
func codeText(name, text string) string {
	end := 0
	if strings.HasPrefix(text, "\ufeff") {
		end = 3
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".cls" || ext == ".frm" {
		lines := strings.SplitAfter(text[end:], "\n")
		if len(lines) > 0 && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(lines[0])), "VERSION ") {
			bomEnd := end
			end += len(lines[0])
			depth := 0
			for _, line := range lines[1:] {
				upper := strings.ToUpper(strings.TrimSpace(line))
				if upper == "BEGIN" || strings.HasPrefix(upper, "BEGIN ") || strings.HasPrefix(upper, "BEGINPROPERTY ") {
					depth++
				} else if upper == "END" || upper == "ENDPROPERTY" {
					depth--
				} else if depth == 0 && upper != "" && !strings.HasPrefix(upper, "OBJECT =") {
					break
				}
				end += len(line)
				if depth == 0 && upper == "END" {
					break
				}
			}
			// An unfinished designer block must not hide the remainder of an
			// edited file from parsing and diagnostics.
			if depth != 0 {
				end = bomEnd
			}
		}
	}
	if end == 0 {
		return text
	}
	masked := []byte(text[:end])
	for i, b := range masked {
		if b != '\r' && b != '\n' {
			masked[i] = ' '
		}
	}
	return string(masked) + text[end:]
}
