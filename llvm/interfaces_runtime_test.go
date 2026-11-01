package llvm_test

import "testing"

func TestInterfaceRuntimeDispatchAndOwnership(t *testing.T) {
	nativeRuntimeC(t, interfaceRuntimeProbe)
}

const interfaceRuntimeProbe = `
#include <stdlib.h>
#include <stdio.h>
static int live,fail_next;
void *vba_test_malloc(size_t n){if(fail_next){fail_next=0;return NULL;}void *p=malloc(n);if(p)live++;return p;}
void vba_test_free(void *p){if(p)live--;free(p);}
#include "runtime_internal.h"
#define CHECK(c) do {if(!(c)){fprintf(stderr,"line %d: %s (live=%d)\n",__LINE__,#c,live);return 1;}}while(0)
static const VbaObjectType first={.size=sizeof(VbaObject)},second={.size=sizeof(VbaObject)},empty={.size=sizeof(VbaObject)};
static int32_t one(void *error,int32_t *out,VbaObject *object,int32_t value){(void)error;(void)object;*out=value+40;return 0;}
static int32_t two(void *error,int32_t *out,VbaObject *object,int32_t value){(void)error;(void)object;*out=value+100;return 0;}
static const VbaInterfaceEntry first_methods[]={{7,(VbaInterfaceMethod)one}},second_methods[]={{8,(VbaInterfaceMethod)two}};
static const VbaInterface interfaces[]={{&first,1,first_methods},{&second,1,second_methods},{&empty,0,NULL}};
static int terminated;
static int32_t terminate(void *error,VbaObject *object){(void)error;(void)object;terminated++;return 0;}
static const VbaObjectType concrete={.size=sizeof(VbaObject),.terminate=terminate,.interface_count=3,.interfaces=interfaces};
int main(void){
 VbaObject *object=NULL,*fallback=NULL;CHECK(!vba_class_new(&object,&concrete));CHECK(!vba_class_new(&fallback,&first));
 CHECK(!vba_class_check(object,&first,0));CHECK(!vba_class_check(object,&second,0));CHECK(!vba_class_check(object,&empty,0));
 CHECK(vba_class_check_exact(object,&first)==13);CHECK(!vba_class_check_exact(object,&concrete));CHECK(vba_class_check_exact(NULL,&first)==91);
 CHECK(vba_class_check(fallback,&second,1)==13);CHECK(vba_class_check(NULL,&first,0)==91);CHECK(!vba_class_check(NULL,&first,1));
 VbaInterfaceMethod method=(VbaInterfaceMethod)two;
 CHECK(vba_interface_resolve(&method,NULL,&first,7,(VbaInterfaceMethod)two)==91);CHECK(method==(VbaInterfaceMethod)two);
 CHECK(vba_interface_resolve(&method,object,&first,999,(VbaInterfaceMethod)two)==13);CHECK(method==(VbaInterfaceMethod)two);
 CHECK(vba_interface_resolve(&method,fallback,&second,8,(VbaInterfaceMethod)one)==13);CHECK(method==(VbaInterfaceMethod)two);
 fail_next=1;CHECK(!vba_interface_resolve(&method,object,&first,7,(VbaInterfaceMethod)two));CHECK(fail_next==1);fail_next=0;
 int32_t result=0;CHECK(!((int32_t (*)(void*,int32_t*,VbaObject*,int32_t))method)(NULL,&result,object,2));CHECK(result==42);
 CHECK(!vba_interface_resolve(&method,object,&second,8,(VbaInterfaceMethod)one));CHECK(method==(VbaInterfaceMethod)two);
 CHECK(!vba_interface_resolve(&method,fallback,&first,7,(VbaInterfaceMethod)two));CHECK(method==(VbaInterfaceMethod)two);
 CHECK(object->references==1);
 VbaVariant boxed={0};CHECK(!vba_native_box(&boxed,&object,V_PRIVATE_OBJECT));CHECK(object->references==2);
 VbaObject *view=NULL;CHECK(!vba_native_from(&view,&boxed,V_PRIVATE_OBJECT,&first,0));CHECK(view==object && object->references==3);
 vba_object_clear(&object);CHECK(!terminated);vba_variant_clear(&boxed);CHECK(!terminated);vba_object_clear(&view);CHECK(terminated==1);
 vba_object_clear(&fallback);CHECK(!live);return 0;
}
`
