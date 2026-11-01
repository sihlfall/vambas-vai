package llvm_test

import "testing"

func TestNativeArrayAliasSharing(t *testing.T) {
	const source = `Function Run() As Long
 Dim values(0 To 0) As Long
 values(0) = 3
 Change values, values
 Run = values(0)
End Function
Sub Change(ByRef a() As Long, ByRef b() As Long)
 a(0) = a(0) + 1
 b(0) = b(0) + a(0)
End Sub`
	// With aliasing, the first write produces 4 and the second produces 8.
	// Separate/copied contents or a noalias assumption would change that result.
	variantResult(t, source, 8)
}
