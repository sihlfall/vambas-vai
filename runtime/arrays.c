#include "arrays.h"
#include "runtime_internal.h"
#include <stdlib.h>
#include <string.h>
#include <limits.h>
#ifdef _WIN32
#include <windows.h>
#include <oleauto.h>
#endif

/* Record arrays have private metadata before the SAFEARRAY-shaped descriptor.
   Their elements use private UDT layout, not an Automation IRecordInfo ABI. */
const VbaRecordType *vba_internal_array_record_type(VbaArray *a) {
 const VbaRecordType *type=NULL;
 if(a && (a->features&A_RECORD)) memcpy(&type,(char*)a-16,sizeof(type));
 return type;
}
/* Private object arrays carry a nominal constraint before the descriptor.
   The descriptor keeps SAFEARRAY shape, but SDK object/Variant functions must
   never interpret its private reference payload. */
static VbaObjectArrayType object_array_type(VbaArray *a) {
    VbaObjectArrayType type={0};
    if(a && (a->features&A_PRIVATE_OBJECT))memcpy(&type,(char*)a-24,sizeof(type));
    return type;
}
static int same_object_array_type(VbaObjectArrayType a,VbaObjectArrayType b) {
    return a.type==b.type && a.builtin==b.builtin;
}
int vba_internal_array_lock(VbaArray *a) {
    if(!a)return 0;
#ifdef _WIN32
    if(!(a->features&A_PRIVATE_OBJECT))return FAILED(SafeArrayLock((SAFEARRAY*)a))?10:0;
#endif
    if(a->locks==UINT32_MAX)return 10;
    a->locks++;return 0;
}
void vba_internal_array_unlock(VbaArray *a) {
    if(!a)return;
#ifdef _WIN32
    if(!(a->features&A_PRIVATE_OBJECT)){SafeArrayUnlock((SAFEARRAY*)a);return;}
#endif
    a->locks--;
}
int32_t vba_array_lock(VbaArray *a) { return vba_internal_array_lock(a); }
void vba_array_unlock(VbaArray *a) { vba_internal_array_unlock(a); }
static int object_array_create(VbaArray **out,const VbaObjectArrayType *type,int rank,const VbaArrayBound *bounds) {
    if(!type || type->builtin<0 || type->builtin>4 || (type->type && type->builtin) || rank<1 || rank>60)return 13;
    size_t count=1;
    for(int i=0;i<rank;i++) {if(bounds[i].count && count>SIZE_MAX/bounds[i].count)return 7;count*=bounds[i].count;}
    if(count>SIZE_MAX/sizeof(void*))return 7;
    size_t header=24+offsetof(VbaArray,bounds)+(size_t)rank*sizeof(VbaArrayBound);
    unsigned char *allocation=malloc(header);if(!allocation)return 7;
    memset(allocation,0,header);memcpy(allocation,type,sizeof(*type));
    VbaArray *a=(VbaArray*)(allocation+24);
    a->dimensions=(uint16_t)rank;a->features=A_PRIVATE_OBJECT;a->element_size=sizeof(void*);
    for(int i=0;i<rank;i++)a->bounds[rank-i-1]=bounds[i];
    if(count){a->data=malloc(count*sizeof(void*));if(!a->data){free(allocation);return 7;}memset(a->data,0,count*sizeof(void*));}
    *out=a;return 0;
}
/* Scalar coercions and BSTR ownership use the shared internal interface.
   No compiler-private pointer is stored in an Automation payload. */
