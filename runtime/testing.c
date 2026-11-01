#include "testing.h"
#include "runtime_internal.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

/* Independently implemented compatibility services, based on the documented
   source-visible assertion API and this repository's test design. */
#include <time.h>
static int64_t test_time_ns(void){struct timespec t;timespec_get(&t,TIME_UTC);return (int64_t)t.tv_sec*1000000000+t.tv_nsec;}

typedef struct {int failures,inconclusive,assertions,errors;const char *phase;} VbaTestContext;
static VbaTestContext *test_context;
static void test_json_text(const VbaString *s){
 putchar('"');for(int i=0;i<vba_internal_slen(s);i++){
  uint32_t c=vba_internal_unit(s,i);
  if(c=='"'||c=='\\'){putchar('\\');putchar((int)c);}
  else if(c<32 || (c>=0xd800&&c<=0xdfff)){printf("\\u%04x",(unsigned)c);}
  else if(c<128)putchar((int)c);
  else if(c<2048){putchar(0xc0|(c>>6));putchar(0x80|(c&63));}
  else {putchar(0xe0|(c>>12));putchar(0x80|((c>>6)&63));putchar(0x80|(c&63));}
 }putchar('"');
}
static const char *test_method_names[]={"","AreEqual","AreNotEqual","AreSame","AreNotSame","IsTrue","IsFalse","IsNothing","IsNotNothing","SequenceEquals","NotSequenceEquals","Fail","Inconclusive","Succeed"};
static void test_assert_event(int method,int inconclusive,const VbaString *message,int procedure,int statement,int offset){
 if(inconclusive)test_context->inconclusive++;else test_context->failures++;
 printf("{\"event\":\"assertion\",\"phase\":\"%s\",\"method\":\"%s\",\"inconclusive\":%s,\"procedure\":%d,\"statement\":%d,\"offset\":%d,\"message\":",test_context->phase,test_method_names[method],inconclusive?"true":"false",procedure,statement,offset);
 test_json_text(message);puts("}");fflush(stdout);
}
static int test_equal(const VbaVariant *left,const VbaVariant *right,int permissive,int depth,int *equal);
static int test_sequence(VbaArray *a,VbaArray *b,int permissive,int depth,int *equal){
 *equal=0;if(!a||!b){*equal=a==b;return 0;}
 if(depth>64)return 13;
 if(a->dimensions!=b->dimensions)return 0;
 for(int i=0;i<a->dimensions;i++)if(a->bounds[i].lower!=b->bounds[i].lower||a->bounds[i].count!=b->bounds[i].count)return 0;
 uint16_t at=vba_internal_array_tag(a),bt=vba_internal_array_tag(b);
 if(!permissive&&at!=bt)return 0;
 for(size_t i=0;i<vba_internal_array_count(a);i++){
  VbaVariant av={0},bv={0};int e=0;
  if(at==V_VARIANT)av=((VbaVariant*)a->data)[i];else {av.vt=at;memcpy(&av.value,(char*)a->data+i*a->element_size,a->element_size);}
  if(bt==V_VARIANT)bv=((VbaVariant*)b->data)[i];else {bv.vt=bt;memcpy(&bv.value,(char*)b->data+i*b->element_size,b->element_size);}
  int eq=0;e=test_equal(&av,&bv,permissive,depth+1,&eq);if(e||!eq)return e;
 }*equal=1;return 0;
}
static int test_equal(const VbaVariant *left,const VbaVariant *right,int permissive,int depth,int *equal){
 VbaVariant a={0},b={0};*equal=0;int e=vba_internal_borrow(&a,left);if(e)return e;e=vba_internal_borrow(&b,right);if(e)return e;
 if(!permissive&&a.vt!=b.vt)return 0;
 if((a.vt|b.vt)&V_ARRAY){if(!(a.vt&V_ARRAY)||!(b.vt&V_ARRAY))return 0;return test_sequence(a.value.pointer,b.value.pointer,permissive,depth,equal);}
 if(a.vt==V_NULL||b.vt==V_NULL){*equal=a.vt==b.vt;return 0;}
 if(a.vt==V_EMPTY&&b.vt==V_EMPTY){*equal=1;return 0;}
 if(a.vt==V_ERROR||b.vt==V_ERROR){*equal=a.vt==b.vt&&a.value.error==b.value.error;return 0;}
 VbaVariant result={0};int mode=permissive&&((a.vt==V_BSTR)!=(b.vt==V_BSTR))?1:0;
 e=vba_variant_compare(&result,&a,&b,1,mode);if(e)return e;*equal=result.vt==V_BOOL&&result.value.boolean!=0;return 0;
}
static int test_name_equal(const char *a,const char *b){while(*a&&*b){unsigned char x=(unsigned char)*a++,y=(unsigned char)*b++;if(x>='A'&&x<='Z')x+=32;if(y>='A'&&y<='Z')y+=32;if(x!=y)return 0;}return *a==*b;}
int32_t vba_test_assert(VbaObject *o,int32_t method,int32_t count,const VbaTestArg *args,int32_t procedure,int32_t statement,int32_t offset){
 if(!o)return 91;if(o->kind!=1&&o->kind!=2)return 438;
 if(!test_context)return 5;if(method<1||method>13)return 438;
 int required=method<=4||method==9||method==10?2:method<=8?1:0;
 const VbaTestArg *bound[3]={0};int positional=0;
 for(int i=0;i<count;i++){
  int slot=-1;
  if(!*args[i].name){slot=positional++;}
  else if(test_name_equal(args[i].name,"Message"))slot=required;
  else if(required==2&&test_name_equal(args[i].name,"Expected"))slot=0;
  else if(required==2&&test_name_equal(args[i].name,"Actual"))slot=1;
  else if(required==1&&test_name_equal(args[i].name,method==5||method==6?"Condition":"Value"))slot=0;
  else return 448;
  if(slot<0||slot>required||bound[slot])return 450;bound[slot]=args+i;
 }
 for(int i=0;i<required;i++)if(!bound[i])return 450;
 VbaString message={0};int e=0;
 if(bound[required]){if(bound[required]->kind)return 13;e=vba_variant_to(&message,bound[required]->value,V_BSTR);if(e)return e;}
 int truth=0,inconclusive=method==12;
 if(method==11||method==12)truth=0;
 else if(method==13)truth=1;
 else if(method==3||method==4||method==7||method==8){
  for(int i=0;i<required;i++)if(bound[i]->kind!=1){e=13;goto done;}
  VbaObject *a=*(VbaObject*const*)bound[0]->value;
  truth=required==2?a==*(VbaObject*const*)bound[1]->value:a==NULL;
  if(method==4||method==8)truth=!truth;
 }else if(method==5||method==6){
  if(bound[0]->kind){e=13;goto done;}int16_t value=0;e=vba_variant_to(&value,bound[0]->value,V_BOOL);if(e)goto done;truth=method==5?value!=0:value==0;
 }else{
  if(bound[0]->kind||bound[1]->kind){inconclusive=1;truth=0;}
  else {
   VbaVariant a={0},b={0};e=vba_internal_borrow(&a,bound[0]->value);if(!e)e=vba_internal_borrow(&b,bound[1]->value);if(e)goto done;
   int arrays=(a.vt&V_ARRAY)&&(b.vt&V_ARRAY);
   // AreEqual is a scalar comparison; sequence comparison is explicitly selected.
   if((method==9||method==10)&&!arrays){inconclusive=1;truth=0;}
   else if((method==1||method==2)&&((a.vt|b.vt)&V_ARRAY)){inconclusive=1;truth=0;}
   else {e=test_equal(&a,&b,o->kind==2,0,&truth);if(e){e=0;inconclusive=1;truth=0;}else if(method==2||method==10)truth=!truth;}
  }
 }
 test_context->assertions++;
 if(!truth)test_assert_event(method,inconclusive,&message,procedure,statement,offset);
 done:vba_internal_string_drop(message);return e;
}
static int test_invoke(VbaTestProc invoke,const char *phase){
 if(!invoke)return 0;test_context->phase=phase;VbaTestError error={0};int status=invoke(&error);
 if(status){test_context->errors++;printf("{\"event\":\"error\",\"phase\":\"%s\",\"number\":%d,\"procedure\":%d,\"statement\":%d,\"offset\":%d,\"message\":",phase,status,error.procedure,error.statement,error.offset);VbaString text=vba_internal_err_description(status);test_json_text(&text);puts("}");fflush(stdout);}
 return status||test_context->failures||test_context->inconclusive;
}
int32_t vba_test_main(int32_t argc,char **argv,int32_t count,const VbaTestSuite *suites){
 if(argc<2)return 2;char *end=NULL;long index=strtol(argv[1],&end,10);if(!end||*end||index<0||index>=count)return 2;
 const VbaTestSuite *suite=suites+index;VbaTestContext module={0};test_context=&module;vba_err_clear();
 int blocked=test_invoke(suite->initialize,"ModuleInitialize"),failed=blocked;
 for(int i=0;i<suite->count;i++){
  int selected=argc==2;for(int a=2;a<argc;a++)if(strcmp(argv[a],suite->tests[i].id)==0)selected=1;
  if(!selected)continue;
  printf("{\"event\":\"test_start\",\"index\":%d}\n",i);fflush(stdout);
  int64_t start=test_time_ns();VbaTestContext test={0};test_context=&test;vba_err_clear();
  const char *status="passed";
  if(suite->tests[i].ignored)status="skipped";
  else if(blocked)status="blocked";
  else {
   int setup=test_invoke(suite->before,"TestInitialize");
   if(!setup)test_invoke(suite->tests[i].invoke,"TestMethod");
   test_invoke(suite->after,"TestCleanup");
   status=test.errors?"error":test.failures?"failed":test.inconclusive?"inconclusive":"passed";
  }
  if(strcmp(status,"passed")&&strcmp(status,"skipped"))failed=1;
  printf("{\"event\":\"test_end\",\"index\":%d,\"status\":\"%s\",\"assertions\":%d,\"durationNs\":%lld}\n",i,status,test.assertions,(long long)(test_time_ns()-start));fflush(stdout);
 }
 test_context=&module;vba_err_clear();test_invoke(suite->cleanup,"ModuleCleanup");if(module.errors||module.failures||module.inconclusive)failed=1;
 test_context=NULL;vba_err_clear();puts("{\"event\":\"suite_end\"}");fflush(stdout);return failed?1:0;
}
