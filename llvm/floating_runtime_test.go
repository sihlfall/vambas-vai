package llvm_test

import (
	"strings"
	"testing"
)

func TestFloatingRuntimeABI(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+`
int main(void) {
    struct { float n; uint32_t sentinel; } cell={1.5f,0x12345678};
    VbaVariant ref={.vt=V_BYREF|V_R4,.value.pointer=&cell.n},value={0},copy={0};
    CHECK(!vba_variant_load(&copy,&ref) && copy.vt==V_R4 && copy.value.r4==1.5f);
    CHECK(!vba_variant_box_float(&value,V_R8,2.5));
    CHECK(!vba_variant_store(&ref,&value) && cell.n==2.5f && cell.sentinel==0x12345678);
    value.value.r8=1e100;
    CHECK(vba_variant_store(&ref,&value)==6 && cell.n==2.5f);
    double d=1.25;
    ref.vt=V_BYREF|V_R8;ref.value.pointer=&d;
    CHECK(!vba_variant_store(&ref,&copy) && d==1.5);
    CHECK(!vba_variant_to(&cell.n,&copy,V_R4) && cell.n==1.5f);
    VbaArray *a=NULL;int32_t bounds[]={0,1};
    CHECK(!vba_array_redim(&a,V_R4,V_R4,0,1,bounds) && a->element_size==4);
    CHECK(!vba_array_box(&value,a,V_R4) && value.vt==(V_ARRAY|V_R4));
    vba_variant_clear(&value);vba_array_clear(&a);
    CHECK(!vba_array_redim(&a,V_R8,V_R8,0,1,bounds) && a->element_size==8);
    vba_array_clear(&a);
    vba_variant_clear(&copy);
    CHECK(live==0);
    return 0;
}
`)
}