uint32_t vba_internal_array_element_size(uint16_t tag) {
    switch(tag) {
    case V_UI1: return 1;
    case V_I2: case V_BOOL: return 2;
    case V_I4: case V_R4: return 4;
    case V_R8: case V_DATE: case V_CY: return 8;
    case V_BSTR: case V_PRIVATE_OBJECT: return sizeof(void*);
    case V_VARIANT: return sizeof(VbaVariant);
    default: return 0;
    }
}
uint16_t vba_internal_array_tag(VbaArray *a) {
    if(!a) return 0;
    if(a->features&A_RECORD) return 36;
    if(a->features&A_PRIVATE_OBJECT) return V_PRIVATE_OBJECT;
#ifdef _WIN32
    VARTYPE tag=0;
    if(FAILED(SafeArrayGetVartype((SAFEARRAY*)a,&tag))) return 0;
    return tag;
#else
    uint32_t tag=0;
    if(a->features&A_VARTYPE) memcpy(&tag,(char*)a-4,4);
    else if(a->features&A_BSTR) tag=V_BSTR;
    else if(a->features&A_VARIANT) tag=V_VARIANT;
    return (uint16_t)tag;
#endif
}
size_t vba_internal_array_count(VbaArray *a) {
    size_t n=1;
    for(unsigned i=0;i<a->dimensions;i++) n*=a->bounds[i].count;
    return n;
}
int vba_internal_array_create_typed(VbaArray **out,uint16_t tag,int rank,const VbaArrayBound *bounds,const VbaRecordType *record) {
    if(tag==V_PRIVATE_OBJECT){VbaObjectArrayType type={0};return object_array_create(out,&type,rank,bounds);}
    uint32_t size=record && record->size<=UINT32_MAX ? (uint32_t)record->size : vba_internal_array_element_size(tag);
    if(!size || rank<1 || rank>60) return 13;
    size_t count=1;
    for(int i=0;i<rank;i++) {
        if(bounds[i].count && count>SIZE_MAX/bounds[i].count) return 7;
        count*=bounds[i].count;
    }
    if(count>SIZE_MAX/size) return 7;
    if(record) {
        size_t header=16+offsetof(VbaArray,bounds)+(size_t)rank*sizeof(VbaArrayBound);
        unsigned char *allocation=malloc(header);
        if(!allocation) return 7;
        memset(allocation,0,header);
        memcpy(allocation,&record,sizeof(record));
        uint32_t record_tag=36;memcpy(allocation+12,&record_tag,4);
        VbaArray *a=(VbaArray*)(allocation+16);
        a->dimensions=(uint16_t)rank;a->features=A_RECORD|A_VARTYPE;a->element_size=size;
        for(int i=0;i<rank;i++) a->bounds[rank-i-1]=bounds[i];
        if(count) {a->data=malloc(count*size);if(!a->data){free(allocation);return 7;}memset(a->data,0,count*size);}
        *out=a;return 0;
    }
#ifdef _WIN32
    VbaArray *a=(VbaArray*)SafeArrayCreate(tag,(UINT)rank,(SAFEARRAYBOUND*)bounds);
    if(!a) return 7;
#else
    size_t header=8+offsetof(VbaArray,bounds)+(size_t)rank*sizeof(VbaArrayBound);
    unsigned char *allocation=malloc(header);
    if(!allocation) return 7;
    memset(allocation,0,header);
    uint32_t vartype=tag;
    memcpy(allocation+4,&vartype,4);
    VbaArray *a=(VbaArray*)(allocation+8);
    a->dimensions=(uint16_t)rank;
    a->features=A_VARTYPE | (tag==V_BSTR ? A_BSTR : tag==V_VARIANT ? A_VARIANT : 0);
    a->element_size=size;
    /* SafeArrayCreate takes source-order bounds; descriptors store them reversed.
       Data is column-major: the first source subscript varies fastest. */
    for(int i=0;i<rank;i++) a->bounds[rank-i-1]=bounds[i];
    if(count) {
        a->data=malloc(count*size);
        if(!a->data) { free(allocation); return 7; }
        memset(a->data,0,count*size);
    }
#endif
    *out=a;
    return 0;
}
int vba_internal_array_create(VbaArray **out,uint16_t tag,int rank,const VbaArrayBound *bounds) {
 return vba_internal_array_create_typed(out,tag,rank,bounds,NULL);
}
static int array_check_children_unlocked(VbaArray *a) {
    const VbaRecordType *record=vba_internal_array_record_type(a);
    if(record) {for(size_t i=0;i<vba_internal_array_count(a);i++) {int e=vba_internal_record_replaceable((char*)a->data+i*a->element_size,record);if(e) return e;}}
    if(vba_internal_array_tag(a)==V_VARIANT) {
        VbaVariant *p=a->data;
        size_t count=vba_internal_array_count(a);
        for(size_t i=0;i<count;i++) {
            if((p[i].vt&V_ARRAY) && !(p[i].vt&V_BYREF) && vba_array_check_unlocked(p[i].value.pointer)) return 10;
        }
    }
    return 0;
}
int32_t vba_array_check_unlocked(VbaArray *a) {
    if(!a) return 0;
    if(a->locks) return 10;
    return array_check_children_unlocked(a);
}
static int array_clear_data(VbaArray *a,int reset_fixed) {
    int error=reset_fixed ? array_check_children_unlocked(a) : vba_array_check_unlocked(a);
    if(error) return error;
    uint16_t tag=vba_internal_array_tag(a);
    size_t n=vba_internal_array_count(a);
    const VbaRecordType *record=vba_internal_array_record_type(a);
    if(record) {for(size_t i=0;i<n;i++) vba_record_clear((char*)a->data+i*a->element_size,record);}
    else if(tag==V_BSTR) {
        void **p=a->data;
        for(size_t i=0;i<n;i++) vba_bstr_free(p[i]);
    } else if(tag==V_PRIVATE_OBJECT) {
        VbaObject **p=a->data;
        if(reset_fixed) {
            if(!n)return 0;
            VbaObject **old=malloc(n*sizeof(*old));if(!old)return 7;
            memcpy(old,p,n*sizeof(*old));memset(p,0,n*sizeof(*p));
            for(size_t i=0;i<n;i++)vba_object_clear(old+i);
            free(old);return 0;
        }
        for(size_t i=0;i<n;i++)vba_object_clear(p+i);
    } else if(tag==V_VARIANT) {
        VbaVariant *p=a->data;
        for(size_t i=0;i<n;i++) vba_variant_clear(&p[i]);
    }
    if(n) memset(a->data,0,n*a->element_size);
    return 0;
}
int32_t vba_array_destroy(VbaArray *a) {
    if(!a) return 0;
    if(vba_array_check_unlocked(a)) return 10;
    if(a->features&A_PRIVATE_OBJECT) {
        int error=array_clear_data(a,0);if(error)return error;
        if(a->data)free(a->data);free((char*)a-24);return 0;
    }
    if(vba_internal_array_record_type(a)) {
        int error=array_clear_data(a,0);if(error) return error;
        if(a->data) free(a->data);free((char*)a-16);return 0;
    }
#ifdef _WIN32
    /* SDK VariantClear does not know our private object/array tags. Clear
       Variant elements ourselves before the SDK releases their empty slots. */
    if(vba_internal_array_tag(a)==V_VARIANT) {int e=array_clear_data(a,0);if(e)return e;}
    HRESULT hr=SafeArrayDestroy((SAFEARRAY*)a);
    return hr==DISP_E_ARRAYISLOCKED ? 10 : FAILED(hr) ? 13 : 0;
#else
    int error=array_clear_data(a,0);
    if(error) return error;
    if(a->data) free(a->data);
    free((char*)a-8);
    return 0;
#endif
}
void vba_array_clear(VbaArray **slot) {
    VbaArray *old=*slot;if(vba_array_check_unlocked(old))return;
    *slot=NULL;vba_array_destroy(old);
}
int vba_internal_array_copy_elements(VbaArray *out,VbaArray *source,size_t count) {
    uint16_t tag=vba_internal_array_tag(source);
    const VbaRecordType *record=vba_internal_array_record_type(source);
    if(record) {
        for(size_t i=0;i<count;i++) {int e=vba_record_load((char*)out->data+i*out->element_size,(char*)source->data+i*source->element_size,record);if(e) return e;}
    } else if(tag==V_BSTR) {
        uint16_t **src=source->data, **dst=out->data;
        for(size_t i=0;i<count;i++) {
            if(src[i]) {
                dst[i]=vba_internal_bstr_alloc(src[i],vba_internal_bstr_len(src[i]));
                if(!dst[i]) return 14;
            }
        }
    } else if(tag==V_PRIVATE_OBJECT) {
        VbaObject **src=source->data,**dst=out->data;
        for(size_t i=0;i<count;i++){vba_object_retain(src[i]);dst[i]=src[i];}
    } else if(tag==V_VARIANT) {
        VbaVariant *src=source->data, *dst=out->data;
        for(size_t i=0;i<count;i++) {
            int error=vba_variant_load(&dst[i],&src[i]);
            if(error) return error;
        }
    } else if(count) memcpy(out->data,source->data,count*source->element_size);
    return 0;
}
int32_t vba_array_copy(VbaArray **out,VbaArray *source) {
    if(!source) { *out=NULL; return 0; }
    const VbaRecordType *record=vba_internal_array_record_type(source);
    if(record) {
        VbaArrayBound bounds[60];if(source->dimensions<1 || source->dimensions>60) return 13;
        for(unsigned i=0;i<source->dimensions;i++) bounds[i]=source->bounds[source->dimensions-i-1];
        VbaArray *copy=NULL;int e=vba_internal_array_create_typed(&copy,36,source->dimensions,bounds,record);
        if(!e) e=vba_internal_array_copy_elements(copy,source,vba_internal_array_count(source));
        if(e){if(copy) vba_array_destroy(copy);return e;}*out=copy;return 0;
    }
    if(source->features&A_PRIVATE_OBJECT) {
        VbaArrayBound bounds[60];if(source->dimensions<1 || source->dimensions>60)return 13;
        for(unsigned i=0;i<source->dimensions;i++)bounds[i]=source->bounds[source->dimensions-i-1];
        VbaObjectArrayType type=object_array_type(source);VbaArray *copy=NULL;
        int e=object_array_create(&copy,&type,source->dimensions,bounds);
        if(!e)e=vba_internal_array_copy_elements(copy,source,vba_internal_array_count(source));
        if(e){vba_array_destroy(copy);return e;}*out=copy;return 0;
    }
    if(!vba_internal_array_element_size(vba_internal_array_tag(source))) return 13;
#ifdef _WIN32
    /* VariantCopy (used by SafeArrayCopy) retains VT_BYREF wrappers. Owned
       language copies must dereference ParamArray bindings, including nested
       Variant arrays, so they cannot escape with borrowed argument storage. */
    if(vba_internal_array_tag(source)!=V_VARIANT) {
        VbaArray *copy=NULL;
        HRESULT hr=SafeArrayCopy((SAFEARRAY*)source,(SAFEARRAY**)&copy);
        if(FAILED(hr)) return hr==E_OUTOFMEMORY ? 7 : 13;
        copy->features&=~A_FIXED;*out=copy;return 0;
    }
#endif
    VbaArrayBound bounds[60];
    if(source->dimensions<1 || source->dimensions>60) return 13;
    for(unsigned i=0;i<source->dimensions;i++) bounds[i]=source->bounds[source->dimensions-i-1];
    VbaArray *copy=NULL;
    int error=vba_internal_array_create(&copy,vba_internal_array_tag(source),source->dimensions,bounds);
    if(error) return error;
    error=vba_internal_array_copy_elements(copy,source,vba_internal_array_count(source));
    if(error) { vba_array_destroy(copy); return error; }
    *out=copy;
    return 0;
}
int32_t vba_array_store(VbaArray **slot,VbaArray *source) {
    VbaArray *old=*slot, *copy=NULL;
    if(vba_array_check_unlocked(old) || (old && (old->features&A_FIXED))) return 10;
    int error=vba_array_copy(&copy,source);
    if(error) return error;
    *slot=copy;
    vba_array_destroy(old);
    return 0;
}
int32_t vba_array_box(VbaVariant *out,VbaArray *source,int32_t tag) {
    if(!vba_internal_array_element_size((uint16_t)tag) || (source && vba_internal_array_tag(source)!=tag)) return 13;
    VbaVariant value={0}; value.vt=V_ARRAY|(uint16_t)tag;
    int error=vba_array_copy((VbaArray**)&value.value.pointer,source);
    if(!error) {
        if(tag==V_PRIVATE_OBJECT){VbaObjectArrayType type=object_array_type(source);value.value.record.info=(void*)type.type;value.reserved1=(uint16_t)type.builtin;}
        *out=value;
    }
    return error;
}
int32_t vba_object_array_box(VbaVariant *out,VbaArray *source,const VbaObjectArrayType *type) {
    if(!type || type->builtin<0 || type->builtin>4 || (type->type && type->builtin) || (source && (!(source->features&A_PRIVATE_OBJECT) || !same_object_array_type(object_array_type(source),*type))))return 13;
    int e=vba_array_box(out,source,V_PRIVATE_OBJECT);
    if(!e){out->value.record.info=(void*)type->type;out->reserved1=(uint16_t)type->builtin;}return e;
}
int vba_internal_object_array_check(const VbaVariant *source,const VbaObjectArrayType *type) {
    VbaVariant value={0};int e=vba_internal_borrow(&value,source);if(e)return e;
    if(value.vt!=(V_ARRAY|V_PRIVATE_OBJECT) || !type || type->builtin<0 || type->builtin>4 || (type->type && type->builtin))return 13;
    VbaObjectArrayType actual={value.value.record.info,value.reserved1};
    VbaArray *array=value.value.pointer;
    if(!same_object_array_type(actual,*type) || (array && (!(array->features&A_PRIVATE_OBJECT) || array->element_size!=sizeof(void*) || !same_object_array_type(object_array_type(array),*type))))return 13;
    return 0;
}
int32_t vba_object_array_unbox(VbaArray **out,const VbaVariant *source,const VbaObjectArrayType *type) {
    int e=vba_internal_object_array_check(source,type);if(e)return e;
    VbaVariant value={0};e=vba_internal_borrow(&value,source);if(e)return e;
    return vba_array_copy(out,value.value.pointer);
}
/* Assignment uses UTF-16LE bytes without a length prefix or terminator.
   Source bounds/rank do not change the contiguous order of the byte payload.
   A final incomplete UTF-16 code unit is discarded (MS-VBAL 5.5.1.2.6). */
