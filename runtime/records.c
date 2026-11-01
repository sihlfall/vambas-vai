#include "records.h"
#include "runtime_internal.h"
#include <stdlib.h>
#include <string.h>
#include <limits.h>

/* All record outputs start zeroed. Every source-owned payload is copied before
   replacement. No callbacks occur during preparation/commit. */
static void *record_cell(void *record,const VbaRecordField *field) {
    return (unsigned char*)record+field->offset;
}
static const void *record_const_cell(const void *record,const VbaRecordField *field) {
    return (const unsigned char*)record+field->offset;
}
void vba_record_clear(void *record,const VbaRecordType *type) {
    for(uint32_t i=type->count;i>0;i--) {
        const VbaRecordField *f=&type->fields[i-1];void *cell=record_cell(record,f);
        switch(f->kind) {
        case R_STRING: vba_internal_string_drop(*(VbaString*)cell); break;
        case R_VARIANT: vba_variant_clear(cell); break;
        case R_ARRAY: vba_array_clear(cell); break;
        case R_RECORD: vba_record_clear(cell,f->nested); break;
        case R_OBJECT: vba_object_clear(cell);break;
        default: break;
        }
    }
    memset(record,0,(size_t)type->size);
}
int32_t vba_record_init(void *record,const VbaRecordType *type) {
    for(uint32_t i=0;i<type->count;i++) {
        const VbaRecordField *f=&type->fields[i];void *cell=record_cell(record,f);
        int error=0;
        if(f->kind==R_STRING) ((VbaString*)cell)->length=-1;
        else if(f->kind==R_RECORD) error=vba_record_init(cell,f->nested);
        else if(f->kind==R_ARRAY && f->rank) {
 if(f->tag==36) error=vba_record_array_redim(cell,f->nested,2,(int32_t)f->rank,f->bounds);
 else if(f->tag==V_PRIVATE_OBJECT) error=vba_object_array_redim(cell,V_PRIVATE_OBJECT,(const VbaObjectArrayType*)f->nested,2,(int32_t)f->rank,f->bounds);
 else error=vba_array_redim(cell,(int32_t)f->tag,(int32_t)f->tag,2,(int32_t)f->rank,f->bounds);
 }
        if(error) {vba_record_clear(record,type);return error;}
    }
    return 0;
}
int32_t vba_record_load(void *out,const void *source,const VbaRecordType *type) {
    for(uint32_t i=0;i<type->count;i++) {
        const VbaRecordField *f=&type->fields[i];void *dst=record_cell(out,f);
        const void *src=record_const_cell(source,f);
        int error=0;
        switch(f->kind) {
        case R_STRING: error=vba_string_load(dst,src);break;
        case R_VARIANT: error=vba_variant_load(dst,src);break;
        case R_ARRAY:
            error=vba_array_copy(dst,*(VbaArray*const*)src);
            if(!error && f->rank && *(VbaArray**)dst) (*(VbaArray**)dst)->features|=A_FIXED;
            break;
        case R_RECORD: error=vba_record_load(dst,src,f->nested);break;
        case R_OBJECT: error=vba_object_load(dst,(VbaObject**)src,0);break;
        default: memcpy(dst,src,(size_t)f->size);break;
        }
        if(error) {vba_record_clear(out,type);return error;}
    }
    return 0;
}
int vba_internal_record_replaceable(const void *record,const VbaRecordType *type) {
    for(uint32_t i=0;i<type->count;i++) {
        const VbaRecordField *f=&type->fields[i];const void *cell=record_const_cell(record,f);
        int error=0;
        if(f->kind==R_ARRAY) error=vba_array_check_unlocked(*(VbaArray*const*)cell);
        else if(f->kind==R_VARIANT) {
            const VbaVariant *v=cell;
            /* Real record fields own values, never borrowed typed views. */
            if(v->vt&V_BYREF) return 13;
            if(v->vt&V_ARRAY) error=vba_array_check_unlocked(v->value.pointer);
        } else if(f->kind==R_RECORD) error=vba_internal_record_replaceable(cell,f->nested);
        if(error) return error;
    }
    return 0;
}
static int record_prepare_aliases(void *next,const void *destination,const VbaRecordType *type) {
    for(uint32_t i=0;i<type->count;i++) {
        const VbaRecordField *f=&type->fields[i];void *dst=record_cell(next,f);
        const void *old=record_const_cell(destination,f);int error=0;
        /* A typed ByRef Variant can already point at this String's BSTR*
           slot. Keep that representation after the aggregate replacement. */
        if(f->kind==R_STRING && ((const VbaString*)old)->length==-2) error=vba_string_pin(dst);
        else if(f->kind==R_RECORD) error=record_prepare_aliases(dst,old,f->nested);
        if(error) return error;
    }
    return 0;
}
int32_t vba_record_store(void *destination,const void *source,const VbaRecordType *type) {
    int error=vba_internal_record_replaceable(destination,type);if(error) return error;
    if(destination==source) return 0;
    void *next=malloc((size_t)type->size);if(!next) return 7;
    memset(next,0,(size_t)type->size);
    error=vba_record_load(next,source,type);
    if(!error) error=record_prepare_aliases(next,destination,type);
    if(!error) {
        /* Publish every new field before releasing old references: a child
           terminator may reenter and inspect the destination record. */
        for(size_t i=0;i<(size_t)type->size;i++) {
            unsigned char previous=((unsigned char*)destination)[i];
            ((unsigned char*)destination)[i]=((unsigned char*)next)[i];
            ((unsigned char*)next)[i]=previous;
        }
    }
    vba_record_clear(next,type);free(next);return error;
}

