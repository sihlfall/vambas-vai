package llvm_test

import "testing"

func TestBuiltinRuntimeAllocationAndABI(t *testing.T) {
	nativeRuntimeC(t, builtinAllocationTest)
}

const builtinAllocationTest = `
#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>
static int live,fail_after=-1;
void *vba_test_malloc(size_t n) {
 if(fail_after>=0 && fail_after--==0) {fail_after=-1;return NULL;}
 void *p=malloc(n);if(p) live++;return p;
}
void vba_test_free(void *p) {if(p) live--;free(p);}
#include "runtime_internal.h"
#include <string.h>
#include <math.h>
#include <limits.h>
#define CHECK(c) do {if(!(c)){fprintf(stderr,"line %d: %s\n",__LINE__,#c);return 1;}}while(0)
int main(void) {
 VbaVariant text={.vt=V_BSTR},delimiter={.vt=V_BSTR},length={.vt=V_I4},limit={.vt=V_I4},mode={.vt=V_I4};
 const uint16_t units[]={'a',0,'b',0,'c'},sep[]={0};
 text.value.pointer=vba_internal_bstr_alloc(units,5);delimiter.value.pointer=vba_internal_bstr_alloc(sep,1);
 CHECK(text.value.pointer && delimiter.value.pointer);
 length.value.i4=3;limit.value.i4=-1;
 int baseline=live,success[5]={0};
 for(int op=0;op<5;op++) {
  for(int at=0;at<16;at++) {
   fail_after=at;
   VbaVariant v={0};VbaString s={0};VbaArray *array=NULL;
   int error=op==0 ? vba_left(&v,&text,&length,0) :
       op==1 ? vba_left(&s,&text,&length,1) :
       op==2 ? vba_string_repeat(&s,&length,&text,1) :
       op==3 ? vba_split(&array,&text,&delimiter,&limit,&mode) :
       vba_iif(&v,&length,&text,&delimiter);
   fail_after=-1;
   if(error) {
    CHECK(error==7 || error==14);
    CHECK(v.vt==0 && s.length==0 && s.data==NULL && array==NULL);
   } else {
    success[op]=1;
    if(op==0 || op==4) {CHECK(v.vt==V_BSTR);vba_variant_clear(&v);}
    else if(op==3) {
     CHECK(array->dimensions==1 && array->bounds[0].count==3 && array->bounds[0].lower==0);
     uint16_t **p=array->data;
     CHECK(vba_internal_bstr_len(p[0])==1 && p[0][0]=='a' && p[1][0]=='b' && p[2][0]=='c');
     CHECK(!vba_array_destroy(array));
    } else {CHECK(s.length!=0);vba_internal_string_drop(s);}
   }
   CHECK(live==baseline && vba_internal_bstr_len(text.value.pointer)==5);
  }
  CHECK(success[op]);
 }
 VbaVariant byte_a={.vt=V_UI1},byte_b={.vt=V_UI1},byte_out={0};
 byte_a.value.byte=255;byte_b.value.byte=4;
 CHECK(!vba_variant_binary(&byte_out,&byte_a,&byte_b,6) && byte_out.vt==V_UI1 && byte_out.value.byte==63);
 CHECK(!vba_variant_binary(&byte_out,&byte_a,&byte_b,7) && byte_out.vt==V_UI1 && byte_out.value.byte==3);
 VbaVariant error_value={.vt=V_ERROR},null_operand={.vt=V_NULL};
 CHECK(vba_variant_binary(&byte_out,&error_value,&null_operand,6)==13);
 CHECK(vba_variant_binary(&byte_out,&null_operand,&error_value,7)==13);
 int16_t unit=0;
 CHECK(!vba_ascw(&unit,&text) && unit=='a');
 VbaVariant null_value={.vt=V_NULL},out={0},empty={0};
 CHECK(vba_ascw(&unit,&empty)==5 && unit=='a');
 CHECK(vba_ascw(&unit,&null_value)==94 && unit=='a');
 CHECK(!vba_strcomp(&out,&null_value,&text,&mode) && out.vt==V_NULL);
 vba_variant_clear(&out);
 limit.value.i4=0;VbaArray *array=NULL;
 CHECK(!vba_split(&array,&text,&delimiter,&limit,&mode) && array->bounds[0].count==0);
 CHECK(!vba_array_destroy(array));
 vba_variant_clear(&text);vba_variant_clear(&delimiter);CHECK(live==0);
 return 0;
}
`
