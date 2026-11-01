#ifndef VBA_TESTING_H
#define VBA_TESTING_H
#include "objects.h"
/* Private virtual-library handles. They are never exposed as Automation COM
   pointers. The test protocol and registration table are framework-neutral. */
typedef VbaObjectArg VbaTestArg;
typedef struct {int32_t number,procedure,statement,offset;} VbaTestError;
typedef int32_t (*VbaTestProc)(VbaTestError *);
typedef struct {const char *id;VbaTestProc invoke;int32_t ignored;} VbaTestEntry;
typedef struct {const char *id;VbaTestProc initialize,cleanup,before,after;int32_t count;const VbaTestEntry *tests;} VbaTestSuite;
_Static_assert(sizeof(VbaTestArg)==24 && offsetof(VbaTestArg,value)==16,"test argument layout");
_Static_assert(sizeof(VbaTestEntry)==24 && offsetof(VbaTestEntry,ignored)==16,"test entry layout");
_Static_assert(sizeof(VbaTestSuite)==56 && offsetof(VbaTestSuite,tests)==48,"test suite layout");
_Static_assert(sizeof(VbaTestError)==16,"test error layout");
int32_t vba_test_assert(VbaObject *receiver,int32_t method,int32_t count,const VbaTestArg *args,int32_t procedure,int32_t statement,int32_t offset);
int32_t vba_test_main(int32_t argc,char **argv,int32_t count,const VbaTestSuite *suites);
#endif
