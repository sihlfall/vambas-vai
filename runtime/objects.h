#ifndef VBA_OBJECTS_H
#define VBA_OBJECTS_H
#include "variant.h"
/* Private runtime references. These are not Automation interface pointers. */
typedef struct VbaRecordType VbaRecordType;
typedef struct VbaArray VbaArray;
typedef struct VbaObject VbaObject;
typedef int32_t (*VbaLifecycle)(void *error,VbaObject *object);
typedef struct VbaInvokeArg VbaInvokeArg;
typedef int32_t (*VbaInvoke)(void *error,VbaVariant *out,VbaObject *object,const char *name,int32_t flags,int32_t count,const VbaInvokeArg *args);
typedef struct VbaInterface VbaInterface;
typedef struct VbaObjectType {
 uint64_t size;
 const VbaRecordType *fields; /* Payload immediately follows the private header. */
 void (*dispose)(VbaObject *object); /* Releases custom payload; allocator frees header. */
 VbaLifecycle initialize,terminate;
 VbaInvoke invoke;
 uint64_t interface_count;
 const VbaInterface *interfaces;
} VbaObjectType;
typedef void (*VbaInterfaceMethod)(void); /* Erased native entry point; caller uses its checked signature. */
typedef struct {uint32_t member;VbaInterfaceMethod call;} VbaInterfaceEntry;
struct VbaInterface {const VbaObjectType *type;uint64_t count;const VbaInterfaceEntry *methods;};
_Static_assert(sizeof(VbaInterfaceEntry)==16 && offsetof(VbaInterfaceEntry,call)==8,"interface entry layout");
_Static_assert(sizeof(VbaInterface)==24,"interface descriptor layout");
struct VbaObject {uint64_t references;int32_t kind;uint32_t flags;const VbaObjectType *type;};
_Static_assert(sizeof(VbaObject)==24 && offsetof(VbaObject,type)==16,"object header layout");
_Static_assert(sizeof(VbaObjectType)==64 && offsetof(VbaObjectType,invoke)==40 && offsetof(VbaObjectType,interfaces)==56,"object descriptor layout");
enum { O_ASSERT=1,O_PERMISSIVE_ASSERT=2,O_FAKES=3,O_COLLECTION=4 };
typedef struct {const char *name;int32_t kind;const void *value;} VbaObjectArg;
_Static_assert(sizeof(VbaObjectArg)==24 && offsetof(VbaObjectArg,value)==16,"object argument layout");
int32_t vba_object_check(VbaObject *object,int32_t kind);
int32_t vba_object_new(VbaObject **out,int32_t kind);
int32_t vba_object_allocate(VbaObject **out,const VbaObjectType *type,int32_t kind);
int32_t vba_class_new(VbaObject **out,const VbaObjectType *type);
int32_t vba_class_construct(VbaObject **out,const VbaObjectType *type,void *error);
int32_t vba_class_load(VbaObject **out,VbaObject **slot,const VbaObjectType *type,void *error);
int32_t vba_class_check(VbaObject *object,const VbaObjectType *type,int32_t nullable);
int32_t vba_class_check_exact(VbaObject *object,const VbaObjectType *type);
int32_t vba_interface_resolve(VbaInterfaceMethod *out,VbaObject *object,const VbaObjectType *type,uint32_t member,VbaInterfaceMethod fallback);
void vba_object_retain(VbaObject *object);
void vba_object_clear(VbaObject **slot);
int32_t vba_object_load(VbaObject **out,VbaObject **slot,int32_t auto_kind);
int32_t vba_object_store(VbaObject **slot,VbaObject *object);
int32_t vba_create_object(VbaObject **out,const VbaString *name);

enum { I_METHOD=1,I_GET=2,I_LET=4,I_SET=8,I_DISCARD=16,P_BYREF=1,P_OPTIONAL=2,P_PARAMARRAY=4 };
struct VbaInvokeArg {const char *name;VbaVariant value;};
typedef int32_t (*VbaDefault)(void *error,void *out);
typedef struct {const char *name;int32_t flags,tag;const VbaObjectType *type;int32_t builtin;VbaDefault fallback;} VbaInvokeParameter;
typedef struct {
 VbaVariant value;
 uint64_t native[3];
 void *address;
 const VbaInvokeArg *actual;
 int32_t owns,copyback;
} VbaBinding;
typedef int32_t (*VbaMemberCall)(void *error,VbaVariant *out,VbaObject *object,VbaBinding *bindings);
typedef struct {const char *name;int32_t flags,is_default,count;const VbaInvokeParameter *parameters;VbaMemberCall call;} VbaMember;
_Static_assert(sizeof(VbaInvokeArg)==32 && offsetof(VbaInvokeArg,value)==8,"invocation argument layout");
_Static_assert(sizeof(VbaBinding)==72 && offsetof(VbaBinding,address)==48,"invocation binding layout");
_Static_assert(sizeof(VbaInvokeParameter)==40 && offsetof(VbaInvokeParameter,fallback)==32,"invocation parameter layout");
_Static_assert(sizeof(VbaMember)==40 && offsetof(VbaMember,call)==32,"invocation member layout");
void vba_invoke_argument(VbaInvokeArg *out,const char *name,const VbaVariant *value,int32_t byref);
int32_t vba_object_invoke(void *error,VbaVariant *out,const VbaVariant *receiver,const char *name,int32_t flags,int32_t count,const VbaInvokeArg *args);
int32_t vba_invoke_members(void *error,VbaVariant *out,VbaObject *object,const char *name,int32_t flags,int32_t count,const VbaInvokeArg *args,int32_t member_count,const VbaMember *members);
int32_t vba_builtin_invoke(void *error,VbaVariant *out,VbaObject *object,const char *name,int32_t flags,int32_t count,const VbaInvokeArg *args);
int32_t vba_native_box(VbaVariant *out,const void *native,int32_t tag);
int32_t vba_native_from(void *out,const VbaVariant *value,int32_t tag,const VbaObjectType *type,int32_t builtin);
void vba_native_clear(void *native,int32_t tag);
int32_t vba_object_default(VbaVariant *out,void *object);
int32_t vba_object_value(VbaVariant *out,const VbaVariant *source);
int32_t vba_object_value_at(VbaVariant *out,const VbaVariant *source,void *error);
int32_t vba_variant_to_at(void *out,const VbaVariant *source,int32_t tag,void *error);
void *vba_invocation_error_push(void *error);
void vba_invocation_error_pop(void *previous);
int32_t vba_invocation_capture(VbaVariant *out,const VbaVariant *source);
void vba_capture_clear(VbaVariant *capture);
int32_t vba_capture_index(void *out,const VbaVariant *capture,int32_t rank,const int32_t *indices);
int32_t vba_field_invoke(void *error,VbaVariant *out,void *field,int32_t tag,const VbaObjectType *type,int32_t builtin,int32_t flags,VbaArray *indices,const VbaVariant *rhs,const VbaObjectType *auto_type,int32_t auto_builtin);
#endif
