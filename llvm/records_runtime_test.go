package llvm_test

import (
	"strings"
	"testing"
)

func TestUDTRuntimeAtomicCopiesAndAliases(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+`
typedef struct { int32_t number; VbaString text; VbaVariant value; VbaArray *array; } State;
static const VbaRecordField state_fields[]={
 {R_SCALAR,V_I4,offsetof(State,number),sizeof(int32_t),NULL,0,NULL},
 {R_STRING,V_BSTR,offsetof(State,text),sizeof(VbaString),NULL,0,NULL},
 {R_VARIANT,V_VARIANT,offsetof(State,value),sizeof(VbaVariant),NULL,0,NULL},
 {R_ARRAY,V_I4,offsetof(State,array),sizeof(void*),NULL,0,NULL}
};
static const VbaRecordType state_type={sizeof(State),4,state_fields};
typedef struct { int16_t prefix; State child; } Parent;
static const VbaRecordField parent_fields[]={
 {R_SCALAR,V_I2,offsetof(Parent,prefix),sizeof(int16_t),NULL,0,NULL},
 {R_RECORD,0,offsetof(Parent,child),sizeof(State),&state_type,0,NULL}
};
static const VbaRecordType parent_type={sizeof(Parent),2,parent_fields};
int main(void) {
 Parent source={0},destination={0};
 CHECK(!vba_record_init(&source,&parent_type) && source.child.text.length==-1);
 CHECK(!vba_record_init(&destination,&parent_type));
 source.prefix=3;source.child.number=42;
 VbaString fresh={.data=(uint16_t[]){110,101,119},.length=3};
 VbaString old={.data=(uint16_t[]){111,108,100},.length=3};
 CHECK(!vba_string_store(&source.child.text,&fresh));
 CHECK(!vba_string_pin(&source.child.text));
 CHECK(!vba_variant_box_string(&source.child.value,&fresh));
 int32_t bounds[]={0,1};
 CHECK(!vba_array_redim(&source.child.array,V_I4,V_I4,0,1,bounds));
 destination.child.number=7;
 CHECK(!vba_string_store(&destination.child.text,&old));
 CHECK(!vba_string_pin(&destination.child.text));
 VbaVariant alias={.vt=V_BYREF|V_BSTR,.value.pointer=&destination.child.text.data};
 uint16_t *original=destination.child.text.data;
 int saw_success=0;
 for(int stage=0;stage<24;stage++) {
  int before=live;fail_after=stage;
  int error=vba_record_store(&destination,&source,&parent_type);
  fail_after=-1;
  if(!error) {saw_success=1;break;}
  CHECK(error==7 || error==14);
  CHECK(live==before && destination.child.number==7 && destination.child.text.data==original);
  CHECK(destination.child.text.length==-2 && !destination.child.array && destination.child.value.vt==V_EMPTY);
 }
 CHECK(saw_success && destination.prefix==3 && destination.child.number==42);
 CHECK(destination.child.text.length==-2 && destination.child.text.data!=source.child.text.data);
 CHECK(destination.child.value.value.pointer!=source.child.value.value.pointer);
 CHECK(destination.child.array!=source.child.array);
 VbaVariant read={0};CHECK(!vba_variant_load(&read,&alias));
 CHECK(read.vt==V_BSTR && ((uint16_t*)read.value.pointer)[0]==110);
 vba_variant_clear(&read);
 destination.child.array->locks++;
 CHECK(vba_record_store(&destination,&source,&parent_type)==10);
 CHECK(vba_record_store(&destination,&destination,&parent_type)==10);
 destination.child.array->locks--;
 vba_record_clear(&source,&parent_type);vba_record_clear(&destination,&parent_type);
 CHECK(live==0);
 /* Initialization failures release partially constructed fixed fields. */
 static const int32_t shape[]={1,3};
 static const VbaRecordField fixed_field={R_ARRAY,V_I4,0,sizeof(void*),NULL,1,shape};
 static const VbaRecordType fixed_type={sizeof(void*),1,&fixed_field};
 for(int stage=0;stage<2;stage++) {
  VbaArray *array=NULL;fail_after=stage;
  CHECK(vba_record_init(&array,&fixed_type)==7);fail_after=-1;
  CHECK(array==NULL && live==0);vba_record_clear(&array,&fixed_type);
 }
 return 0;
}
`)
}
