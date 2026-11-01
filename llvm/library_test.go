package llvm

import (
	"testing"

	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestLibraryABICatalogCoverage(t *testing.T) {
	declarations := make(map[string]string)
	for _, fn := range stdlib.Functions() {
		abi, ok := libraryABIs[fn.ID]
		if ok != (fn.Call == stdlib.SnapshotCall) {
			t.Fatalf("%s has inconsistent ABI coverage", fn.Name)
		}
		if !ok {
			continue
		}
		signature, available := ir.LookupLibraryCall(fn.ID)
		if !available || abi.name == "" {
			t.Fatalf("%s has no verified contract or native name", fn.Name)
		}
		declaration := abi.declaration(signature.Arity)
		if previous, exists := declarations[abi.name]; exists && previous != declaration {
			t.Fatalf("%s has conflicting native prototypes", abi.name)
		}
		declarations[abi.name] = declaration
	}
	for id := range libraryABIs {
		if _, ok := ir.LookupLibraryCall(id); !ok {
			t.Fatalf("native member %d has no verified library contract", id)
		}
	}
}
