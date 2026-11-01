package llvm_test

import (
	"strings"
	"testing"
)

func TestArrayABIAndOwnership(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+arrayABITest)
}

const arrayABITest = `
int main(void) {
    CHECK(sizeof(VbaArray)==32 && _Alignof(VbaArray)==8);
    CHECK(offsetof(VbaArray,dimensions)==0 && offsetof(VbaArray,features)==2);
    CHECK(offsetof(VbaArray,element_size)==4 && offsetof(VbaArray,locks)==8);
    CHECK(offsetof(VbaArray,data)==16 && offsetof(VbaArray,bounds)==24);
    VbaArray *a=NULL, *copy=NULL;
    int32_t bounds[]={-1,0,2,4}, indices[]={0,3};
    CHECK(!vba_array_redim(&a,3,3,0,2,bounds));
    CHECK(a->dimensions==2 && a->element_size==4 && a->features==0x80);
    CHECK(a->bounds[0].count==3 && a->bounds[0].lower==2);
    CHECK(a->bounds[1].count==2 && a->bounds[1].lower==-1);
    VbaElement element={0};
    CHECK(!vba_array_index(&element,&a,3,3,2,indices) && a->locks==1);
    CHECK(element.data==(int32_t*)a->data+3);
    *(int32_t*)vba_element_address(&element,3)=42;
    VbaArray *old=a;
    CHECK(vba_array_redim(&a,3,3,0,2,bounds)==10 && a==old);
    CHECK(vba_array_store(&a,NULL)==10 && a==old);
    CHECK(vba_array_destroy(a)==10);
    vba_element_clear(&element); CHECK(a->locks==0);
    CHECK(!vba_array_copy(&copy,a) && copy!=a && copy->data!=a->data);
    CHECK(((int32_t*)copy->data)[3]==42);
    ((int32_t*)copy->data)[3]=99;
    CHECK(((int32_t*)a->data)[3]==42);
    vba_array_clear(&copy);
    bounds[3]=5;
    fail_next=1;
    CHECK(vba_array_redim(&a,3,3,1,2,bounds)==7 && a==old);
    CHECK(((int32_t*)a->data)[3]==42 && a->bounds[0].count==3);
    CHECK(!vba_array_redim(&a,3,3,1,2,bounds));
    CHECK(((int32_t*)a->data)[3]==42 && ((int32_t*)a->data)[7]==0);
    old=a; bounds[0]=-2;
    CHECK(vba_array_redim(&a,3,3,1,2,bounds)==9 && a==old);
    bounds[0]=-1; bounds[1]=1;
    CHECK(vba_array_redim(&a,3,3,1,2,bounds)==9 && a==old);
    bounds[1]=0;
    VbaVariant boxed={0}, view={.vt=0x6003,.value.pointer=&a};
    CHECK(!vba_variant_load(&boxed,&view) && boxed.vt==0x2003);
    CHECK(boxed.value.pointer!=a && vba_variant_query(&view,1)==0x2003);
    CHECK(vba_variant_query(&view,5)==-1);
    CHECK(vba_array_redim(&view,0,8,0,2,bounds)==13 && a==old);
    CHECK(!vba_array_index(&element,&a,3,3,2,indices));
    CHECK(vba_variant_store(&view,&boxed)==10);
    vba_element_clear(&element);
    vba_variant_clear(&boxed); vba_variant_clear(&view);
    vba_array_clear(&a);
    int32_t one[]={1,2};
    CHECK(!vba_array_redim(&a,8,8,0,1,one));
    indices[0]=1;
    CHECK(!vba_array_index(&element,&a,8,8,1,indices));
    VbaString source={.data=(uint16_t[]){49,0,50},.length=3};
    CHECK(!vba_string_store(vba_element_address(&element,8),&source));
    CHECK(((uint16_t**)a->data)[0][1]==0);
    vba_element_clear(&element);
    CHECK(!vba_array_copy(&copy,a));
    CHECK(((void**)a->data)[0]!=((void**)copy->data)[0]);
    old=copy;
    for(int stage=0;stage<3;stage++) {
        int before=live;
        fail_after=stage;
        CHECK(vba_array_store(&copy,a)==(stage==2 ? 14 : 7) && copy==old);
        CHECK(live==before && ((uint16_t**)a->data)[0][2]==50);
    }
    vba_array_clear(&copy); vba_array_clear(&a);
    CHECK(!vba_array_redim(&a,12,12,2,1,one));
    CHECK(a->features&A_FIXED);
    CHECK(vba_array_redim(&a,12,12,0,1,one)==10);
    CHECK(!vba_array_index(&element,&a,12,12,1,indices));
    VbaVariant text={0}; CHECK(!vba_variant_box_string(&text,&source));
    CHECK(!vba_variant_store(vba_element_address(&element,12),&text));
    vba_element_clear(&element); vba_variant_clear(&text);
    CHECK(!vba_array_copy(&copy,a) && !(copy->features&A_FIXED));
    VbaVariant *src=a->data, *dst=copy->data;
    CHECK(src[0].vt==8 && dst[0].vt==8 && src[0].value.pointer!=dst[0].value.pointer);
    CHECK(!vba_array_erase(&a,12) && a && src[0].vt==0);
    vba_array_clear(&a); vba_array_clear(&copy);
    CHECK(live==0);
    return 0;
}
`