int32_t vba_record_array_redim(VbaArray **slot,const VbaRecordType *type,int32_t flags,int32_t rank,const int32_t *bounds) {
 VbaArray *old=*slot;
 if(vba_array_check_unlocked(old) || (old && (old->features&A_FIXED))) return 10;
 if(old && vba_internal_array_record_type(old)!=type) return 13;
 if(rank<1 || rank>60) return 9;
 VbaArrayBound shape[60];
 for(int i=0;i<rank;i++) {int64_t count=(int64_t)bounds[2*i+1]-bounds[2*i]+1;if(count<=0 || count>UINT32_MAX)return 9;shape[i]=(VbaArrayBound){(uint32_t)count,bounds[2*i]};}
 if((flags&1) && old) {
  if(old->dimensions!=rank)return 9;
  for(int i=0;i<rank;i++) {VbaArrayBound b=old->bounds[rank-i-1];if(b.lower!=shape[i].lower || (i!=rank-1 && b.count!=shape[i].count))return 9;}
 }
 VbaArray *next=NULL;int error=vba_internal_array_create_typed(&next,36,rank,shape,type);if(error)return error;
 size_t copied=0,count=vba_internal_array_count(next);
 if((flags&1) && old) {size_t before=vba_internal_array_count(old);copied=before<count?before:count;error=vba_internal_array_copy_elements(next,old,copied);}
 for(size_t i=copied;!error && i<count;i++) error=vba_record_init((char*)next->data+i*next->element_size,type);
 if(!error) error=vba_array_destroy(old);
 if(error) {vba_array_destroy(next);return error;}
 if(flags&2) next->features|=A_FIXED;
 *slot=next;return 0;
}
int32_t vba_record_array_index(VbaElement *out,VbaArray **slot,const VbaRecordType *type,int32_t rank,const int32_t *indices) {
 if(*slot && vba_internal_array_record_type(*slot)!=type)return 13;
 return vba_array_index(out,slot,36,36,rank,indices);
}
int32_t vba_record_array_erase(VbaArray **slot,const VbaRecordType *type) {
 VbaArray *a=*slot;if(!a)return 0;
 if(vba_internal_array_record_type(a)!=type)return 13;
 if(vba_array_check_unlocked(a))return 10;
 if(!(a->features&A_FIXED))return vba_array_erase(slot,36);
 VbaArrayBound bounds[60];for(unsigned i=0;i<a->dimensions;i++)bounds[i]=a->bounds[a->dimensions-i-1];
 VbaArray *next=NULL;int e=vba_internal_array_create_typed(&next,36,a->dimensions,bounds,type);if(e)return e;
 for(size_t i=0;!e && i<vba_internal_array_count(next);i++) e=vba_record_init((char*)next->data+i*next->element_size,type);
 if(!e){void *data=a->data;a->data=next->data;next->data=data;}
 vba_array_destroy(next);return e;
}
