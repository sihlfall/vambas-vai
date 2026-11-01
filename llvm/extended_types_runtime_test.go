package llvm_test

import (
	"strings"
	"testing"
)

func TestExtendedNumericRuntimeABIAndBoundaries(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+`
static int from_text(VbaVariant *out,const char *text,int tag) {
    uint16_t units[100];size_t n=strlen(text);if(n>100)return 13;
    for(size_t i=0;i<n;i++)units[i]=(unsigned char)text[i];
    VbaString string_value={.data=units,.length=(int32_t)n,.unit=n?units[0]:0};
    VbaVariant input={0};int status=vba_variant_box_string(&input,&string_value);
    if(!status)status=vba_internal_convert(out,&input,(uint16_t)tag);
    vba_variant_clear(&input);return status;
}
static int text_equal(const VbaVariant *value,const char *expected) {
    VbaVariant text={0};if(vba_internal_convert(&text,value,V_BSTR))return 0;
    int same=vba_internal_bstr_len(text.value.pointer)==strlen(expected);uint16_t *units=text.value.pointer;
    for(uint32_t i=0;same && i<vba_internal_bstr_len(units);i++)same=units[i]==(unsigned char)expected[i];
    vba_variant_clear(&text);return same;
}
int main(void) {
    CHECK(sizeof(((VbaVariant*)0)->decimal)==16 && offsetof(VbaVariant,decimal.scale)==2 && offsetof(VbaVariant,decimal.hi)==4 && offsetof(VbaVariant,decimal.lo)==8);
    VbaVariant a={0},b={0},out={0};
    CHECK(!from_text(&a,"922337203685477.5807",V_CY) && a.value.i8==INT64_MAX);
    CHECK(!from_text(&b,"-922337203685477.5808",V_CY) && b.value.i8==INT64_MIN);
    CHECK(from_text(&out,"922337203685477.5808",V_CY)==6 && out.vt==0);
    CHECK(from_text(&out,"-922337203685477.5809",V_CY)==6 && out.vt==0);
    CHECK(!from_text(&out,"1.23445",V_CY) && out.value.i8==12344);
    CHECK(!from_text(&out,"1.23455",V_CY) && out.value.i8==12346);
    CHECK(!from_text(&out,"-1.23455",V_CY) && out.value.i8==-12346);
    CHECK(vba_variant_unary(&out,&b,1)==6 && out.value.i8==-12346);
    struct {int64_t value;uint64_t sentinel;} money={12345,0xabcdef};
    VbaVariant ref={.vt=V_BYREF|V_CY,.value.pointer=&money.value};
    CHECK(!vba_variant_load(&out,&ref) && out.vt==V_CY && out.value.i8==12345);
    CHECK(!vba_variant_store(&ref,&a) && money.value==INT64_MAX && money.sentinel==0xabcdef);
    VbaVariant error={.vt=V_ERROR,.value.error=1};
    CHECK(vba_variant_store(&ref,&error)==13 && money.value==INT64_MAX);
    CHECK(!from_text(&b,"0.0001",V_CY));
    CHECK(vba_variant_binary(&out,&a,&b,1)==6 && out.value.i8==12345);
    CHECK(!vba_variant_binary(&out,&a,&b,2) && out.value.i8==INT64_MAX-1);
    CHECK(text_equal(&out,"922337203685477.5806"));

    CHECK(!from_text(&a,"79228162514264337593543950335",V_DECIMAL));
    CHECK(a.vt==V_DECIMAL && a.decimal.hi==UINT32_MAX && a.decimal.lo==UINT64_MAX && a.decimal.scale==0);
    CHECK(!from_text(&b,"1",V_DECIMAL));
    CHECK(!vba_variant_binary(&out,&a,&b,2));
    CHECK(text_equal(&out,"79228162514264337593543950334"));
    CHECK(vba_variant_binary(&out,&a,&b,1)==6 && text_equal(&out,"79228162514264337593543950334"));
    CHECK(from_text(&out,"79228162514264337593543950336",V_DECIMAL)==6);
    CHECK(!from_text(&a,"0.0000000000000000000000000001",V_DECIMAL) && a.decimal.scale==28 && a.decimal.lo==1);
    CHECK(!from_text(&b,"0.5",V_DECIMAL));
    CHECK(!vba_variant_binary(&out,&a,&b,3) && text_equal(&out,"0"));
    CHECK(!from_text(&a,"0.0000000000000000000000000003",V_DECIMAL));
    CHECK(!vba_variant_binary(&out,&a,&b,3) && text_equal(&out,"0.0000000000000000000000000002"));
    CHECK(!from_text(&a,"1",V_DECIMAL) && !from_text(&b,"6",V_DECIMAL));
    CHECK(!vba_variant_binary(&out,&a,&b,5) && text_equal(&out,"0.1666666666666666666666666667"));
    CHECK(!from_text(&a,"79228162514264337593543950335",V_DECIMAL));
    CHECK(!from_text(&b,"0.0000000000000000000000000001",V_DECIMAL));
    CHECK(!vba_variant_binary(&out,&a,&b,3) && text_equal(&out,"7.9228162514264337593543950335"));
    CHECK(vba_variant_binary(&out,&a,&b,5)==6);
    VbaVariant decimal_ref={.vt=V_BYREF|V_DECIMAL,.value.pointer=&a.decimal};
    CHECK(!vba_variant_load(&out,&decimal_ref) && out.decimal.lo==UINT64_MAX && out.decimal.hi==UINT32_MAX);
    VbaVariant native_decimal={0};
    CHECK(!vba_variant_to(&native_decimal.decimal,&a,V_DECIMAL) && native_decimal.decimal.reserved==0);
    decimal_ref.value.pointer=&native_decimal.decimal;
    CHECK(!vba_variant_load(&out,&decimal_ref) && out.vt==V_DECIMAL && out.decimal.hi==UINT32_MAX);
    CHECK(!from_text(&b,"-1.23",V_DECIMAL));
    CHECK(!vba_variant_store(&decimal_ref,&b) && native_decimal.decimal.reserved==0);
    CHECK(!vba_variant_load(&out,&decimal_ref) && text_equal(&out,"-1.23"));
    const unsigned char expected_decimal[16]={14,0,2,128,0,0,0,0,123,0,0,0,0,0,0,0};
    CHECK(!memcmp(&out,expected_decimal,16));
    VbaVariant invalid=a;invalid.decimal.scale=29;
    CHECK(vba_variant_load(&out,&invalid)==13);

    VbaVariant date={0};
    CHECK(!from_text(&date,"2024-02-29 12:30:15",V_DATE) && date.vt==V_DATE);
    CHECK(text_equal(&date,"2024-02-29 12:30:15"));
    CHECK(from_text(&out,"2023-02-29",V_DATE)==13);
    CHECK(from_text(&out,"2024-01-01 24:00:00",V_DATE)==13);
    CHECK(!from_text(&date,"0100-01-01",V_DATE) && date.value.r8==-657434);
    CHECK(!from_text(&date,"9999-12-31 23:59:59",V_DATE) && text_equal(&date,"9999-12-31 23:59:59"));
    double serial=1.25;VbaVariant date_ref={.vt=V_BYREF|V_DATE,.value.pointer=&serial};
    CHECK(!vba_variant_load(&out,&date_ref) && out.vt==V_DATE && out.value.r8==1.25);
    VbaVariant n={.vt=V_R8,.value.r8=2958466};
    CHECK(vba_variant_store(&date_ref,&n)==6 && serial==1.25);
    CHECK(vba_convert_explicit(&serial,&error,V_DATE)==13 && serial==1.25);
    n.value.r8=-1.25;CHECK(!vba_variant_store(&date_ref,&n) && serial==-1.25);
    double number_value=0;CHECK(!vba_variant_to(&number_value,&date_ref,V_R8) && number_value==-1.25);
    VbaArray *array=NULL;int32_t bounds[]={0,1};
    CHECK(!vba_array_redim(&array,V_DATE,V_DATE,0,1,bounds) && array->element_size==8);
    vba_array_clear(&array);
    CHECK(!vba_array_redim(&array,V_CY,V_CY,0,1,bounds) && array->element_size==8);
    vba_array_clear(&array);
    VbaVariant currency_sample={.vt=V_CY,.value.i8=12345};
    VbaVariant samples[]={date,currency_sample,a};
    for(int i=0;i<3;i++) {
        int success=0;
        for(int at=0;at<5;at++) {
            fail_after=at;VbaString text={0};
            int status=vba_convert_explicit(&text,samples+i,V_BSTR);fail_after=-1;
            if(status)CHECK(status==14 && !text.data && !text.owner && text.length==0);
            else {success=1;vba_internal_string_drop(text);}
            CHECK(live==0);
        }
        CHECK(success);
    }
    vba_variant_clear(&a);vba_variant_clear(&b);vba_variant_clear(&out);vba_variant_clear(&date);
    CHECK(live==0);return 0;
}
`)
}
