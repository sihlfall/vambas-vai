#ifndef VBA_BUILTINS_H
#define VBA_BUILTINS_H
#include "arrays.h"

/* Fresh output required; failure leaves output untouched. Library arguments
   are value snapshots, never writable aliases. dollar selects String output;
   otherwise Left/String produce an Automation Variant (String or Null). */
int32_t vba_ascw(int16_t *out,const VbaVariant *source);
int32_t vba_strcomp(VbaVariant *out,const VbaVariant *left,const VbaVariant *right,const VbaVariant *compare);
int32_t vba_instr(VbaVariant *out,const VbaVariant *start,const VbaVariant *text,const VbaVariant *needle,const VbaVariant *compare);
int32_t vba_left(void *out,const VbaVariant *source,const VbaVariant *length,int32_t dollar);
int32_t vba_string_repeat(void *out,const VbaVariant *number_value,const VbaVariant *character,int32_t dollar);
int32_t vba_split(VbaArray **out,const VbaVariant *expression,const VbaVariant *delimiter,const VbaVariant *limit,const VbaVariant *compare);
int32_t vba_iif(VbaVariant *out,const VbaVariant *expression,const VbaVariant *truepart,const VbaVariant *falsepart);
#endif
