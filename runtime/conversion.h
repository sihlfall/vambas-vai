#ifndef VBA_CONVERSION_H
#define VBA_CONVERSION_H
#include "variant.h"

/* Library arguments are value snapshots. Outputs commit only on success.
   Explicit coercion can extract an Error code; ordinary Let coercion cannot. */
int32_t vba_convert_explicit(void *out,const VbaVariant *source,int32_t tag);
int32_t vba_cverr(VbaVariant *out,const VbaVariant *source);
int32_t vba_iserror(int16_t *out,const VbaVariant *source);
int32_t vba_cvdate(VbaVariant *out,const VbaVariant *source);
int32_t vba_cdec(VbaVariant *out,const VbaVariant *source);
#endif
