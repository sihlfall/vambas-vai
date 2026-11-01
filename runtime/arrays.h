#ifndef VBA_ARRAYS_H
#define VBA_ARRAYS_H
#include "variant.h"

typedef struct { uint32_t count; int32_t lower; } VbaArrayBound;
typedef struct VbaArray {
    uint16_t dimensions, features;
    uint32_t element_size, locks;
    void *data;
    VbaArrayBound bounds[1];
} VbaArray;
enum { V_ARRAY=0x2000, A_FIXED=0x10, A_RECORD=0x20, A_VARTYPE=0x80, A_BSTR=0x100, A_VARIANT=0x800 };
/* Private references are never Automation IDispatch/IUnknown pointers. */
enum { A_PRIVATE_OBJECT=0x2000 };
typedef struct VbaObjectType VbaObjectType;
typedef struct {const VbaObjectType *type;int32_t builtin;} VbaObjectArrayType;
_Static_assert(sizeof(VbaObjectArrayType)==16 && offsetof(VbaObjectArrayType,builtin)==8,"private object array constraint layout");
int32_t vba_object_array_redim(void *slot,int32_t declared,const VbaObjectArrayType *type,int32_t flags,int32_t rank,const int32_t *bounds);
int32_t vba_object_array_box(VbaVariant *out,VbaArray *source,const VbaObjectArrayType *type);
int32_t vba_object_array_unbox(VbaArray **out,const VbaVariant *source,const VbaObjectArrayType *type);
_Static_assert(offsetof(VbaArray,data)==16 && offsetof(VbaArray,bounds)==24, "64-bit SAFEARRAY offsets");
_Static_assert(sizeof(VbaArray)==32 && sizeof(VbaArrayBound)==8, "SAFEARRAY size");

/* A private owned lock token. String is a borrowed proxy for a BSTR*; reference
   is a borrowed Variant view for dynamically typed indexing. Neither owns data. */
typedef struct {
    VbaArray *array;
    void *data;
    VbaVariant reference;
    VbaString string;
} VbaElement;
_Static_assert(sizeof(VbaElement)==64, "element token layout");

/* A ParamArray packs parameter bindings, not ordinary Variant value copies.
   Referenced arguments retain their declared storage type through VT_BYREF. */
typedef struct {const VbaVariant *value;int32_t byref;} VbaParamArgument;
_Static_assert(sizeof(VbaParamArgument)==16 && offsetof(VbaParamArgument,byref)==8,"ParamArray argument layout");
int32_t vba_paramarray_pack(VbaArray **out,int32_t count,const VbaParamArgument *args);
int32_t vba_array_copy(VbaArray **out, VbaArray *source);
int32_t vba_array_destroy(VbaArray *array);
int32_t vba_array_check_unlocked(VbaArray *array);
void vba_array_clear(VbaArray **slot);
int32_t vba_array_store(VbaArray **slot, VbaArray *source);
int32_t vba_array_box(VbaVariant *out, VbaArray *source, int32_t tag);
int32_t vba_array_unbox(VbaArray **out, const VbaVariant *source, int32_t tag);
/* declared_tag=0 addresses Variant storage; otherwise slot is SAFEARRAY**.
   requested_tag=0 retains an existing Variant array's subtype (Variant if new).
   flags: 1 Preserve, 2 fixed-size initialization. Bounds are source-order pairs. */
int32_t vba_array_redim(void *slot, int32_t declared_tag, int32_t requested_tag,
    int32_t flags, int32_t rank, const int32_t *bounds);
int32_t vba_array_erase(void *slot, int32_t declared_tag);
int32_t vba_array_bound(int32_t *out, void *slot, int32_t declared_tag, int32_t dimension, int32_t upper);
int32_t vba_array_index(VbaElement *out, void *slot, int32_t declared_tag,
    int32_t expected_tag, int32_t rank, const int32_t *indices);
void *vba_element_address(VbaElement *element, int32_t tag);
void vba_element_clear(VbaElement *element);
/* Narrow lock ABI for compiler-inlined typed scalar element access. */
int32_t vba_array_lock(VbaArray *array);
void vba_array_unlock(VbaArray *array);
void *vba_string_ref(VbaString *slot);
#endif
