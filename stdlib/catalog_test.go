package stdlib_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/stdlib"
)

func TestCatalogIntegrity(t *testing.T) {
	ids := make(map[stdlib.MemberID]string)
	names := make(map[string]bool)
	modules := make(map[string]bool)
	for _, module := range stdlib.Modules() {
		key := strings.ToLower(module.Name)
		if key == "" || module.Section == "" || modules[key] {
			t.Fatalf("invalid or duplicate procedural module %q", module.Name)
		}
		modules[key], names[key] = true, true
	}
	enums := make(map[string]bool)
	for _, enum := range stdlib.Enums() {
		key := strings.ToLower(enum.Name)
		if key == "" || enum.Section == "" || names[key] {
			t.Fatalf("invalid or duplicate enum %q", enum.Name)
		}
		enums[key], names[key] = true, true
	}
	check := func(id stdlib.MemberID, name string) {
		t.Helper()
		if id == stdlib.None || name == "" {
			t.Fatalf("incomplete catalog identity: %d %q", id, name)
		}
		if previous, exists := ids[id]; exists {
			t.Fatalf("%s and %s share member ID %d", previous, name, id)
		}
		ids[id] = name
		key := strings.ToLower(name) // Preserve distinct $ exports.
		if names[key] {
			t.Fatalf("duplicate ambient export %s", name)
		}
		names[key] = true
	}
	for _, fn := range stdlib.Functions() {
		check(fn.ID, fn.Name)
		if fn.Call != stdlib.SpecializedCall && fn.Call != stdlib.SnapshotCall {
			t.Fatalf("function %s has an unknown call contract", fn.Name)
		}
		if fn.LanguageBuiltin {
			if fn.Module != "" || fn.Section != "" {
				t.Fatalf("language built-in %s was attributed to a section 6 module", fn.Name)
			}
		} else if !modules[strings.ToLower(fn.Module)] || fn.Section == "" {
			t.Fatalf("function %s has no module or specification section", fn.Name)
		}
		parameters := make(map[string]bool)
		for _, p := range fn.Parameters {
			key := strings.ToLower(p.Name)
			if key == "" || parameters[key] {
				t.Fatalf("invalid or duplicate parameter %q in %s", p.Name, fn.Name)
			}
			parameters[key] = true
			if !p.Optional && p.Default.Kind != stdlib.NoLiteral {
				t.Fatalf("required parameter %s.%s has an optional default", fn.Name, p.Name)
			}
		}
	}
	for _, c := range stdlib.Constants() {
		check(c.ID, c.Name)
		if (!modules[strings.ToLower(c.Owner)] && !enums[strings.ToLower(c.Owner)]) || c.Section == "" {
			t.Fatalf("constant %s has no owner or specification section", c.Name)
		}
		if c.Value.Kind == stdlib.NoLiteral {
			t.Fatalf("constant %s has no value", c.Name)
		}
		if c.Value.Kind == stdlib.NullStringLiteral && (c.Type != (stdlib.Type{Kind: stdlib.TypeString}) || c.Value.Text != "") {
			t.Fatalf("null string constant %s has incompatible type or contents", c.Name)
		}
	}
	for _, class := range stdlib.Classes() {
		check(class.ID, class.Name)
		if class.Section == "" {
			t.Fatalf("class %s has no specification section", class.Name)
		}
		defaultFound := class.DefaultMember == stdlib.None
		for _, member := range class.Members {
			check(member.ID, class.Name+"."+member.Name)
			if member.Section == "" || member.Kind > stdlib.VariableMember {
				t.Fatalf("incomplete member %s.%s", class.Name, member.Name)
			}
			if member.Kind == stdlib.VariableMember && len(member.Parameters) != 0 {
				t.Fatalf("variable %s.%s has parameters", class.Name, member.Name)
			}
			if member.ID == class.DefaultMember {
				defaultFound = member.Kind == stdlib.GetMember
			}
			parameters := make(map[string]bool)
			for _, p := range member.Parameters {
				key := strings.ToLower(p.Name)
				if key == "" || parameters[key] || !p.Optional && p.Default.Kind != stdlib.NoLiteral {
					t.Fatalf("invalid parameter %s.%s.%s", class.Name, member.Name, p.Name)
				}
				parameters[key] = true
			}
		}
		if !defaultFound {
			t.Fatalf("class %s has no matching default getter", class.Name)
		}
	}
}

func TestCatalogDescriptorsAreIndependent(t *testing.T) {
	for i, enum := range stdlib.Enums() {
		changed := stdlib.Enums()
		changed[i].Name = "changed"
		if stdlib.Enums()[i] != enum {
			t.Fatal("enum descriptor leaked between catalog readers")
		}
	}
	for i, module := range stdlib.Modules() {
		changed := stdlib.Modules()
		changed[i].Name = "changed"
		if stdlib.Modules()[i] != module {
			t.Fatal("module descriptor leaked between catalog readers")
		}
	}
	functions := stdlib.Functions()
	for i := range functions {
		functions[i].Name = "changed"
		for j := range functions[i].Parameters {
			functions[i].Parameters[j].Name = "changed"
			functions[i].Parameters[j].Default.Text = "changed"
		}
	}
	constants := stdlib.Constants()
	for i := range constants {
		constants[i].Name = "changed"
		constants[i].Value.Text = "changed"
	}
	for _, fn := range stdlib.Functions() {
		if fn.Name == "changed" {
			t.Fatal("function descriptor leaked between catalog readers")
		}
		for _, p := range fn.Parameters {
			if p.Name == "changed" || p.Default.Text == "changed" {
				t.Fatal("parameter/default descriptor leaked between catalog readers")
			}
		}
	}
	for _, c := range stdlib.Constants() {
		if c.Name == "changed" || c.Value.Text == "changed" {
			t.Fatal("constant descriptor leaked between catalog readers")
		}
	}
	classes := stdlib.Classes()
	for i := range classes {
		classes[i].Name = "changed"
		for j := range classes[i].Members {
			classes[i].Members[j].Name = "changed"
			for k := range classes[i].Members[j].Parameters {
				classes[i].Members[j].Parameters[k].Name = "changed"
				classes[i].Members[j].Parameters[k].Default.Text = "changed"
			}
		}
	}
	for _, class := range stdlib.Classes() {
		if class.Name == "changed" {
			t.Fatal("class descriptor leaked between catalog readers")
		}
		for _, member := range class.Members {
			if member.Name == "changed" {
				t.Fatal("member descriptor leaked between catalog readers")
			}
			for _, p := range member.Parameters {
				if p.Name == "changed" || p.Default.Text == "changed" {
					t.Fatal("member parameter/default leaked between catalog readers")
				}
			}
		}
	}
}
