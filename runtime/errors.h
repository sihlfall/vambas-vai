#ifndef VBA_ERRORS_H
#define VBA_ERRORS_H
#include "variant.h"
/* Per-runtime Err state, independent of activation-local On Error policy.
   Source literals passed to capture are compiler-owned immutable values. */
void vba_err_clear(void);
void vba_err_capture(int32_t code,const VbaString *source);
void vba_err_raise_failure(int32_t code,const VbaString *source);
int32_t vba_err_raise(const VbaVariant *number_value,const VbaVariant *source,const VbaVariant *description,const VbaVariant *help_file,const VbaVariant *help_context,const VbaString *origin);
int32_t vba_err_get(void *out,int32_t property);
int32_t vba_err_set(int32_t property,const VbaVariant *value);
#endif
