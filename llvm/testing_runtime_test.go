package llvm_test

import (
	"bytes"
	"testing"
)

// Exercise private handle ownership and assertion/runtime failure separation
// with allocation tracking and failure injection at both native opt levels.
func TestTestingRuntimeOwnership(t *testing.T) {
	for _, output := range nativeRuntimeC(t, testingRuntimeProbe) {
		if bytes.Count(output, []byte(`"event":"assertion"`)) != 1 || !bytes.Contains(output, []byte(`"assertions":2`)) {
			t.Fatalf("assertion failure/counter protocol changed: %s", output)
		}
	}
}

const testingRuntimeProbe = `
#include <stdlib.h>
#include <stdio.h>
static int live,fail_next;
void *vba_test_malloc(size_t n){if(fail_next){fail_next=0;return NULL;}void *p=malloc(n);if(p)live++;return p;}
void vba_test_free(void *p){if(p)live--;free(p);}
#include "runtime_internal.h"
#include <string.h>
#include <math.h>
#include <limits.h>
#define CHECK(c) do {if(!(c)){fprintf(stderr,"line %d: %s\n",__LINE__,#c);return 1;}}while(0)
static VbaObject *assertion;
static int completed;
static int32_t exercise_assertions(VbaTestError *error){
 (void)error;
 VbaVariant one={.vt=V_I4,.value.i4=1},two={.vt=V_I4,.value.i4=2};
 VbaTestArg args[]={{"",0,&one},{"",0,&two}};
 CHECK(!vba_test_assert(assertion,1,2,args,1,2,3));
 CHECK(vba_test_assert(NULL,1,2,args,1,2,3)==91);
 VbaObject *nothing=NULL;VbaTestArg object_arg={"",1,&nothing};
 CHECK(!vba_test_assert(assertion,7,1,&object_arg,1,2,3));
 CHECK(vba_test_assert(assertion,5,1,&object_arg,1,2,3)==13);
 completed=1;return 0;
}
int main(void){
 VbaObject *a=NULL,*copy=NULL;
 fail_next=1;CHECK(vba_object_new(&a,1)==7&&!a&&live==0);
 CHECK(!vba_object_load(&copy,&a,1)&&a==copy&&a->references==2&&live==1);
 vba_object_clear(&copy);CHECK(a->references==1);
 CHECK(!vba_object_store(&a,a)&&a->references==1);
 CHECK(!vba_object_load(&copy,&a,0));vba_object_clear(&a);CHECK(copy&&copy->references==1);
 vba_object_clear(&copy);CHECK(live==0);
 VbaString unknown={.data=(uint16_t*)u"No.Such.Class",.length=13};CHECK(vba_create_object(&a,&unknown)==429&&!a);
 CHECK(!vba_object_new(&a,1));
 VbaVariant one={.vt=V_I4,.value.i4=1},two={.vt=V_I4,.value.i4=2};
 VbaTestArg args[]={{"",0,&one},{"",0,&two}};
 CHECK(vba_test_assert(a,1,2,args,1,2,3)==5); // No active context cannot silently pass.
 const VbaTestEntry entry={.id="probe",.invoke=exercise_assertions};
 const VbaTestSuite suite={.id="suite",.count=1,.tests=&entry};
 char *argv[]={"probe","0"};assertion=a;
 CHECK(vba_test_main(2,argv,1,&suite)==1 && completed);
 CHECK(vba_test_assert(a,1,2,args,1,2,3)==5);
 vba_object_clear(&a);vba_err_clear();CHECK(live==0);return 0;
}
`
