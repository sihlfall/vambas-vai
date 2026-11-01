#ifndef VBA_RUNTIME_INTERNAL_H
#define VBA_RUNTIME_INTERNAL_H

/* Runtime implementation interface. Generated code uses the public headers;
   these shared helpers have hidden visibility and are not part of its ABI. */
#include "variant.h"
#include "arrays.h"
#include "control.h"
#include "records.h"
#include "builtins.h"
#include "conversion.h"
#include "errors.h"
#include "collection.h"
#include "testing.h"

#if defined(__GNUC__)
#define VBA_INTERNAL __attribute__((visibility("hidden")))
#else
#define VBA_INTERNAL
#endif

VBA_INTERNAL uint32_t vba_internal_bstr_len(const uint16_t *s);
VBA_INTERNAL uint16_t *vba_internal_bstr_alloc(const uint16_t *s, uint32_t n);
VBA_INTERNAL int vba_internal_supported(uint16_t vt);
VBA_INTERNAL int vba_internal_borrow(VbaVariant *out, const VbaVariant *v);
VBA_INTERNAL int vba_internal_slen(const VbaString *s);
VBA_INTERNAL uint16_t vba_internal_unit(const VbaString *s, int i);
VBA_INTERNAL void vba_internal_string_drop(VbaString s);
VBA_INTERNAL void vba_internal_string_retain(VbaString s);
VBA_INTERNAL int vba_internal_convert(VbaVariant *out, const VbaVariant *source, uint16_t tag);
VBA_INTERNAL int vba_internal_object_operand(VbaVariant *owned,const VbaVariant **source);
VBA_INTERNAL const VbaRecordType *vba_internal_array_record_type(VbaArray *a);
VBA_INTERNAL uint32_t vba_internal_array_element_size(uint16_t tag);
VBA_INTERNAL uint16_t vba_internal_array_tag(VbaArray *a);
VBA_INTERNAL size_t vba_internal_array_count(VbaArray *a);
VBA_INTERNAL int vba_internal_array_create_typed(VbaArray **out,uint16_t tag,int rank,const VbaArrayBound *bounds,const VbaRecordType *record);
VBA_INTERNAL int vba_internal_array_create(VbaArray **out,uint16_t tag,int rank,const VbaArrayBound *bounds);
VBA_INTERNAL int vba_internal_array_copy_elements(VbaArray *out,VbaArray *source,size_t count);
VBA_INTERNAL int vba_internal_byte_array_bstr(VbaVariant *out,VbaArray *source);
VBA_INTERNAL int vba_internal_byte_array_from_bstr(VbaArray **out,const uint16_t *text);
VBA_INTERNAL int vba_internal_array_location(void *slot,int declared,VbaArray ***location,VbaVariant **variant,int *constraint);
VBA_INTERNAL int vba_internal_compare_bstr(const uint16_t *a,const uint16_t *b);
VBA_INTERNAL int vba_internal_date_valid(double n);
VBA_INTERNAL int vba_internal_numeric_fixed_convert(VbaVariant *out,const VbaVariant *v,uint16_t tag);
VBA_INTERNAL int vba_internal_numeric_binary(VbaVariant *out,const VbaVariant *a,const VbaVariant *b,int op);
VBA_INTERNAL int vba_internal_numeric_compare(const VbaVariant *a,const VbaVariant *b,int *comparison);
VBA_INTERNAL int vba_internal_record_replaceable(const void *record,const VbaRecordType *type);
VBA_INTERNAL int vba_internal_array_lock(VbaArray *array);
VBA_INTERNAL void vba_internal_array_unlock(VbaArray *array);
VBA_INTERNAL int vba_internal_object_array_check(const VbaVariant *source,const VbaObjectArrayType *type);

/* Borrowed description; valid until the next mutation of intrinsic Err state. */
VBA_INTERNAL VbaString vba_internal_err_description(int32_t code);

/* Saving retains owned strings; restoring consumes the saved state. The current
   Err instance remains private to errors.c. */
typedef struct {int32_t number,help_context;VbaString source,description,help_file;} VbaErrState;
VBA_INTERNAL VbaErrState vba_internal_err_save(void);
VBA_INTERNAL void vba_internal_err_restore(VbaErrState saved);

#endif
