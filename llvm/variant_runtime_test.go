package llvm_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Compile the ABI boundary as C, independently of the LLVM representation.
// Inject failures into actual BSTR/private String allocations and track owners.
func TestVariantABIAndOwnership(t *testing.T) {
	nativeRuntimeC(t, abiTest)
}

func nativeRuntimeC(t *testing.T, sourceText string) [][]byte {
	t.Helper()
	clang, err := exec.LookPath("clang-20")
	if err != nil {
		clang, err = exec.LookPath("clang")
	}
	if err != nil {
		if os.Getenv("VBA_LLVM_TEST_REQUIRED") != "" {
			t.Fatal(err)
		}
		t.Skip("clang not installed")
	}
	var outputs [][]byte
	dir := t.TempDir()
	source := filepath.Join(dir, "abi.c")
	if err := os.WriteFile(source, []byte(sourceText), 0600); err != nil {
		t.Fatal(err)
	}
	include, err := filepath.Abs("../runtime")
	if err != nil {
		t.Fatal(err)
	}
	// Instrument every runtime translation unit, while the probe's allocator
	// callbacks themselves continue to call the real C allocator.
	hooks := filepath.Join(dir, "alloc_hooks.h")
	if err := os.WriteFile(hooks, []byte(`#include <stdlib.h>
void *vba_test_malloc(size_t);
void vba_test_free(void *);
#define malloc vba_test_malloc
#define free vba_test_free
`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"-O0", "-O2"} {
		var objects []string
		for _, file := range runtimeSources(t) {
			object := filepath.Join(dir, filepath.Base(file)+level+".o")
			args := []string{"-std=c11", "-Wall", "-Wextra", "-Werror", level, "-c", file, "-o", object}
			if strings.Contains(sourceText, "vba_test_malloc") {
				args = append(args, "-include", hooks)
			}
			command(t, clang, args...)
			objects = append(objects, object)
		}
		binary := filepath.Join(dir, "abi"+level)
		args := append([]string{"-std=c11", "-Wall", "-Wextra", "-Werror", level, "-I", include, source}, objects...)
		out, err := exec.Command(clang, append(args, "-lm", "-o", binary)...).CombinedOutput()
		if err != nil {
			t.Fatalf("compile: %v\n%s", err, out)
		}
		out, err = exec.Command(binary).CombinedOutput()
		if err != nil {
			t.Fatalf("ABI/ownership: %v\n%s", err, out)
		}
		outputs = append(outputs, out)
	}
	return outputs
}

const abiTest = `
#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>
static int live, fail_next, fail_after=-1;
void *vba_test_malloc(size_t n) {
    if (fail_next) { fail_next=0; return NULL; }
    if (fail_after>=0 && fail_after--==0) { fail_after=-1; return NULL; }
    void *p=malloc(n); if(p) live++; return p;
}
void vba_test_free(void *p) { if(p) live--; free(p); }
#include "runtime_internal.h"
#include <string.h>
#include <math.h>
#include <limits.h>
#define CHECK(c) do { if(!(c)) { fprintf(stderr,"line %d: %s\n",__LINE__,#c); return 1; } } while(0)
int main(void) {
    CHECK(sizeof(VbaVariant)==24 && _Alignof(VbaVariant)==8);
    CHECK(offsetof(VbaVariant,vt)==0 && offsetof(VbaVariant,value)==8);
    uint16_t units[]={49,0,50,0xd83d,0xde00};
    VbaString s={.data=units,.length=5}, snapshot={0};
    VbaVariant a={0}, b={0}, alias={0}, number_value={0};
    CHECK(!vba_variant_box_string(&a,&s));
    CHECK(a.vt==8 && !a.reserved1 && !a.reserved2 && !a.reserved3);
    uint32_t bytes=0; memcpy(&bytes,(char*)a.value.pointer-4,4);
    CHECK(bytes==10 && ((uint16_t*)a.value.pointer)[5]==0);
    CHECK(!vba_variant_load(&b,&a));
    CHECK(b.value.pointer!=a.value.pointer && !memcmp(a.value.pointer,b.value.pointer,10));
    fail_next=1;
    CHECK(vba_variant_store(&b,&a)==14 && b.vt==8);
    CHECK(!memcmp(a.value.pointer,b.value.pointer,10));
    int32_t n=12;
    alias.vt=0x4003; alias.value.pointer=&n;
    CHECK(!vba_variant_box_number(&number_value,2,21));
    CHECK(!vba_variant_store(&alias,&number_value) && n==21 && alias.vt==0x4003);
    CHECK(vba_variant_store(&alias,&a)==13 && n==21);
    VbaVariant null_value={.vt=1};
    CHECK(vba_variant_store(&alias,&null_value)==94 && n==21);
    CHECK(!vba_variant_load(&number_value,&alias) && number_value.vt==3 && number_value.value.i4==21);
    int16_t small=3; alias.vt=0x4002; alias.value.pointer=&small;
    CHECK(!vba_variant_box_number(&number_value,3,32768));
    CHECK(vba_variant_store(&alias,&number_value)==6 && small==3);
    vba_variant_clear(&alias); CHECK(small==3);
    fail_next=1;
    CHECK(vba_string_pin(&s)==14 && s.length==5 && s.data==units);
    CHECK(!vba_string_pin(&s) && s.length==-2);
    CHECK(!vba_string_load(&snapshot,&s));
    alias.vt=0x4008; alias.value.pointer=&s.data;
    CHECK(!vba_variant_box_number(&number_value,3,42));
    CHECK(!vba_variant_store(&alias,&number_value));
    CHECK(vba_internal_slen(&s)==2 && s.data[0]==52 && s.data[1]==50);
    CHECK(snapshot.length==5 && snapshot.data[1]==0);
    VbaString next={.unit=55,.length=1};
    uint16_t *old=s.data;
    fail_next=1;
    CHECK(vba_string_store(&s,&next)==14 && s.data==old);
    CHECK(!vba_string_store(&s,&next) && s.length==-2 && s.data[0]==55);
    CHECK(!vba_variant_load(&number_value,&alias) && number_value.vt==8);
    CHECK(number_value.value.pointer!=s.data);
    VbaVariant indirect={.vt=0x400c,.value.pointer=&number_value};
    CHECK(vba_variant_query(&indirect,1)==8);
    CHECK(!vba_variant_store(&indirect,&null_value) && number_value.vt==1);
    vba_variant_clear(&a); vba_variant_clear(&b); vba_variant_clear(&number_value);
    vba_variant_clear(&alias); vba_variant_clear(&indirect);
    vba_internal_string_drop(s); vba_internal_string_drop(snapshot);
    CHECK(live==0);
    return 0;
}
`
