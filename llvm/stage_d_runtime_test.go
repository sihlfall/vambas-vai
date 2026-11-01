package llvm_test

import (
	"strings"
	"testing"
)

func TestStageDConversionRuntimeABIAndFailures(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+`
int main(void) {
    VbaVariant error_value={.vt=V_ERROR,.value.error=255},out={0};
    struct { uint8_t value, sentinel[7]; } byte_cell={7,{1,2,3,4,5,6,7}};
    CHECK(!vba_convert_explicit(&byte_cell.value,&error_value,V_UI1) && byte_cell.value==255);
    for(int i=0;i<7;i++) CHECK(byte_cell.sentinel[i]==i+1);
    error_value.value.error=256;
    CHECK(vba_convert_explicit(&byte_cell.value,&error_value,V_UI1)==6 && byte_cell.value==255);
    CHECK(vba_variant_to(&byte_cell.value,&error_value,V_UI1)==13 && byte_cell.value==255);
    VbaVariant byte_view={.vt=V_BYREF|V_UI1,.value.pointer=&byte_cell.value};
    CHECK(vba_variant_store(&byte_view,&error_value)==13 && byte_cell.value==255);
    VbaVariant boolean={.vt=V_BOOL,.value.boolean=-1};
    CHECK(!vba_convert_explicit(&byte_cell.value,&boolean,V_UI1) && byte_cell.value==255);
    CHECK(!vba_variant_to(&byte_cell.value,&boolean,V_UI1) && byte_cell.value==255);
    CHECK(!vba_variant_store(&byte_view,&boolean) && byte_cell.value==255);
    boolean.value.boolean=0;
    CHECK(!vba_convert_explicit(&byte_cell.value,&boolean,V_UI1) && byte_cell.value==0);
    CHECK(!vba_variant_store(&byte_view,&boolean) && byte_cell.value==0);
    for(int i=0;i<7;i++) CHECK(byte_cell.sentinel[i]==i+1);

    struct { int32_t value; uint32_t sentinel; } code={2001,0x12345678};
    VbaVariant view={.vt=V_BYREF|V_ERROR,.value.pointer=&code.value};
    int16_t predicate=0;
    CHECK(!vba_iserror(&predicate,&view) && predicate==-1);
    CHECK(!vba_cverr(&out,&view) && out.vt==V_ERROR && out.value.error==2001);
    CHECK(!out.reserved1 && !out.reserved2 && !out.reserved3);
    CHECK(code.value==2001 && code.sentinel==0x12345678);
    int32_t number_value=7;
    CHECK(!vba_convert_explicit(&number_value,&view,V_I4) && number_value==2001);
    CHECK(vba_variant_to(&number_value,&view,V_I4)==13 && number_value==2001);
    VbaVariant wrapper={.vt=V_BYREF|V_VARIANT,.value.pointer=&view};
    CHECK(!vba_iserror(&predicate,&wrapper) && predicate==-1);

    VbaVariant missing={.vt=V_ERROR,.value.error=(int32_t)0x80020004};
    CHECK(!vba_cverr(&out,&missing) && out.value.error==missing.value.error);
    CHECK(vba_variant_query(&out,4)==-1);
    CHECK(!vba_convert_explicit(&number_value,&out,V_I4) && number_value==missing.value.error);
    CHECK(!vba_iserror(&predicate,&missing) && predicate==-1);
    VbaVariant null_value={.vt=V_NULL},empty={0},array={.vt=V_ARRAY|V_UI1};
    CHECK(!vba_iserror(&predicate,&null_value) && predicate==0);
    CHECK(!vba_iserror(&predicate,&empty) && predicate==0);
    CHECK(!vba_iserror(&predicate,&array) && predicate==0);

    VbaVariant n={.vt=V_R8};
    const double inputs[]={-0.5,0.5,1.5,2.5,254.5,255.499};
    const int expected[]={0,0,2,2,254,255};
    for(int i=0;i<6;i++) {
        n.value.r8=inputs[i];
        CHECK(!vba_convert_explicit(&byte_cell.value,&n,V_UI1) && byte_cell.value==expected[i]);
    }
    const double invalid[]={-0.501,255.5,INFINITY,-INFINITY,NAN};
    for(int i=0;i<5;i++) {
        n.value.r8=invalid[i];byte_cell.value=19;
        CHECK(vba_convert_explicit(&byte_cell.value,&n,V_UI1)==6 && byte_cell.value==19);
    }
    n.value.r8=65535.49;
    CHECK(!vba_cverr(&out,&n) && out.value.error==65535);
    n.value.r8=65535.5;
    CHECK(vba_cverr(&out,&n)==5 && out.value.error==65535);
    n.value.r8=-1;
    CHECK(vba_cverr(&out,&n)==5 && out.value.error==65535);
    n.value.r8=1e30;
    CHECK(vba_cverr(&out,&n)==6 && out.value.error==65535);
    CHECK(vba_cverr(&out,&null_value)==94 && out.value.error==65535);

    /* Every Error arithmetic operand fails before Null can propagate. */
    for(int op=1;op<=7;op++) {
        CHECK(vba_variant_binary(&out,&error_value,&null_value,op)==13 && out.value.error==65535);
        CHECK(vba_variant_binary(&out,&null_value,&error_value,op)==13 && out.value.error==65535);
        CHECK(vba_variant_binary(&out,&error_value,&error_value,op)==13 && out.value.error==65535);
    }
    CHECK(vba_variant_unary(&out,&error_value,1)==13 && out.value.error==65535);
    VbaVariant other={.vt=V_ERROR,.value.error=257};
    for(int op=1;op<=6;op++) {
        int want=op==2 || op==3 || op==4;
        CHECK(!vba_variant_compare(&out,&error_value,&other,op,0) && out.vt==V_BOOL && out.value.boolean==(want?-1:0));
        CHECK(vba_variant_compare(&out,&error_value,&null_value,op,0)==13);
        CHECK(vba_variant_compare(&out,&missing,&missing,op,0)==13);
    }

    /* Exercise every allocation failure in CStr(Error), including cleanup of
       the intermediate BSTR if the final private String allocation fails. */
    int success=0;
    for(int at=0;at<5;at++) {
        fail_after=at;
        VbaString text={0};
        int status=vba_convert_explicit(&text,&missing,V_BSTR);
        fail_after=-1;
        if(status) CHECK(status==14 && !text.data && !text.owner && text.length==0);
        else {
            const char *expected_text="Error -2147352572";
            CHECK(vba_internal_slen(&text)==(int)strlen(expected_text));
            for(int i=0;i<vba_internal_slen(&text);i++) CHECK(vba_internal_unit(&text,i)==(unsigned char)expected_text[i]);
            success=1;vba_internal_string_drop(text);
        }
        CHECK(live==0 && missing.value.error==(int32_t)0x80020004);
    }
    CHECK(success);
    vba_variant_clear(&out);
    CHECK(live==0);
    return 0;
}
`)
}
