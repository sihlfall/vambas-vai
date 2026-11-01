package llvm_test

import "testing"

func TestClassRuntimeOwnershipAndConstructionFailure(t *testing.T) {
	nativeRuntimeC(t, classRuntimeProbe)
}

const classRuntimeProbe = `
#include <stdlib.h>
#include <stdio.h>
static int live, fail_after=-1;
void *vba_test_malloc(size_t n){
 if(fail_after>=0 && fail_after--==0){fail_after=-1;return NULL;}
 void *p=malloc(n);if(p)live++;return p;
}
void vba_test_free(void *p){if(p)live--;free(p);}
#include "runtime_internal.h"
#include <string.h>
#include <math.h>
#include <limits.h>
#define CHECK(c) do {if(!(c)){fprintf(stderr,"line %d: %s (live=%d)\n",__LINE__,#c,live);return 1;}}while(0)
typedef struct {VbaString text;VbaVariant value;VbaArray *first,*second;VbaObject *child;} Fields;
typedef struct {VbaObject object;Fields fields;} Instance;
static const int32_t bounds[]={1,3};
static const VbaRecordField fields[]={
 {R_STRING,V_BSTR,offsetof(Fields,text),sizeof(VbaString),NULL,0,NULL},
 {R_VARIANT,V_VARIANT,offsetof(Fields,value),sizeof(VbaVariant),NULL,0,NULL},
 {R_ARRAY,V_I4,offsetof(Fields,first),sizeof(VbaArray*),NULL,1,bounds},
 {R_ARRAY,V_BSTR,offsetof(Fields,second),sizeof(VbaArray*),NULL,1,bounds},
 {R_OBJECT,V_PRIVATE_OBJECT,offsetof(Fields,child),sizeof(VbaObject*),NULL,0,NULL}
};
static const VbaRecordType shape={sizeof(Fields),5,fields};
static const VbaObjectType type={.size=sizeof(Instance),.fields=&shape};
static const VbaObjectType other_type={.size=sizeof(Instance),.fields=&shape};
static int terminated;
static VbaObject *saved;
static int32_t initialize_failure(void *error,VbaObject *object){
 (void)error;Instance *instance=(Instance*)object;
 int status=vba_object_new(&instance->fields.child,O_COLLECTION);if(status)return status;
 VbaString text={.data=(uint16_t*)u"constructed",.length=11};
 status=vba_variant_box_string(&instance->fields.value,&text);return status?status:37;
}
static int32_t terminate_count(void *error,VbaObject *object){(void)error;(void)object;terminated++;return 0;}
static int32_t terminate_revive(void *error,VbaObject *object){(void)error;terminated++;return vba_object_store(&saved,object);}
static const VbaObjectType failing_type={.size=sizeof(Instance),.fields=&shape,.initialize=initialize_failure,.terminate=terminate_count};
static const VbaObjectType reviving_type={.size=sizeof(Instance),.fields=&shape,.terminate=terminate_revive};
int main(void){
 CHECK(sizeof(VbaObject)==offsetof(Instance,fields));
 VbaObject *object=NULL,*alias=NULL;
 CHECK(vba_class_check(NULL,&type,0)==91);
 CHECK(vba_class_check(NULL,&type,1)==0);
 int completed=0;
 for(int i=0;i<32;i++){
  fail_after=i;int e=vba_class_new(&object,&type);fail_after=-1;
  if(!e){vba_object_clear(&object);CHECK(!object&&live==0);completed=1;break;}
  CHECK(e==7&&!object&&live==0);
 }
 CHECK(completed);
 completed=0;
 for(int i=0;i<32;i++){
  fail_after=i;int e=vba_class_new(&object,&failing_type);fail_after=-1;
  CHECK(!object&&live==0&&(e==7||e==14||e==37));
  if(e==37){completed=1;break;}
 }
 CHECK(completed&&terminated>0);terminated=0;
 CHECK(!vba_class_new(&object,&reviving_type));
 vba_object_clear(&object);CHECK(!object&&saved&&saved->references==1&&terminated==1);
 vba_object_clear(&saved);CHECK(!saved&&terminated==1&&live==0);
 CHECK(!vba_class_new(&object,&type));
 CHECK(!vba_class_check(object,&type,0));
 CHECK(vba_class_check(object,&other_type,1)==13);
 CHECK(vba_object_check(object,O_COLLECTION)==13);
 Instance *instance=(Instance*)object;
 CHECK(instance->fields.text.length==-1);
 VbaString source={.data=(uint16_t*)u"managed",.length=7};
 CHECK(!vba_string_store(&instance->fields.text,&source));
 CHECK(!vba_variant_box_string(&instance->fields.value,&source));
 CHECK(!vba_object_load(&alias,&object,0)&&alias==object&&object->references==2);
 CHECK(!vba_object_store(&object,object)&&object->references==2);
 vba_object_clear(&object);CHECK(!object&&alias->references==1);
 vba_object_clear(&alias);CHECK(!alias&&live==0);
 CHECK(!vba_object_new(&object,O_COLLECTION));
 CHECK(vba_class_check(object,&type,0)==13);
 vba_object_clear(&object);CHECK(live==0);
 return 0;
}
`
