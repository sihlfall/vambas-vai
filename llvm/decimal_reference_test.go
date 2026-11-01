package llvm_test

import (
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"testing"
)

func decimalText(coefficient *big.Int, scale int) string {
	negative := coefficient.Sign() < 0
	text := new(big.Int).Abs(coefficient).String()
	if scale != 0 {
		if len(text) <= scale {
			text = strings.Repeat("0", scale+1-len(text)) + text
		}
		text = text[:len(text)-scale] + "." + text[len(text)-scale:]
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if negative && text != "0" {
		text = "-" + text
	}
	return text
}

// The reference uses arbitrary-precision rational arithmetic, independently of
// the runtime's fixed-size limbs, long division and discarded-digit bookkeeping.
func decimalReference(value *big.Rat) (string, bool) {
	abs := new(big.Int).Abs(value.Num())
	for scale := 28; scale >= 0; scale-- {
		power := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
		numerator := new(big.Int).Mul(abs, power)
		q, rem := new(big.Int), new(big.Int)
		q.QuoRem(numerator, value.Denom(), rem)
		cmp := new(big.Int).Lsh(rem, 1).Cmp(value.Denom())
		if cmp > 0 || cmp == 0 && q.Bit(0) != 0 {
			q.Add(q, big.NewInt(1))
		}
		if q.BitLen() <= 96 {
			if value.Sign() < 0 {
				q.Neg(q)
			}
			return decimalText(q, scale), true
		}
	}
	return "", false
}

func TestDecimalArithmeticAgainstRationalReference(t *testing.T) {
	var code strings.Builder
	code.WriteString(strings.Split(abiTest, "int main(void)")[0])
	code.WriteString(`
static int make_decimal(VbaVariant *out,const char *text) {
 uint16_t units[100];size_t n=strlen(text);if(n>100)return 13;
 for(size_t i=0;i<n;i++)units[i]=(unsigned char)text[i];
 VbaString string_value={.data=units,.length=(int32_t)n,.unit=n?units[0]:0};
 VbaVariant input={0};int e=vba_variant_box_string(&input,&string_value);if(!e)e=vba_cdec(out,&input);vba_variant_clear(&input);return e;
}
static int matches(const VbaVariant *value,const char *expected) {
 VbaVariant text={0};if(vba_internal_convert(&text,value,V_BSTR))return 0;
 int equal=vba_internal_bstr_len(text.value.pointer)==strlen(expected);uint16_t *units=text.value.pointer;
 for(uint32_t i=0;equal && i<vba_internal_bstr_len(units);i++)equal=units[i]==(unsigned char)expected[i];
 vba_variant_clear(&text);return equal;
}
int main(void) { VbaVariant a={0},b={0},out={0};
`)
	random := rand.New(rand.NewSource(173))
	for i := 0; i < 40; i++ {
		coefficient := func() *big.Int {
			n := new(big.Int).SetUint64(uint64(random.Uint32()))
			n.Lsh(n, 64).Add(n, new(big.Int).SetUint64(random.Uint64()))
			if random.Intn(2) == 0 {
				n.Neg(n)
			}
			return n
		}
		a, b := coefficient(), coefficient()
		sa, sb := random.Intn(29), random.Intn(29)
		x, _ := new(big.Rat).SetString(decimalText(a, sa))
		y, _ := new(big.Rat).SetString(decimalText(b, sb))
		fmt.Fprintf(&code, "CHECK(!make_decimal(&a,%q)); CHECK(!make_decimal(&b,%q));\n", decimalText(a, sa), decimalText(b, sb))
		for _, op := range []int{1, 2, 3, 5} {
			result := new(big.Rat)
			switch op {
			case 1:
				result.Add(x, y)
			case 2:
				result.Sub(x, y)
			case 3:
				result.Mul(x, y)
			case 5:
				result.Quo(x, y)
			}
			expected, ok := decimalReference(result)
			if ok {
				fmt.Fprintf(&code, "CHECK(!vba_variant_binary(&out,&a,&b,%d) && matches(&out,%q));\n", op, expected)
			} else {
				fmt.Fprintf(&code, "out=(VbaVariant){0}; CHECK(vba_variant_binary(&out,&a,&b,%d)==6 && out.vt==0);\n", op)
			}
		}
	}
	code.WriteString("CHECK(live==0);return 0;}\n")
	nativeRuntimeC(t, code.String())
}
