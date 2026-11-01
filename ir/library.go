package ir

import "sihlfall.ch/vambas-vai/stdlib"

// LibrarySignature describes prepared operands independently of the native ABI.
// Every argument in the initial SnapshotCall contract is a Variant value. The
// catalog retains the declared formal types used by checker/lowering coercions.
type LibrarySignature struct {
	Name    string
	Arity   int
	Result  Type
	Effects Effects // Base effects; default-member and termination effects add to these.
}

var librarySignatures = makeLibrarySignatures()

// LookupLibraryCall returns an immutable value and rejects specialized or
// unavailable declarations, constants, and unknown IDs.
func LookupLibraryCall(id stdlib.MemberID) (LibrarySignature, bool) {
	signature, ok := librarySignatures[id]
	return signature, ok
}

func makeLibrarySignatures() map[stdlib.MemberID]LibrarySignature {
	result := make(map[stdlib.MemberID]LibrarySignature)
	for _, fn := range stdlib.Functions() {
		if fn.Call != stdlib.SnapshotCall {
			continue
		}
		// The current preparation path supports Variant values and pre-coerced
		// Long values, then boxes both. Other contracts must be added explicitly.
		valid := true
		for _, p := range fn.Parameters {
			if p.ByRef || p.Type.Array || p.Type.Kind != stdlib.TypeVariant && p.Type.Kind != stdlib.TypeLong || p.Optional && p.Default.Kind != stdlib.NumericLiteral && p.Default.Kind != stdlib.StringLiteral {
				valid = false
				break
			}
		}
		scalar := map[stdlib.TypeKind]Scalar{
			stdlib.TypeByte: Byte,
			stdlib.TypeDate: Date, stdlib.TypeCurrency: Currency,
			stdlib.TypeVariant: Variant, stdlib.TypeInteger: Integer, stdlib.TypeLong: Long,
			stdlib.TypeString: String, stdlib.TypeBoolean: Boolean,
			stdlib.TypeSingle: Single, stdlib.TypeDouble: Double,
		}[fn.Result.Kind]
		if !valid || scalar == Invalid {
			continue
		}
		t := Type{Scalar: scalar}
		if fn.Result.Array {
			t = ArrayOf(t)
		}
		effects := MayRaise
		if Managed(t) {
			effects |= ManagesLifetime
		}
		result[fn.ID] = LibrarySignature{Name: "VBA." + fn.Module + "." + fn.Name, Arity: len(fn.Parameters), Result: t, Effects: effects}
	}
	return result
}
