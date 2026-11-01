#ifndef VBA_VARIANT_H
#define VBA_VARIANT_H
#include <stdint.h>
#include <stddef.h>

/* Automation VARIANT layout, with explicitly private native extensions below.
   Includes the BRECORD arm that makes Win64 size 24.
   DECIMAL overlays the entire union; reserved fields are always initialized. */
typedef struct VbaVariant VbaVariant;
struct VbaVariant {
    union {
        struct {
            uint16_t vt, reserved1, reserved2, reserved3;
            union {
                int16_t i2, boolean;
                int32_t i4, error;
                int64_t i8; /* Currency: scaled signed integer, never a Double. */
                uint8_t byte;
                float r4;
                double r8;
                void *pointer;
                struct { void *record, *info; } record;
            } value;
        };
        struct { uint16_t reserved; uint8_t scale, sign; uint32_t hi; uint64_t lo; } decimal;
    };
};
_Static_assert(offsetof(VbaVariant, vt) == 0, "VARIANT vt offset");
_Static_assert(offsetof(VbaVariant, value) == 8, "VARIANT payload offset");
_Static_assert(sizeof(VbaVariant) == (sizeof(void*) == 8 ? 24 : 16), "VARIANT size");

/* Private expression representation. length -2 means that data is the owning
   BSTR slot exposed to Automation ByRef; all accesses must refresh through it. */
typedef struct { uint16_t *data; void *owner; int32_t length; uint16_t unit; } VbaString;
_Static_assert(sizeof(void*) != 8 || sizeof(VbaString) == 24, "private String layout");

enum { V_EMPTY=0, V_NULL=1, V_I2=2, V_I4=3, V_R4=4, V_R8=5, V_CY=6, V_DATE=7,
       V_BSTR=8, V_ERROR=10, V_BOOL=11, V_VARIANT=12, V_DECIMAL=14, V_UI1=17, V_BYREF=0x4000 };
/* Native-only extension, never an Automation VT_DISPATCH/VT_UNKNOWN payload.
   External Automation boundaries must marshal this value through an adapter. */
enum { V_PRIVATE_OBJECT=0x1000,V_PRIVATE_ARRAY_CAPTURE=0x0800 };
int32_t vba_variant_box_object(VbaVariant *out,void *object);
int32_t vba_variant_object(void *out,const VbaVariant *value);

/* Producing helpers require fresh/empty output storage. On failure output is
   untouched. Use store to replace a live value: it acquires before releasing,
   dereferences typed views, and commits only after successful coercion. */
void vba_variant_clear(VbaVariant *value);
int32_t vba_variant_load(VbaVariant *out, const VbaVariant *source);
int32_t vba_variant_store(VbaVariant *destination, const VbaVariant *source);
int32_t vba_variant_box_number(VbaVariant *out, int32_t tag, int64_t value);
int32_t vba_variant_box_float(VbaVariant *out, int32_t tag, double value);
int32_t vba_variant_box_string(VbaVariant *out, const VbaString *value);
int32_t vba_variant_to(void *out, const VbaVariant *source, int32_t tag);
int32_t vba_variant_binary(VbaVariant *out, const VbaVariant *a, const VbaVariant *b, int32_t operation);
int32_t vba_variant_unary(VbaVariant *out, const VbaVariant *a, int32_t operation);
int32_t vba_variant_query(const VbaVariant *source, int32_t operation);
int32_t vba_variant_len(VbaVariant *out, const VbaVariant *source, int32_t bytes);
int32_t vba_string_load(VbaString *out, const VbaString *source);
int32_t vba_string_store(VbaString *destination, const VbaString *source);
int32_t vba_string_pin(VbaString *slot);
void vba_bstr_free(void *value);
#endif