int vba_internal_byte_array_bstr(VbaVariant *out,VbaArray *source) {
    if(source && (vba_internal_array_tag(source)!=V_UI1 || source->element_size!=1)) return 13;
    size_t count=source ? vba_internal_array_count(source)/2 : 0;
    if(count>INT32_MAX) return 14;
    uint16_t *text=vba_internal_bstr_alloc(NULL,(uint32_t)count);
    if(!text) return 14;
    const unsigned char *bytes=source ? source->data : NULL;
    for(size_t i=0;i<count;i++) text[i]=(uint16_t)(bytes[2*i]|((uint16_t)bytes[2*i+1]<<8));
    *out=(VbaVariant){.vt=V_BSTR,.value.pointer=text};
    return 0;
}
int vba_internal_byte_array_from_bstr(VbaArray **out,const uint16_t *text) {
    uint32_t length=vba_internal_bstr_len(text);
    if(length>INT32_MAX/2) return 6;
    VbaArrayBound bound={.count=length*2,.lower=0};
    VbaArray *array=NULL;
    int error=vba_internal_array_create(&array,V_UI1,1,&bound);
    if(error) return error;
    unsigned char *bytes=array->data;
    for(uint32_t i=0;i<length;i++) {
        bytes[2*i]=(unsigned char)text[i];
        bytes[2*i+1]=(unsigned char)(text[i]>>8);
    }
    *out=array;
    return 0;
}
int32_t vba_array_unbox(VbaArray **out,const VbaVariant *source,int32_t tag) {
    VbaVariant value;
    int error=vba_internal_borrow(&value,source);
    if(error) return error;
    if(tag==V_UI1 && value.vt==V_BSTR) return vba_internal_byte_array_from_bstr(out,value.value.pointer);
    if(value.vt!=(V_ARRAY|tag)) return 13;
    return vba_array_copy(out,value.value.pointer);
}

