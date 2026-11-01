#include "runtime_internal.h"
#include <stdlib.h>
#include <string.h>
#ifdef _WIN32
#include <windows.h>
#include <oleauto.h>
#endif

/* Shared native invocation contract. Arguments are evaluated by generated code
   before binding; typed BYREF views retain their storage and class constraints. */
static _Thread_local void *active_invocation_error;
void *vba_invocation_error_push(void *error){void *previous=active_invocation_error;active_invocation_error=error;return previous;}
void vba_invocation_error_pop(void *previous){active_invocation_error=previous;}
static int invoke_name(const char *a,const char *b) {
    if(!a || !b)return a==b;
    for(;*a && *b;a++,b++) {
        unsigned char x=(unsigned char)*a,y=(unsigned char)*b;
        if(x>='A'&&x<='Z')x+=32;if(y>='A'&&y<='Z')y+=32;
        if(x!=y)return 0;
    }
    return *a==*b;
}
void vba_capture_clear(VbaVariant *capture) {
    VbaArray *array=capture->value.pointer;memset(capture,0,sizeof(*capture));
    if(array) {
        vba_internal_array_unlock(array);
    }
}
int32_t vba_invocation_capture(VbaVariant *out,const VbaVariant *source) {
    VbaVariant value={0};int status=vba_internal_borrow(&value,source);if(status)return status;
    if(!(value.vt&V_ARRAY))return vba_variant_load(out,source);
    VbaArray *array=value.value.pointer;
    if(array) {
        status=vba_internal_array_lock(array);if(status)return status;
    }
    *out=(VbaVariant){.vt=V_PRIVATE_ARRAY_CAPTURE,.value.pointer=array};return 0;
}
int32_t vba_capture_index(void *out,const VbaVariant *capture,int32_t rank,const int32_t *indices) {
    VbaElement *element=out;
    if(capture->vt==V_PRIVATE_ARRAY_CAPTURE) {
        VbaArray *array=capture->value.pointer;
        if(!array)return 9;
        return vba_array_index(element,&array,vba_internal_array_tag(array),V_VARIANT,rank,indices);
    }
    if(rank<0 || rank>60)return 9;
    VbaInvokeArg args[60];memset(args,0,sizeof(args));
    for(int i=0;i<rank;i++)args[i].value=(VbaVariant){.vt=V_I4,.value.i4=indices[i]};
    int32_t error[4]={0};
    return vba_object_invoke(error,&element->reference,capture,"",I_GET|I_METHOD,rank,args);
}
void vba_invoke_argument(VbaInvokeArg *out,const char *name,const VbaVariant *value,int32_t byref) {
    out->name=name;
    if(byref && !(value->vt&V_BYREF)) out->value=(VbaVariant){.vt=V_BYREF|V_VARIANT,.value.pointer=(void*)value};
    else out->value=*value;
}
int32_t vba_native_from(void *out,const VbaVariant *value,int32_t tag,const VbaObjectType *type,int32_t builtin) {
    if(tag==V_VARIANT)return vba_variant_load(out,value);
    if(tag==V_PRIVATE_OBJECT) {
        VbaObject *object=NULL;int error=vba_variant_object(&object,value);
        if(!error && type)error=vba_class_check(object,type,1);
        if(!error && builtin)error=vba_object_check(object,builtin);
        if(error){vba_object_clear(&object);return error;}
        *(VbaObject**)out=object;return 0;
    }
    if(tag==(V_ARRAY|V_PRIVATE_OBJECT)){VbaObjectArrayType constraint={type,builtin};return vba_object_array_unbox(out,value,&constraint);}
    if(tag&V_ARRAY)return vba_array_unbox(out,value,tag&~V_ARRAY);
    return vba_variant_to(out,value,tag);
}
int32_t vba_native_box(VbaVariant *out,const void *native,int32_t tag) {
    if(tag==V_VARIANT)return vba_variant_load(out,native);
    if(tag==V_PRIVATE_OBJECT)return vba_variant_box_object(out,*(VbaObject*const*)native);
    if(tag==V_BSTR)return vba_variant_box_string(out,native);
    if(tag&V_ARRAY)return vba_array_box(out,*(VbaArray*const*)native,tag&~V_ARRAY);
    if(tag==V_R4)return vba_variant_box_float(out,tag,*(const float*)native);
    if(tag==V_R8 || tag==V_DATE)return vba_variant_box_float(out,tag,*(const double*)native);
    if(tag==V_CY)return vba_variant_box_number(out,tag,*(const int64_t*)native);
    if(tag==V_I2 || tag==V_BOOL)return vba_variant_box_number(out,tag,*(const int16_t*)native);
    if(tag==V_UI1)return vba_variant_box_number(out,tag,*(const uint8_t*)native);
    if(tag==V_I4)return vba_variant_box_number(out,tag,*(const int32_t*)native);
    return 13;
}
int32_t vba_field_invoke(void *error,VbaVariant *out,void *field,int32_t tag,const VbaObjectType *type,int32_t builtin,int32_t flags,VbaArray *arguments,const VbaVariant *rhs,const VbaObjectType *auto_type,int32_t auto_builtin) {
    size_t count=arguments?vba_internal_array_count(arguments):0;
    if(!count && (tag&V_ARRAY) && flags==I_SET)return 13;
    if(count) {
        if(!(tag&V_ARRAY) || count>60)return 450;
        int32_t indices[60];VbaVariant *values=arguments->data;
        for(size_t i=0;i<count;i++){int status=vba_variant_to(indices+i,values+i,V_I4);if(status)return status;}
        VbaElement element={0};int status=vba_array_index(&element,field,tag&~V_ARRAY,V_VARIANT,(int32_t)count,indices);
        if(status)return status;
        VbaVariant *value=vba_element_address(&element,V_VARIANT);
        status=flags==I_GET?vba_variant_load(out,value):vba_variant_store(value,rhs);
        vba_element_clear(&element);return status;
    }
    if(flags==I_GET) {
        if(auto_type || auto_builtin) {
            VbaObject *object=NULL;
            int status=auto_type?vba_class_load(&object,field,auto_type,error):vba_object_load(&object,field,auto_builtin);
            if(!status)status=vba_variant_box_object(out,object);
            vba_object_clear(&object);return status;
        }
        if(tag==(V_ARRAY|V_PRIVATE_OBJECT)){VbaObjectArrayType constraint={type,builtin};return vba_object_array_box(out,*(VbaArray**)field,&constraint);}
        return vba_native_box(out,field,tag);
    }
    uint64_t value[3]={0};int status=vba_native_from(value,rhs,tag,type,builtin);
    if(status)return status;
    if(tag==V_PRIVATE_OBJECT)status=vba_object_store(field,*(VbaObject**)value);
    else if(tag==V_VARIANT)status=vba_variant_store(field,(VbaVariant*)value);
    else if(tag==V_BSTR)status=vba_string_store(field,(VbaString*)value);
    else if(tag&V_ARRAY)status=vba_array_store(field,*(VbaArray**)value);
    else if(tag==V_UI1)memcpy(field,value,1);
    else if(tag==V_I2||tag==V_BOOL)memcpy(field,value,2);
    else if(tag==V_I4||tag==V_R4)memcpy(field,value,4);
    else if(tag==V_R8 || tag==V_DATE || tag==V_CY)memcpy(field,value,8);
    else status=13;
    vba_native_clear(value,tag);return status;
}
void vba_native_clear(void *native,int32_t tag) {
    if(tag==V_VARIANT)vba_variant_clear(native);
    else if(tag==V_PRIVATE_OBJECT)vba_object_clear(native);
    else if(tag==V_BSTR){VbaString old=*(VbaString*)native;memset(native,0,sizeof(old));vba_internal_string_drop(old);}
    else if(tag&V_ARRAY)vba_array_clear(native);
}
static int invoke_missing(const VbaVariant *source) {
    VbaVariant v={0};return !vba_internal_borrow(&v,source) && v.vt==V_ERROR && (uint32_t)v.value.error==UINT32_C(0x80020004);
}
static int invoke_bind_one(void *error,VbaBinding *b,const VbaInvokeParameter *p,const VbaInvokeArg *actual) {
    b->actual=actual;
    b->address=p->tag==V_VARIANT ? (void*)&b->value : b->native;
    if(!actual || invoke_missing(&actual->value)) {
        if(!(p->flags&P_OPTIONAL))return 450;
        b->owns=1;
        if(p->fallback)return p->fallback(error,b->address);
        if(p->tag==V_VARIANT)b->value=(VbaVariant){.vt=V_ERROR,.value.error=(int32_t)0x80020004};
        else if(p->tag==V_BSTR)((VbaString*)b->native)->length=-1;
        return 0;
    }
    const VbaVariant *v=&actual->value;
    if((p->flags&P_BYREF) && (v->vt&V_BYREF)) {
        if(p->tag==V_VARIANT){b->value=*v;return 0;}
        if((v->vt&~V_BYREF)==p->tag) {
            if(p->tag==V_BSTR){*(VbaString*)b->native=(VbaString){.data=v->value.pointer,.length=-3};return 0;}
            if(p->tag==(V_ARRAY|V_PRIVATE_OBJECT)) {
                VbaObjectArrayType constraint={p->type,p->builtin};
                int e=vba_internal_object_array_check(v,&constraint);if(e)return e;
                b->address=v->value.pointer;return 0;
            }
            if(p->tag!=V_PRIVATE_OBJECT || (v->value.record.info==p->type && v->reserved1==p->builtin)) {
                b->address=v->value.pointer;return 0;
            }
            b->copyback=1; /* Object storage adaptation, only on normal return. */
        } else return 13;
    }
    b->owns=1;
    return vba_native_from(b->address,v,p->tag,p->type,p->builtin);
}
int32_t vba_invoke_members(void *error,VbaVariant *out,VbaObject *object,const char *name,int32_t flags,int32_t count,const VbaInvokeArg *args,int32_t member_count,const VbaMember *members) {
    const VbaMember *member=NULL;
    for(int32_t i=0;i<member_count;i++) {
        if(((*name && invoke_name(name,members[i].name)) || (!*name && members[i].is_default)) && (members[i].flags&flags&15)) {member=members+i;break;}
    }
    if(!member){
        if(flags&I_DISCARD)for(int i=0;i<member_count;i++)if(((*name&&invoke_name(name,members[i].name))||(!*name&&members[i].is_default))&&(members[i].flags&I_GET))return 450;
        return 438;
    }
    if((member->flags&I_DISCARD) && !(flags&I_DISCARD))return 450;
    int32_t n=member->count,status=0,position=0,paramarray=-1;
    VbaBinding *bindings=n?malloc((size_t)n*sizeof(*bindings)):NULL;
    if(n && !bindings)return 7;
    if(n)memset(bindings,0,(size_t)n*sizeof(*bindings));
    for(int32_t i=0;i<n;i++)if(member->parameters[i].flags&P_PARAMARRAY)paramarray=i;
    VbaParamArgument *packed=count?malloc((size_t)count*sizeof(*packed)):NULL;
    if(count && !packed){free(bindings);return 7;}
    int packed_count=0,setter=(member->flags&(I_LET|I_SET))!=0;
    for(int32_t i=0;i<count && !status;i++) {
        int32_t index=-1;
        if(args[i].name && args[i].name[0]==1)index=n-1;
        else if(args[i].name && *args[i].name) {
            for(int32_t j=0;j<n-setter;j++)if(invoke_name(args[i].name,member->parameters[j].name)){index=j;break;}
            if(index<0 || index==paramarray){status=448;break;}
        } else index=position++;
        if(paramarray>=0 && index>=paramarray && !(setter && args[i].name && args[i].name[0]==1)) {
            packed[packed_count++]=(VbaParamArgument){&args[i].value,(args[i].value.vt&V_BYREF)!=0};continue;
        }
        if(index<0 || index>=n || (setter && index==n-1 && (!args[i].name || args[i].name[0]!=1))){status=450;break;}
        if(bindings[index].actual){status=450;break;}
        status=invoke_bind_one(error,bindings+index,member->parameters+index,args+i);
    }
    for(int32_t i=0;i<n && !status;i++) {
        if(i==paramarray) {
            bindings[i].address=bindings[i].native;bindings[i].owns=1;
            status=vba_paramarray_pack((VbaArray**)bindings[i].native,packed_count,packed);
        } else if(!bindings[i].actual)status=invoke_bind_one(error,bindings+i,member->parameters+i,NULL);
    }
    if(!status)status=member->call(error,out,object,bindings);
    if(!status)for(int32_t i=0;i<n && !status;i++)if(bindings[i].copyback) {
        VbaVariant value={0},destination=bindings[i].actual->value;
        status=vba_native_box(&value,bindings[i].address,member->parameters[i].tag);
        if(!status)status=vba_variant_store(&destination,&value);
        vba_variant_clear(&value);
    }
    for(int32_t i=n;i>0;i--)if(bindings[i-1].owns)vba_native_clear(bindings[i-1].address,member->parameters[i-1].tag);
    free(packed);free(bindings);
    if(status)vba_variant_clear(out);
    return status;
}
static int32_t object_invoke(void *error,VbaVariant *out,const VbaVariant *receiver,const char *name,int32_t flags,int32_t count,const VbaInvokeArg *args) {
    if(receiver->vt==V_PRIVATE_ARRAY_CAPTURE) {
        if(*name)return 424;
        int setter=(flags&(I_LET|I_SET))!=0,rank=count-setter;
        if(rank<1 || rank>60)return 9;
        int32_t indices[60];
        for(int i=0;i<rank;i++) {if(args[i].name && *args[i].name)return 448;int status=vba_variant_to(indices+i,&args[i].value,V_I4);if(status)return status;}
        VbaElement element={0};int status=vba_capture_index(&element,receiver,rank,indices);if(status)return status;
        VbaVariant *value=vba_element_address(&element,V_VARIANT);
        status=setter?vba_variant_store(value,&args[count-1].value):vba_variant_load(out,value);
        vba_element_clear(&element);return status;
    }
    VbaVariant value={0};int status=vba_internal_borrow(&value,receiver);if(status)return status;
    if(value.vt!=V_PRIVATE_OBJECT)return 424;
    VbaObject *object=value.value.pointer;
    if(!object)return 91;
    if(!object->type->invoke)return 438;
    vba_object_retain(object);
    status=object->type->invoke(error,out,object,name,flags,count,args);
    vba_object_clear(&object);return status;
}
int32_t vba_object_invoke(void *error,VbaVariant *out,const VbaVariant *receiver,const char *name,int32_t flags,int32_t count,const VbaInvokeArg *args) {
    void *previous=active_invocation_error;active_invocation_error=error;
    int32_t status=object_invoke(error,out,receiver,name,flags,count,args);
    active_invocation_error=previous;return status;
}
int32_t vba_object_default(VbaVariant *out,void *object) {
    static int depth;
    if(depth>=64)return 28;
    int32_t error[4]={0};VbaVariant receiver={.vt=V_PRIVATE_OBJECT,.value.pointer=object};
    depth++;int status=vba_object_invoke(active_invocation_error?active_invocation_error:error,out,&receiver,"",I_GET|I_METHOD,0,NULL);depth--;
    return status;
}
int32_t vba_object_value_at(VbaVariant *out,const VbaVariant *source,void *error) {
    void *previous=active_invocation_error;active_invocation_error=error;
    int32_t status=vba_object_value(out,source);active_invocation_error=previous;return status;
}
int32_t vba_variant_to_at(void *out,const VbaVariant *source,int32_t tag,void *error) {
    void *previous=active_invocation_error;active_invocation_error=error;
    int32_t status=vba_variant_to(out,source,tag);active_invocation_error=previous;return status;
}
int32_t vba_object_value(VbaVariant *out,const VbaVariant *source) {
    static int depth;
    VbaVariant value={0};int status=vba_internal_borrow(&value,source);if(status)return status;
    if(value.vt!=V_PRIVATE_OBJECT)return vba_variant_load(out,source);
    if(depth>=64)return 28;
    depth++;VbaVariant result={0};status=vba_object_default(&result,value.value.pointer);
    if(!status)status=vba_object_value(out,&result);
    vba_variant_clear(&result);depth--;return status;
}
int32_t vba_builtin_invoke(void *error,VbaVariant *out,VbaObject *object,const char *name,int32_t flags,int32_t count,const VbaInvokeArg *args) {
    int method=0;
    if(object->kind==O_COLLECTION) {
        method=!*name||invoke_name(name,"Item")?2:invoke_name(name,"Add")?1:invoke_name(name,"Remove")?3:invoke_name(name,"Count")?4:0;
        if(!method)return 438;
        if(!(flags&((method==2||method==4)?I_GET:I_METHOD)))return (flags&I_DISCARD) && (method==2||method==4)?450:438;
        if((method==1||method==3) && !(flags&I_DISCARD))return 450;
    } else if(object->kind==O_ASSERT || object->kind==O_PERMISSIVE_ASSERT) {
        const char *names[]={"AreEqual","AreNotEqual","AreSame","AreNotSame","IsTrue","IsFalse","IsNothing","IsNotNothing","SequenceEquals","NotSequenceEquals","Fail","Inconclusive","Succeed"};
        if(!(flags&I_METHOD))return 438;
        if(!(flags&I_DISCARD))return 450;
        for(int i=0;i<13;i++)if(invoke_name(name,names[i]))method=i+1;
        if(!method)return 438;
    } else return 438;
    VbaObjectArg *native=count?malloc((size_t)count*sizeof(*native)):NULL;
    VbaVariant *values=count?malloc((size_t)count*sizeof(*values)):NULL;
    if(count && (!native||!values)){free(native);free(values);return 7;}
    int status=0;
    for(int i=0;i<count;i++) {
        status=vba_internal_borrow(values+i,&args[i].value);if(status)break;
        native[i]=(VbaObjectArg){args[i].name,values[i].vt==V_PRIVATE_OBJECT,values[i].vt==V_PRIVATE_OBJECT?(const void*)&values[i].value.pointer:(const void*)&args[i].value};
    }
    if(!status && object->kind==O_COLLECTION) {
        if(method==4){int32_t value=0;status=vba_collection_invoke(&value,object,method,count,native,3);if(!status)status=vba_variant_box_number(out,V_I4,value);}
        else status=vba_collection_invoke(out,object,method,count,native,method==2?12:0);
    } else if(!status) {
        int32_t *origin=error;
        status=vba_test_assert(object,method,count,native,origin?origin[1]:0,origin?origin[2]:0,origin?origin[3]:0);
    }
    free(values);free(native);return status;
}
