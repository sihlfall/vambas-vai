package llvm_test

import (
	"strings"
	"testing"
)

func TestObjectArrayRuntimeFailureAtomicityAndTermination(t *testing.T) {
	nativeRuntimeC(t, strings.Split(abiTest, "int main(void)")[0]+`
static VbaArray *owner;
static int terminated,expect_count,expect_empty;
static int32_t terminate(void *error,VbaObject *object) {
 (void)error;(void)object;terminated++;
 CHECK(owner && (int)owner->bounds[0].count==expect_count);
 if(expect_empty)for(uint32_t i=0;i<owner->bounds[0].count;i++)CHECK(!((VbaObject**)owner->data)[i]);
 return 0;
}
static const VbaObjectType object_type={.size=sizeof(VbaObject),.terminate=terminate};
static const VbaObjectType other_type={.size=sizeof(VbaObject)};
static const VbaObjectArrayType array_type={&object_type,0},other_array_type={&other_type,0};
int main(void) {
 int32_t bounds[]={0,1};
 CHECK(!vba_object_array_redim(&owner,V_PRIVATE_OBJECT,&array_type,0,1,bounds));
 VbaObject *object=NULL;CHECK(!vba_class_new(&object,&object_type));
 CHECK(!vba_object_store(owner->data,object));vba_object_clear(&object);
 object=((VbaObject**)owner->data)[0];CHECK(object->references==1);
 VbaArray *copy=NULL;CHECK(!vba_array_copy(&copy,owner) && copy!=owner);
 CHECK(((VbaObject**)copy->data)[0]==object && object->references==2);
 vba_array_clear(&copy);CHECK(object->references==1);
 VbaVariant boxed={0},bad={0};
 CHECK(!vba_object_array_box(&boxed,owner,&array_type));
 CHECK(vba_variant_query(&boxed,1)==8201 && vba_variant_query(&boxed,5)==-1);
 CHECK(vba_object_array_unbox(&copy,&boxed,&other_array_type)==13 && !copy);
 vba_variant_clear(&boxed);CHECK(object->references==1);
 CHECK(!vba_class_new(&object,&other_type));CHECK(!vba_variant_box_object(&bad,object));vba_object_clear(&object);
 VbaElement element={0};int32_t index=0;
 CHECK(!vba_array_index(&element,&owner,V_PRIVATE_OBJECT,V_VARIANT,1,&index));
 CHECK(vba_variant_store(vba_element_address(&element,V_VARIANT),&bad)==13);
 CHECK(((VbaObject**)owner->data)[0]->type==&object_type);
 CHECK(vba_object_array_redim(&owner,V_PRIVATE_OBJECT,&array_type,1,1,bounds)==10);
 CHECK(vba_array_erase(&owner,V_PRIVATE_OBJECT)==10);
 vba_element_clear(&element);vba_variant_clear(&bad);
 int baseline=live;VbaArray *before=owner;int32_t larger[]={0,3};
 for(int stage=0;stage<2;stage++) {
  fail_after=stage;
  CHECK(vba_object_array_redim(&owner,V_PRIVATE_OBJECT,&array_type,1,1,larger)==7);
  fail_after=-1;CHECK(owner==before && live==baseline && !terminated);
 }
 CHECK(!vba_object_array_redim(&owner,V_PRIVATE_OBJECT,&array_type,1,1,larger));
 CHECK(!vba_class_new(&object,&object_type));CHECK(!vba_object_store((VbaObject**)owner->data+3,object));vba_object_clear(&object);
 expect_count=2;
 CHECK(!vba_object_array_redim(&owner,V_PRIVATE_OBJECT,&array_type,1,1,bounds));
 CHECK(terminated==1 && owner->bounds[0].count==2);
 owner->features|=A_FIXED;baseline=live;before=owner;expect_empty=1;
 CHECK(!vba_array_index(&element,&owner,V_PRIVATE_OBJECT,V_VARIANT,1,&index));
 void *cell=element.data;
 for(int stage=0;stage<1;stage++) {
  fail_after=stage;CHECK(vba_array_erase(&owner,V_PRIVATE_OBJECT)==7);fail_after=-1;
  CHECK(owner==before && live==baseline && ((VbaObject**)owner->data)[0] && terminated==1);
 }
 CHECK(!vba_array_erase(&owner,V_PRIVATE_OBJECT));CHECK(owner==before && terminated==2);
 CHECK(element.data==cell && owner->data==cell && owner->locks==1 && !*(VbaObject**)cell);
 vba_element_clear(&element);
 vba_array_clear(&owner);
 CHECK(!vba_object_array_box(&boxed,NULL,&array_type));
 CHECK(vba_object_array_unbox(&copy,&boxed,&other_array_type)==13);
 CHECK(!vba_object_array_unbox(&copy,&boxed,&array_type) && !copy);
 vba_variant_clear(&boxed);
 VbaArray *primitive=NULL;CHECK(!vba_array_redim(&primitive,V_I4,V_I4,0,1,bounds));
 VbaVariant malformed={.vt=V_ARRAY|V_PRIVATE_OBJECT,.value.pointer=primitive};
 VbaObjectArrayType generic={0};
 CHECK(vba_object_array_unbox(&copy,&malformed,&generic)==13 && !copy);
 vba_array_clear(&primitive);CHECK(!owner && !live);return 0;
}
`)
}
