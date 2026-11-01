#ifndef VBA_RECORDS_H
#define VBA_RECORDS_H
#include "arrays.h"

/* Compiler-private UDT storage. Managed fields retain the existing private
   String/Automation Variant/SAFEARRAY representations. This is not IRecordInfo. */
typedef struct VbaRecordType VbaRecordType;
typedef struct {
    uint32_t kind, tag;
    uint64_t offset, size;
    const VbaRecordType *nested;
    uint32_t rank;
    const int32_t *bounds;
} VbaRecordField;
struct VbaRecordType {
    uint64_t size;
    uint32_t count;
    const VbaRecordField *fields;
};
enum { R_SCALAR, R_STRING, R_VARIANT, R_ARRAY, R_RECORD, R_OBJECT };
_Static_assert(sizeof(VbaRecordField)==48 && offsetof(VbaRecordField,bounds)==40,"record field metadata layout");
_Static_assert(sizeof(VbaRecordType)==24 && offsetof(VbaRecordType,fields)==16,"record type metadata layout");
int32_t vba_record_array_redim(VbaArray **slot,const VbaRecordType *type,int32_t flags,int32_t rank,const int32_t *bounds);
int32_t vba_record_array_index(VbaElement *out,VbaArray **slot,const VbaRecordType *type,int32_t rank,const int32_t *indices);
int32_t vba_record_array_erase(VbaArray **slot,const VbaRecordType *type);
int32_t vba_record_init(void *record,const VbaRecordType *type);
int32_t vba_record_load(void *out,const void *source,const VbaRecordType *type);
int32_t vba_record_store(void *destination,const void *source,const VbaRecordType *type);
void vba_record_clear(void *record,const VbaRecordType *type);
#endif
