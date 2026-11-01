package llvm_test

import (
	"strings"
	"testing"
)

func TestRecordArrayAtomicCopiesLocksAndFailures(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+`
typedef struct {int32_t n;VbaString text;VbaVariant value;VbaArray *children;} Item;
static const VbaRecordField fields[]={
 {R_SCALAR,V_I4,offsetof(Item,n),4,NULL,0,NULL},
 {R_STRING,V_BSTR,offsetof(Item,text),sizeof(VbaString),NULL,0,NULL},
 {R_VARIANT,V_VARIANT,offsetof(Item,value),sizeof(VbaVariant),NULL,0,NULL},
 {R_ARRAY,V_BSTR,offsetof(Item,children),sizeof(void*),NULL,0,NULL}
};
static const VbaRecordType item_type={sizeof(Item),4,fields};
int main(void){
 VbaArray *a=NULL;int32_t bounds[]={1,2};
 CHECK(!vba_record_array_redim(&a,&item_type,0,1,bounds));
 CHECK(a->element_size==sizeof(Item) && vba_internal_array_tag(a)==36 && vba_internal_array_record_type(a)==&item_type);
 Item *items=a->data;items[0].n=42;
 VbaString text={.data=(uint16_t[]){'a',0,'b'},.length=3};
 CHECK(!vba_string_store(&items[0].text,&text));
 CHECK(!vba_variant_box_string(&items[0].value,&text));
 CHECK(!vba_array_redim(&items[0].children,V_BSTR,V_BSTR,0,1,bounds));
 ((uint16_t**)items[0].children->data)[0]=vba_internal_bstr_alloc((uint16_t[]){'x'},1);
 int baseline=live,succeeded=0;
 for(int stage=0;stage<32;stage++){
  VbaArray *copy=NULL;fail_after=stage;
  int error=vba_array_copy(&copy,a);fail_after=-1;
  if(error){CHECK(copy==NULL && live==baseline && items[0].n==42);continue;}
  succeeded=1;Item *copied=copy->data;
  CHECK(copied[0].n==42 && copied[0].children!=items[0].children);
  CHECK(vba_internal_slen(&copied[0].text)==3 && vba_internal_unit(&copied[0].text,1)==0);
  CHECK(!vba_array_destroy(copy));CHECK(live==baseline);break;
 }
 CHECK(succeeded);
 int32_t index=1;VbaElement element={0};
 CHECK(!vba_record_array_index(&element,&a,&item_type,1,&index));
 CHECK(vba_element_address(&element,36)==a->data && a->locks==1);
 int32_t bigger[]={1,3};
 CHECK(vba_record_array_redim(&a,&item_type,1,1,bigger)==10);
 vba_element_clear(&element);CHECK(a->locks==0);
 succeeded=0;
 for(int stage=0;stage<32;stage++){
  VbaArray *old=a;fail_after=stage;
  int error=vba_record_array_redim(&a,&item_type,1,1,bigger);fail_after=-1;
  if(error){CHECK(a==old && live==baseline && ((Item*)a->data)[0].n==42);continue;}
  succeeded=1;CHECK(a->bounds[0].count==3 && ((Item*)a->data)[0].n==42);break;
 }
 CHECK(succeeded);CHECK(!vba_record_array_erase(&a,&item_type) && a==NULL);CHECK(live==0);
 /* Fixed Erase must prepare nested fixed fields before replacing anything. */
 static const int32_t shape[]={1,2};
 static const VbaRecordField nested={R_ARRAY,V_I4,0,sizeof(void*),NULL,1,shape};
 static const VbaRecordType nested_type={sizeof(void*),1,&nested};
 CHECK(!vba_record_array_redim(&a,&nested_type,2,1,bounds));
 baseline=live;succeeded=0;
 for(int stage=0;stage<32;stage++){
  VbaArray *old=a;fail_after=stage;
  int error=vba_record_array_erase(&a,&nested_type);fail_after=-1;
  if(error){CHECK(a==old && live==baseline);continue;}
  succeeded=1;CHECK(a==old && (a->features&A_FIXED));break;
 }
 CHECK(succeeded && !vba_array_destroy(a) && live==0);return 0;
}
`)
}

func TestErrRuntimeOwnershipAndPreparationFailures(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+`
int main(void){
 VbaVariant number_value={.vt=V_I4},missing={.vt=V_ERROR},description={0};
 number_value.value.i4=-2147221400;missing.value.error=(int32_t)0x80020004;
 VbaString text={.data=(uint16_t[]){'a',0,'b'},.length=3};
 VbaString origin={.data=(uint16_t[]){'P','r','o','b','e'},.length=5};
 CHECK(!vba_variant_box_string(&description,&text));
 int baseline=live,succeeded=0;
 for(int stage=0;stage<24;stage++){
  fail_after=stage;int error=vba_err_raise(&number_value,&missing,&description,&missing,&missing,&origin);fail_after=-1;
  int32_t raised=0;CHECK(!vba_err_get(&raised,0));
  if(error!=number_value.value.i4){CHECK(error==7 || error==14);CHECK(raised==0 && live==baseline);continue;}
  succeeded=1;CHECK(raised==number_value.value.i4);break;
 }
 CHECK(succeeded);
 VbaString snapshot={0};CHECK(!vba_err_get(&snapshot,1));
 vba_err_clear();CHECK(vba_internal_slen(&snapshot)==3 && vba_internal_unit(&snapshot,1)==0);
 vba_internal_string_drop(snapshot);CHECK(live==baseline);
 vba_err_capture(11,&origin);int32_t code=0;
 CHECK(!vba_err_get(&code,0) && code==11);
 VbaString message={0};CHECK(!vba_err_get(&message,1));
 vba_err_clear();CHECK(vba_internal_slen(&message)==16);vba_internal_string_drop(message);
 vba_variant_clear(&description);CHECK(live==0);return 0;
}
`)
}