/* Return a descriptor slot without copying it. A typed array exposed through a
   Variant retains its element type constraint; a real Variant may be retagged. */
int vba_internal_array_location(void *slot,int declared,VbaArray ***location,VbaVariant **variant,int *constraint) {
    *variant=NULL; *constraint=declared;
    if(declared) { *location=slot; return 0; }
    VbaVariant *v=slot;
    if(v->vt==(V_BYREF|V_VARIANT)) {
        v=v->value.pointer;
        if(!v || v->vt==(V_BYREF|V_VARIANT)) return 13;
    }
    if(v->vt&V_ARRAY) {
        if(!vba_internal_supported(v->vt)) return 13;
        if(v->vt&V_BYREF) {
            if(!v->value.pointer) return 13;
            *location=v->value.pointer;
            *constraint=v->vt&~(V_ARRAY|V_BYREF);
        } else { *location=(VbaArray**)&v->value.pointer; *variant=v; }
        return 0;
    }
    if(v->vt!=V_EMPTY) return 13;
    *location=(VbaArray**)&v->value.pointer; *variant=v;
    return 0;
}
static int array_redim(void *slot,int32_t declared,int32_t requested,int32_t flags,int32_t rank,const int32_t *bounds,const VbaObjectArrayType *object_type) {
    VbaArray **location=NULL;
    VbaVariant *variant=NULL;
    int constraint=0;
    int error=vba_internal_array_location(slot,declared,&location,&variant,&constraint);
    if(error) return error;
    VbaArray *old=*location;
    if(vba_array_check_unlocked(old) || (old && (old->features&A_FIXED))) return 10;
    int tag=requested ? requested : constraint ? constraint : old ? vba_internal_array_tag(old) : variant && (variant->vt&V_ARRAY) ? variant->vt&~V_ARRAY : V_VARIANT;
    if(!vba_internal_array_element_size((uint16_t)tag) || (constraint && constraint!=tag)) return 13;
    if(rank<1 || rank>60) return 9;
    VbaArrayBound shape[60];
    for(int i=0;i<rank;i++) {
        int64_t n=(int64_t)bounds[2*i+1]-bounds[2*i]+1;
        if(n<=0 || n>UINT32_MAX) return 9;
        shape[i]=(VbaArrayBound){(uint32_t)n,bounds[2*i]};
    }
    if((flags&1) && old) {
        if(old->dimensions!=rank) return 9;
        if(vba_internal_array_tag(old)!=tag) return 13;
        for(int i=0;i<rank;i++) {
            VbaArrayBound previous=old->bounds[rank-i-1];
            if(previous.lower!=shape[i].lower || (i!=rank-1 && previous.count!=shape[i].count)) return 9;
        }
    }
    VbaObjectArrayType object_constraint={0};
    if(tag==V_PRIVATE_OBJECT) {
        object_constraint=object_array_type(old);
        VbaVariant *view=declared?NULL:slot;
        if(view && view->vt==(V_BYREF|V_VARIANT))view=view->value.pointer;
        if(view && view->vt==(V_BYREF|V_ARRAY|V_PRIVATE_OBJECT)) {
            VbaObjectArrayType required={view->value.record.info,view->reserved1};
            if(object_type && !same_object_array_type(required,*object_type))return 13;
            object_constraint=required;
        } else if(!old && view && view->vt==(V_ARRAY|V_PRIVATE_OBJECT)) object_constraint=(VbaObjectArrayType){view->value.record.info,view->reserved1};
        if(object_type) {
            if(constraint==V_PRIVATE_OBJECT && old && !same_object_array_type(object_array_type(old),*object_type))return 13;
            if((flags&1) && old && !same_object_array_type(object_constraint,*object_type))return 13;
            object_constraint=*object_type;
        }
    }
    VbaArray *next=NULL;
    error=tag==V_PRIVATE_OBJECT?object_array_create(&next,&object_constraint,rank,shape):vba_internal_array_create(&next,(uint16_t)tag,rank,shape);
    if(error) return error;
    if((flags&1) && old) {
        size_t before=vba_internal_array_count(old), after=vba_internal_array_count(next);
        error=vba_internal_array_copy_elements(next,old,before<after ? before : after);
        if(error) { vba_array_destroy(next); return error; }
    }
    if(flags&2) next->features|=A_FIXED;
    *location=next;
    if(variant) {variant->vt=V_ARRAY|(uint16_t)tag;variant->value.record.info=tag==V_PRIVATE_OBJECT?(void*)object_constraint.type:NULL;variant->reserved1=tag==V_PRIVATE_OBJECT?(uint16_t)object_constraint.builtin:0;}
    vba_array_destroy(old);return 0;
}
int32_t vba_array_redim(void *slot,int32_t declared,int32_t requested,int32_t flags,int32_t rank,const int32_t *bounds) {
    if(requested==V_PRIVATE_OBJECT){VbaObjectArrayType type={0};return array_redim(slot,declared,requested,flags,rank,bounds,&type);}
    return array_redim(slot,declared,requested,flags,rank,bounds,NULL);
}
int32_t vba_object_array_redim(void *slot,int32_t declared,const VbaObjectArrayType *type,int32_t flags,int32_t rank,const int32_t *bounds) {
    if(!type)return 13;
    return array_redim(slot,declared,V_PRIVATE_OBJECT,flags,rank,bounds,type);
}
int32_t vba_array_erase(void *slot,int32_t declared) {
    VbaArray **location=NULL; VbaVariant *variant=NULL; int constraint=0;
    int error=vba_internal_array_location(slot,declared,&location,&variant,&constraint);
    if(error) return error;
    if(variant && !(variant->vt&V_ARRAY)) return 13;
    VbaArray *a=*location;
    if(a && (a->features&A_FIXED) && !variant) {
        return array_clear_data(a,1);
    }
    if(vba_array_check_unlocked(a))return 10;
    *location=NULL;vba_array_destroy(a);
    /* Erase returns an unallocated array of the same element type; it does not
       perform a Let-assignment of Empty to a real Variant container. */
    return 0;
}
int32_t vba_array_bound(int32_t *out,void *slot,int32_t declared,int32_t dimension,int32_t upper) {
    VbaArray **location=NULL; VbaVariant *variant=NULL; int constraint=0;
    int error=vba_internal_array_location(slot,declared,&location,&variant,&constraint);
    if(error) return error;
    VbaArray *a=*location;
    if(!a || dimension<1 || dimension>a->dimensions) return 9;
    VbaArrayBound bound=a->bounds[a->dimensions-dimension];
    int64_t value=(int64_t)bound.lower+(upper ? (int64_t)bound.count-1 : 0);
    if(value<INT32_MIN || value>INT32_MAX) return 6;
    *out=(int32_t)value;
    return 0;
}
int32_t vba_array_index(VbaElement *out,void *slot,int32_t declared,int32_t expected,int32_t rank,const int32_t *indices) {
    VbaArray **location=NULL; VbaVariant *variant=NULL; int constraint=0;
    int error=vba_internal_array_location(slot,declared,&location,&variant,&constraint);
    if(error) return error;
    VbaArray *a=*location;
    if(!a || rank!=a->dimensions) return 9;
    uint16_t tag=vba_internal_array_tag(a);
    if((!vba_internal_array_element_size(tag) && !vba_internal_array_record_type(a)) || (tag==36 && expected!=36) || (expected!=V_VARIANT && tag!=expected)) return 13;
    size_t offset=0,stride=1;
    for(int i=0;i<rank;i++) {
        VbaArrayBound bound=a->bounds[rank-i-1];
        int64_t index=(int64_t)indices[i]-bound.lower;
        if(index<0 || (uint64_t)index>=bound.count) return 9;
        offset+=(size_t)index*stride;
        stride*=bound.count;
    }
    error=vba_internal_array_lock(a);if(error)return error;
    VbaElement element={0};
    element.array=a;
    element.data=(unsigned char*)a->data+offset*a->element_size;
    element.reference.vt=V_BYREF|tag;
    element.reference.value.pointer=element.data;
    if(tag==V_PRIVATE_OBJECT){VbaObjectArrayType type=object_array_type(a);element.reference.value.record.info=(void*)type.type;element.reference.reserved1=(uint16_t)type.builtin;}
    element.string=(VbaString){.data=element.data,.length=-3};
    *out=element;
    return 0;
}
void *vba_element_address(VbaElement *element,int32_t tag) {
    if(tag==V_VARIANT) return vba_internal_array_tag(element->array)==V_VARIANT ? element->data : &element->reference;
    if(tag==V_BSTR) return &element->string;
    return element->data;
}
void vba_element_clear(VbaElement *element) {
	if(!element->array)vba_variant_clear(&element->reference);
    if(element->array) {
        vba_internal_array_unlock(element->array);
    }
    memset(element,0,sizeof(*element));
}

