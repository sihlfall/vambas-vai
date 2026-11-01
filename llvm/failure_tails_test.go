package llvm_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/llvm"
)

// Cleanup and handler routing are emitted once per region, and activation
// cleanup once per procedure, however many instructions can fail.
func TestFailureTailsAreShared(t *testing.T) {
	for _, policy := range []string{"", " On Error GoTo Handler\n"} {
		var body strings.Builder
		body.WriteString("Function Work(ByVal x As Long) As Long\n Dim v0, v1, v2, v3, v4, v5, v6, v7 As Variant\n" + policy)
		for i := 0; i < 8; i++ {
			// Each statement has several fallible operations.
			fmt.Fprintf(&body, " v%d = x * %d + x * %d + x * %d\n", i, i+1, i+2, i+3)
		}
		body.WriteString(" Work = v0 + v7\n")
		if policy != "" {
			body.WriteString(" Exit Function\nHandler:\n Work = -1\n")
		}
		body.WriteString("End Function\n")
		text := emit(t, lower(t, body.String()), llvm.Options{})
		fails := len(regexp.MustCompile(`(?m)^fail\d+:`).FindAllString(text, -1))
		tails := len(regexp.MustCompile(`(?m)^fail\.s\d+:`).FindAllString(text, -1))
		clears := strings.Count(text, "call void @vba_variant_clear(")
		if strings.Count(text, "\nunwind:\n") != 1 || tails == 0 || tails >= fails {
			t.Fatalf("policy %q: %d failure sites, %d tails\n%s", policy, fails, tails, text)
		}
		// Clears happen at normal return, in unwind, and per region tail.
		// Per-site cleanup would emit at least eight for every failure site.
		if clears >= fails {
			t.Fatalf("policy %q: %d Variant clears for %d failure sites\n%s", policy, clears, fails, text)
		}
	}
}
