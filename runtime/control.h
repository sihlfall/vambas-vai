#ifndef VBA_CONTROL_H
#define VBA_CONTROL_H
#include "arrays.h"
typedef struct {
    VbaArray *array;
    uint64_t index;
    int32_t initialized, owns;
} VbaEnumerator;
_Static_assert(sizeof(VbaEnumerator)==24, "private enumerator layout");
int32_t vba_condition(int16_t *out,const VbaVariant *value);
int32_t vba_variant_compare(VbaVariant *out,const VbaVariant *left,const VbaVariant *right,int32_t op,int32_t mode);
int32_t vba_for_validate(const VbaVariant *start,const VbaVariant *limit,const VbaVariant *step);
int32_t vba_for_test(int16_t *out,const VbaVariant *counter,const VbaVariant *limit,const VbaVariant *step);
int32_t vba_enum_start(VbaEnumerator *out,void *slot,int32_t declared,int32_t clone);
int32_t vba_enum_next(int16_t *out,VbaEnumerator *state,VbaVariant *counter);
void vba_enum_clear(VbaEnumerator *state);

/* Per-procedure resumption list. Addresses are opaque and never interpreted by C. */
typedef struct {void **entries;uint64_t count,capacity;} VbaGosubStack;
_Static_assert(sizeof(VbaGosubStack)==24,"private GoSub stack layout");
int32_t vba_gosub_push_ptr(VbaGosubStack *stack,void *address);
int32_t vba_gosub_pop_ptr(void **out,VbaGosubStack *stack);
void vba_gosub_clear(VbaGosubStack *stack);

#endif
