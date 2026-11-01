package llvm_test

import (
	"strings"
	"testing"
)

func TestControlRuntimeOwnershipAndFailure(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+`
int main(void) {
    VbaArray *a=NULL;
    int32_t bounds[]={0,1},indices[]={0};
    CHECK(!vba_array_redim(&a,8,8,0,1,bounds));
    VbaElement cell={0};
    CHECK(!vba_array_index(&cell,&a,8,8,1,indices));
    VbaString text={.data=(uint16_t[]){49,50},.length=2};
    CHECK(!vba_string_store(vba_element_address(&cell,8),&text));
    vba_element_clear(&cell);
    VbaEnumerator state={0};
    CHECK(!vba_enum_start(&state,&a,8,0) && a->locks==1);
    VbaVariant counter={0}; int16_t has_value=0;
    CHECK(!vba_variant_box_number(&counter,3,7));
    fail_next=1;
    CHECK(vba_enum_next(&has_value,&state,&counter)==14);
    CHECK(state.index==0 && counter.vt==3 && counter.value.i4==7);
    CHECK(!vba_enum_next(&has_value,&state,&counter) && has_value==-1);
    CHECK(counter.vt==8 && vba_internal_bstr_len(counter.value.pointer)==2);
    vba_variant_clear(&counter);
    CHECK(vba_array_redim(&a,8,8,0,1,bounds)==10);
    vba_enum_clear(&state); CHECK(a->locks==0);
    fail_next=1;
    CHECK(vba_enum_start(&state,&a,8,1)==7 && !state.initialized && a->locks==0);
    CHECK(!vba_enum_start(&state,&a,8,1));
    CHECK(state.array!=a && state.owns && state.array->locks==1);
    vba_enum_clear(&state);
    CHECK(vba_enum_next(&has_value,&state,&counter)==92);
    VbaVariant null_value={.vt=1};
    CHECK(!vba_condition(&has_value,&null_value) && has_value==0);
    VbaVariant limit={.vt=3,.value.i4=5},step={.vt=2,.value.i2=1};
    CHECK(vba_for_validate(&null_value,&limit,&step)==13);
    vba_array_clear(&a);
    CHECK(live==0);
    return 0;
}
`)
}
