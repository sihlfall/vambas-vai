package llvm_test

import (
	"fmt"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/stdlib"
)

func TestNativeAllLibraryConstants(t *testing.T) {
	// The catalog values are independently checked against the pinned source
	// index by stdlib's inventory test. Here check their materialization through
	// every supported qualification, including an enum used as an annotation.
	var source strings.Builder
	source.WriteString("Function Run() As Long\n Dim flags As VBA.VbMsgBoxStyle\n")
	for _, c := range stdlib.Constants() {
		if c.Value.Kind != stdlib.NumericLiteral {
			continue
		}
		for _, prefix := range []string{"", "VBA.", c.Owner + ".", "VBA." + c.Owner + "."} {
			fmt.Fprintf(&source, " If %s%s<>%s Then Exit Function\n", prefix, c.Name, c.Value.Text)
		}
	}
	source.WriteString(` If VBA.Constants.vbNewLine<>vbCr & vbLf Then Exit Function
 If Len(VBA.Constants.vbNullString)<>0 Then Exit Function
 If AscW(VBA.Constants.vbNullChar)<>0 Then Exit Function
 If AscW(vbBack)<>8 Or AscW(vbTab)<>9 Then Exit Function
 If AscW(vbLf)<>10 Or AscW(vbVerticalTab)<>11 Then Exit Function
 If AscW(vbFormFeed)<>12 Or AscW(vbCr)<>13 Then Exit Function
 If vbCrLf<>vbNewLine Then Exit Function
 If VBA.SystemColorConstants.vbScrollBars<>-2147483648 Then Exit Function
 flags=vbYesNo Or vbQuestion Or vbDefaultButton2
 If flags<>292 Then Exit Function
 Run=42
End Function`)
	variantResult(t, source.String(), 42)
}
