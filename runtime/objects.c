#include "objects.h"
#include "runtime_internal.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#include "collection.h"
#include "records.h"
static const VbaObjectType simple_object_type={.size=sizeof(VbaObject),.invoke=vba_builtin_invoke};
int32_t vba_object_allocate(VbaObject **out,const VbaObjectType *type,int32_t kind) {
    if(!type || type->size<sizeof(VbaObject) ||
       (type->fields && type->fields->size>type->size-sizeof(VbaObject))) return 5;
    VbaObject *o=malloc((size_t)type->size);
    if(!o) return 7;
    memset(o,0,(size_t)type->size);
    *o=(VbaObject){.references=1,.kind=kind,.type=type,.flags=1};
    int32_t error=type->fields ? vba_record_init(o+1,type->fields) : 0;
    if(error) {vba_object_clear(&o);return error;}
    o->flags=0;
    *out=o;
    return 0;
}
int32_t vba_class_construct(VbaObject **out,const VbaObjectType *type,void *error) {
    VbaObject *object=NULL;
    int32_t status=vba_object_allocate(&object,type,0);
    if(status) return status;
    if(type->initialize) {
        void *previous=vba_invocation_error_push(error);
        status=type->initialize(error,object);vba_invocation_error_pop(previous);
    }
    if(status) {vba_object_clear(&object);return status;}
    *out=object;
    return 0;
}
int32_t vba_class_new(VbaObject **out,const VbaObjectType *type) {
    int32_t error[4]={0};return vba_class_construct(out,type,error);
}
int32_t vba_class_load(VbaObject **out,VbaObject **slot,const VbaObjectType *type,void *error) {
    if(!*slot) {
        VbaObject *object=NULL;int32_t status=vba_class_construct(&object,type,error);
        if(status) return status;
        /* Initialization may reenter and assign the lazy variable itself. */
        VbaObject *old=*slot;*slot=object;vba_object_clear(&old);
    }
    vba_object_retain(*slot);*out=*slot;return 0;
}
int32_t vba_class_check_exact(VbaObject *o,const VbaObjectType *type) {
    if(!o)return 91;
    return o->type==type ? 0 : 13;
}
int32_t vba_class_check(VbaObject *o,const VbaObjectType *type,int32_t nullable) {
    if(!o) return nullable ? 0 : 91;
    if(o->type==type)return 0;
    for(uint64_t i=0;i<o->type->interface_count;i++)if(o->type->interfaces[i].type==type)return 0;
    return 13;
}
int32_t vba_interface_resolve(VbaInterfaceMethod *out,VbaObject *o,const VbaObjectType *type,uint32_t member,VbaInterfaceMethod fallback) {
    if(!o)return 91;
    if(o->type==type){*out=fallback;return 0;}
    for(uint64_t i=0;i<o->type->interface_count;i++) {
        const VbaInterface *iface=o->type->interfaces+i;
        if(iface->type!=type)continue;
        for(uint64_t j=0;j<iface->count;j++)if(iface->methods[j].member==member){*out=iface->methods[j].call;return 0;}
        return 13;
    }
    return 13;
}
int32_t vba_object_check(VbaObject *o,int32_t kind){return kind&&o&&o->kind!=kind?13:0;}
int32_t vba_object_new(VbaObject **out,int32_t kind) {
 if(kind==O_COLLECTION)return vba_collection_new(out);
 if(kind<1 || kind>O_FAKES)return 429;
 return vba_object_allocate(out,&simple_object_type,kind);
}
void vba_object_retain(VbaObject *o){if(o)o->references++;}
void vba_object_clear(VbaObject **slot) {
    VbaObject *o=*slot;
    *slot=NULL;
    if(o && !--o->references) {
        if(!(o->flags&3) && o->type->terminate) {
            /* The protective reference permits Me to escape from termination.
               Set the once flag before calling code that can reenter release. */
            o->flags|=2;o->references=1;
            VbaErrState saved=vba_internal_err_save();
            int32_t error[4]={0};void *previous=vba_invocation_error_push(error);
            int32_t status=o->type->terminate(error,o);vba_invocation_error_pop(previous);
            vba_internal_err_restore(saved);
            if(status) {fprintf(stderr,"Unhandled Class_Terminate error %d\n",status);exit(status>0?status:1);}
            if(--o->references) return;
        }
        if(o->type->dispose) o->type->dispose(o);
        if(o->type->fields) vba_record_clear(o+1,o->type->fields);
        free(o);
    }
}
int32_t vba_object_store(VbaObject **slot,VbaObject *o){vba_object_retain(o);VbaObject *old=*slot;*slot=o;vba_object_clear(&old);return 0;}
int32_t vba_object_load(VbaObject **out,VbaObject **slot,int32_t auto_kind){if(!*slot && auto_kind){int e=vba_object_new(slot,auto_kind);if(e)return e;}vba_object_retain(*slot);*out=*slot;return 0;}
static int test_progid(const VbaString *s,const char *name){int n=vba_internal_slen(s);if(n!=(int)strlen(name))return 0;for(int i=0;i<n;i++){uint16_t c=vba_internal_unit(s,i);if(c>='A'&&c<='Z')c+=32;char d=name[i];if(d>='A'&&d<='Z')d+=32;if(c!=(uint16_t)d)return 0;}return 1;}
int32_t vba_create_object(VbaObject **out,const VbaString *name){int kind=test_progid(name,"Rubberduck.AssertClass")?1:test_progid(name,"Rubberduck.PermissiveAssertClass")?2:test_progid(name,"Rubberduck.FakesProvider")?3:0;return vba_object_new(out,kind);}

