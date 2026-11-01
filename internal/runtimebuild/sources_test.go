package runtimebuild_test

import (
	"path/filepath"
	"slices"
	"testing"

	"sihlfall.ch/vambas-vai/internal/runtimebuild"
)

func TestManifestCoversRuntimeTranslationUnits(t *testing.T) {
	directory := filepath.Join("..", "..", "runtime")
	sources, err := runtimebuild.Sources(directory)
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.c"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(sources)
	if !slices.Equal(sources, files) {
		t.Fatalf("runtime source manifest must cover every C file exactly once: %v; files: %v", sources, files)
	}
}
