// Package runtimebuild reads the native runtime's explicit source manifest.
// Keep build clients independent of the runtime's translation-unit layout.
package runtimebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Sources(directory string) ([]string, error) {
	manifest := filepath.Join(directory, "sources.txt")
	data, err := os.ReadFile(manifest)
	if err != nil {
		return nil, fmt.Errorf("runtime sources: %w", err)
	}
	names := strings.Fields(string(data))
	if len(names) == 0 {
		return nil, fmt.Errorf("runtime source manifest %s is empty", manifest)
	}
	sources := make([]string, len(names))
	for i, name := range names {
		sources[i] = filepath.Join(directory, name)
	}
	return sources, nil
}