/* A ParamArray owns its value arguments, but borrows variable arguments.
   Flatten Variant binding wrappers when forwarding a ParamArray element to
   another ParamArray; Automation forbids a BYREF|VARIANT chain. */
int32_t vba_paramarray_pack(VbaArray **out,int32_t count,const VbaParamArgument *args) {
    if(count<0 || (count && !args)) return 5;
    VbaArray *next=NULL;
    VbaArrayBound bound={(uint32_t)count,0};
    int error=vba_internal_array_create(&next,V_VARIANT,1,&bound);
    if(error) return error;
    VbaVariant *elements=next->data;
    for(int32_t i=0;i<count;i++) {
        const VbaVariant *value=args[i].value;
        if(!value || (args[i].byref!=0 && args[i].byref!=1)) {error=13;break;}
        if(!args[i].byref) error=vba_variant_load(elements+i,value);
        else {
            /* References emitted by this compiler already have at most one
               wrapper. The bound also rejects malformed foreign binding cycles. */
            int depth=0;
            while(value && value->vt==(V_BYREF|V_VARIANT) && depth++<64) value=value->value.pointer;
            if(!value || value->vt==(V_BYREF|V_VARIANT) || !vba_internal_supported(value->vt)) {error=13;break;}
            if(value->vt&V_BYREF) elements[i]=*value;
            else {elements[i].vt=V_BYREF|V_VARIANT;elements[i].value.pointer=(void*)value;}
        }
        if(error) break;
    }
    if(!error) error=vba_array_destroy(*out);
    if(error) {vba_array_destroy(next);return error;}
    *out=next;return 0;
}
