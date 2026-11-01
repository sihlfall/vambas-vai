#include "variant.h"
#include "runtime_internal.h"
#include "arrays.h"
#include "control.h"
#include "records.h"
#include "builtins.h"
#include "conversion.h"
#include "errors.h"
#include "collection.h"
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <stdio.h>
#include <errno.h>
#include <limits.h>
#include <float.h>
_Static_assert(sizeof(void*)==8, "the LLVM backend currently supports 64-bit Variant targets only");
#ifdef _WIN32
#include <windows.h>
#include <oleauto.h>
_Static_assert(sizeof(VbaVariant) == sizeof(VARIANT), "Windows SDK VARIANT size");
_Static_assert(_Alignof(VbaVariant) == _Alignof(VARIANT), "Windows SDK alignment");
_Static_assert(offsetof(VbaVariant, vt) == offsetof(VARIANT, vt), "Windows SDK vt");
_Static_assert(offsetof(VbaVariant, value) == offsetof(VARIANT, lVal), "Windows SDK payload");
_Static_assert(sizeof(((VbaVariant*)0)->value.i8) == sizeof(CY), "Windows SDK Currency storage");
_Static_assert(sizeof(((VbaVariant*)0)->value.r8) == sizeof(DATE), "Windows SDK Date storage");
_Static_assert(sizeof(((VbaVariant*)0)->decimal) == sizeof(DECIMAL), "Windows SDK Decimal storage");
_Static_assert(offsetof(VbaVariant, decimal.scale) == offsetof(DECIMAL, scale), "Windows SDK Decimal scale");
_Static_assert(offsetof(VbaVariant, decimal.sign) == offsetof(DECIMAL, sign), "Windows SDK Decimal sign");
_Static_assert(offsetof(VbaVariant, decimal.hi) == offsetof(DECIMAL, Hi32), "Windows SDK Decimal high bits");
_Static_assert(offsetof(VbaVariant, decimal.lo) == offsetof(DECIMAL, Lo64), "Windows SDK Decimal low bits");
_Static_assert(sizeof(VbaArray)==sizeof(SAFEARRAY), "Windows SDK SAFEARRAY size");
_Static_assert(_Alignof(VbaArray)==_Alignof(SAFEARRAY), "Windows SDK SAFEARRAY alignment");
_Static_assert(offsetof(VbaArray,data)==offsetof(SAFEARRAY,pvData), "Windows SDK SAFEARRAY data");
_Static_assert(offsetof(VbaArray,bounds)==offsetof(SAFEARRAY,rgsabound), "Windows SDK SAFEARRAY bounds");
#endif

uint32_t vba_internal_bstr_len(const uint16_t *s) {
    uint32_t bytes=0;
    if (s) memcpy(&bytes, (const unsigned char*)s-4, 4);
    return bytes/2;
}
uint16_t *vba_internal_bstr_alloc(const uint16_t *s, uint32_t n) {
#ifdef _WIN32
    return (uint16_t*)SysAllocStringLen((const OLECHAR*)s,n);
#else
    if (n > INT32_MAX) return NULL;
    unsigned char *p=malloc((size_t)n*2+6);
    if (!p) return NULL;
    uint32_t bytes=n*2;
    memcpy(p,&bytes,4);
    uint16_t *out=(uint16_t*)(p+4);
    if (s && n) memcpy(out,s,(size_t)n*2);
    out[n]=0;
    return out;
#endif
}
void vba_bstr_free(void *s) {
#ifdef _WIN32
    SysFreeString((BSTR)s);
#else
    if (s) free((unsigned char*)s-4);
#endif
}
int vba_internal_supported(uint16_t vt) {
    if (vt&V_ARRAY) {
        switch(vt & ~(V_ARRAY|V_BYREF)) {
        case V_UI1: case V_I2: case V_I4: case V_R4: case V_R8: case V_CY: case V_DATE: case V_BOOL: case V_BSTR: case V_VARIANT: case V_PRIVATE_OBJECT: return 1;
        default: return 0;
        }
    }
    switch (vt & ~V_BYREF) {
    case V_EMPTY: case V_NULL: case V_I2: case V_I4: case V_R4: case V_R8:
    case V_CY: case V_DATE: case V_DECIMAL: case V_BSTR: case V_ERROR: case V_BOOL: case V_UI1: case V_PRIVATE_OBJECT: return 1;
    case V_VARIANT: return vt == (V_VARIANT|V_BYREF);
    default: return 0;
    }
}

/* Borrow a scalar from an ABI reference; never leave a BYREF chain in an owned
   value. VT_BYREF|VT_VARIANT cannot itself point to another such wrapper. */
int vba_internal_borrow(VbaVariant *out, const VbaVariant *v) {
    if (!vba_internal_supported(v->vt)) return 13;
    if (v->vt == (V_VARIANT|V_BYREF)) {
        v=(const VbaVariant*)v->value.pointer;
        if (!v || v->vt == (V_VARIANT|V_BYREF)) return 13;
        return vba_internal_borrow(out,v);
    }
    *out=*v;
    if (!(v->vt&V_BYREF)) return v->vt==V_DECIMAL && (v->decimal.scale>28 || (v->decimal.sign!=0 && v->decimal.sign!=0x80)) ? 13 : 0;
    if (!v->value.pointer) return 13;
    memset(out,0,sizeof(*out));
    out->vt=v->vt & ~V_BYREF;
    if (out->vt&V_ARRAY) {
        memcpy(&out->value.pointer,v->value.pointer,sizeof(void*));
        out->value.record.info=v->value.record.info;out->reserved1=v->reserved1;
        return 0;
    }
    switch (out->vt) {
    case V_I2: case V_BOOL: memcpy(&out->value.i2,v->value.pointer,2); break;
    case V_I4: case V_ERROR: case V_R4: memcpy(&out->value.i4,v->value.pointer,4); break;
    case V_UI1: memcpy(&out->value.byte,v->value.pointer,1); break;
    case V_R8: case V_DATE: case V_CY: memcpy(&out->value.r8,v->value.pointer,8); break;
    case V_DECIMAL:
        memcpy(&out->decimal,v->value.pointer,16);out->vt=V_DECIMAL;
        if(out->decimal.scale>28 || (out->decimal.sign!=0 && out->decimal.sign!=0x80))return 13;
        break;
    case V_BSTR: case V_PRIVATE_OBJECT: memcpy(&out->value.pointer,v->value.pointer,sizeof(void*)); break;
    default: return 13;
    }
    return 0;
}
void vba_variant_clear(VbaVariant *v) {
    if(v->vt==V_PRIVATE_ARRAY_CAPTURE) {vba_capture_clear(v);return;}
    if(v->vt==V_PRIVATE_OBJECT) {
        VbaObject *object=v->value.pointer;memset(v,0,sizeof(*v));
        vba_object_clear(&object);return;
    }
    if ((v->vt&V_ARRAY) && !(v->vt&V_BYREF)) {
        VbaArray *array=v->value.pointer;
        if(vba_array_check_unlocked(array))return;
        memset(v,0,sizeof(*v));vba_array_destroy(array);return;
    }
    if (v->vt == V_BSTR) vba_bstr_free(v->value.pointer);
    memset(v,0,sizeof(*v));
}
int32_t vba_variant_load(VbaVariant *out, const VbaVariant *source) {
    VbaVariant value;
    int error=vba_internal_borrow(&value,source);
    if (error) return error;
    if (value.vt&V_ARRAY) {
        VbaArray *copy=NULL;
        error=vba_array_copy(&copy,value.value.pointer);
        if(error) return error;
        value.value.pointer=copy;
    }
    if (value.vt==V_BSTR && value.value.pointer) {
        value.value.pointer=vba_internal_bstr_alloc(value.value.pointer,vba_internal_bstr_len(value.value.pointer));
        if (!value.value.pointer) return 14;
    }
    if(value.vt==V_PRIVATE_OBJECT) vba_object_retain(value.value.pointer);
    *out=value;
    return 0;
}
int32_t vba_variant_box_object(VbaVariant *out,void *object) {
    vba_object_retain(object);
    *out=(VbaVariant){.vt=V_PRIVATE_OBJECT,.value.pointer=object};return 0;
}
int32_t vba_variant_object(void *out,const VbaVariant *source) {
    VbaVariant value={0};int error=vba_internal_borrow(&value,source);
    if(error) return error;
    if(value.vt!=V_PRIVATE_OBJECT) return 424;
    vba_object_retain(value.value.pointer);memcpy(out,&value.value.pointer,sizeof(void*));return 0;
}
static uint16_t *string_data(const VbaString *s) {
    if(s->length==-3) { uint16_t *p=NULL; memcpy(&p,s->data,sizeof(p)); return p; }
    return s->data;
}
int vba_internal_slen(const VbaString *s) { return s->length<=-2 ? (int)vba_internal_bstr_len(string_data(s)) : s->length<0 ? 0 : s->length; }
uint16_t vba_internal_unit(const VbaString *s, int i) { return s->length==1 ? s->unit : string_data(s)[i]; }
void vba_internal_string_drop(VbaString s) {
    if (s.length==-3) return;
    if (s.length==-2) { vba_bstr_free(s.data); return; }
    if (s.owner && --*(uint64_t*)s.owner==0) free(s.owner);
}
void vba_internal_string_retain(VbaString s) { if (s.owner) ++*(uint64_t*)s.owner; }
static int string_bstr(uint16_t **out, const VbaString *s) {
    if (s->length==-1 || (s->length<=-2 && !string_data(s))) { *out=NULL; return 0; }
    int n=vba_internal_slen(s);
    uint16_t *b=vba_internal_bstr_alloc(NULL,(uint32_t)n);
    if (!b) return 14;
    for (int i=0;i<n;i++) b[i]=vba_internal_unit(s,i);
    *out=b;
    return 0;
}
static int string_from_bstr(VbaString *out, const uint16_t *b) {
    VbaString s={0};
    if (!b) { s.length=-1; *out=s; return 0; }
    uint32_t n=vba_internal_bstr_len(b);
    if (n>INT32_MAX) return 14;
    s.length=(int32_t)n;
    if (n==1) s.unit=*b;
    else if (n>1) {
        uint64_t *owner=malloc(8+((size_t)n+1)*2);
        if (!owner) return 14;
        *owner=1;
        s.owner=owner; s.data=(uint16_t*)(owner+1);
        memcpy(s.data,b,((size_t)n+1)*2);
    }
    *out=s;
    return 0;
}
int32_t vba_string_load(VbaString *out, const VbaString *s) {
    if (s->length<=-2) return string_from_bstr(out,string_data(s));
    VbaString v=*s;
    vba_internal_string_retain(v); *out=v;
    return 0;
}
int32_t vba_string_store(VbaString *destination, const VbaString *source) {
    VbaString old=*destination;
    if (old.length<=-2) {
        uint16_t *b=NULL;
        int error=string_bstr(&b,source);
        if (error) return error;
        if (old.length==-3) {
            uint16_t *previous=string_data(&old);
            memcpy(old.data,&b,sizeof(b));
            vba_bstr_free(previous);
        } else destination->data=b;
    } else {
        VbaString value={0};
        int error=vba_string_load(&value,source);
        if (error) return error;
        *destination=value;
    }
    vba_internal_string_drop(old);
    return 0;
}
int32_t vba_string_pin(VbaString *slot) {
    if (slot->length<=-2) return 0;
    uint16_t *b=NULL;
    int error=string_bstr(&b,slot);
    if (error) return error;
    VbaString old=*slot;
    *slot=(VbaString){.data=b,.length=-2};
    vba_internal_string_drop(old);
    return 0;
}
void *vba_string_ref(VbaString *slot) { return slot->length==-3 ? (void*)slot->data : (void*)&slot->data; }
int32_t vba_variant_box_number(VbaVariant *out, int32_t tag, int64_t n) {
    VbaVariant v={0}; v.vt=(uint16_t)tag;
    if (tag==V_UI1) v.value.byte=(uint8_t)n;
    else if (tag==V_I2 || tag==V_BOOL) v.value.i2=(int16_t)n;
    else if (tag==V_I4) v.value.i4=(int32_t)n;
    else if (tag==V_CY) v.value.i8=n;
    else return 13;
    *out=v; return 0;
}
int32_t vba_variant_box_string(VbaVariant *out, const VbaString *s) {
    VbaVariant v={0}; v.vt=V_BSTR;
    int error=string_bstr((uint16_t**)&v.value.pointer,s);
    if (!error) *out=v;
    return error;
}
int32_t vba_variant_box_float(VbaVariant *out,int32_t tag,double n) {
    VbaVariant v={0}; v.vt=(uint16_t)tag;
    if(tag==V_R4) v.value.r4=(float)n;
    else if(tag==V_R8) v.value.r8=n;
    else if(tag==V_DATE) {if(!isfinite(n) || n<=-657435.0 || n>=2958466.0)return 6;v.value.r8=n;}
    else return 13;
    *out=v; return 0;
}


#ifndef _WIN32
static int number(const VbaVariant *v, double *n) {
    switch(v->vt) {
    case V_EMPTY: *n=0; return 0;
    case V_NULL: return 94;
    case V_BOOL: case V_I2: *n=v->value.i2; return 0;
    case V_I4: *n=v->value.i4; return 0;
    case V_UI1: *n=v->value.byte; return 0;
    case V_R4: *n=v->value.r4; return 0;
    case V_R8: *n=v->value.r8; return 0;
    case V_DATE: *n=v->value.r8; return 0;
    case V_CY: *n=(double)v->value.i8/10000.0;return 0;
    case V_DECIMAL: {VbaVariant result={0};int e=vba_internal_numeric_fixed_convert(&result,v,V_R8);if(!e)*n=result.value.r8;return e;}
    default: return 13;
    }
}
#endif
#ifdef _WIN32
static int hresult(HRESULT hr) {
    if (SUCCEEDED(hr)) return 0;
    if (hr==DISP_E_OVERFLOW) return 6;
    if (hr==E_OUTOFMEMORY) return 14;
    if (hr==DISP_E_DIVBYZERO) return 11;
    return 13;
}
#endif

/* Windows uses the platform's Automation coercions. The portable profile
   implements the supported scalar set with C-locale decimal parsing/formatting. */
int vba_internal_convert(VbaVariant *out, const VbaVariant *source, uint16_t tag) {
    VbaVariant v;
    int error=vba_internal_borrow(&v,source);
    if (error) return error;
    if (v.vt==V_NULL) return 94;
    if (v.vt==V_ERROR) return 13;
    if (v.vt&V_ARRAY) {
        if(v.vt==(V_ARRAY|V_UI1) && tag==V_BSTR) return vba_internal_byte_array_bstr(out,v.value.pointer);
        return 13;
    }
    if(tag==V_DATE && v.vt==V_DATE && !vba_internal_date_valid(v.value.r8))return 6;
    if (v.vt==tag) return vba_variant_load(out,&v);
    /* Boolean-to-Byte is the exception to True's numeric -1 representation
       (MS-VBAL 5.5.1.2.2). Apply it to explicit, Let and ByRef coercions alike. */
    if(v.vt==V_BOOL && tag==V_UI1) {
        *out=(VbaVariant){.vt=V_UI1,.value.byte=v.value.boolean ? 255 : 0};
        return 0;
    }
    if(v.vt==V_PRIVATE_OBJECT) {
        VbaVariant simple={0};error=vba_object_value(&simple,&v);
        if(!error)error=vba_internal_convert(out,&simple,tag);
        vba_variant_clear(&simple);return error;
    }
    if(tag==V_PRIVATE_OBJECT) return 13;
    if(tag==V_CY || tag==V_DATE || tag==V_DECIMAL || v.vt==V_CY || v.vt==V_DATE || v.vt==V_DECIMAL)
        return vba_internal_numeric_fixed_convert(out,&v,tag);
#ifdef _WIN32
    VbaVariant result={0};
    error=hresult(VariantChangeType((VARIANT*)&result,(VARIANT*)&v,VARIANT_ALPHABOOL,tag));
    if (!error) *out=result;
    return error;
#else
    VbaVariant result={0}; result.vt=tag;
    double n=0;
    if (tag==V_BSTR) {
        char text[64];
        if (v.vt==V_EMPTY) text[0]=0;
        else if (v.vt==V_BOOL) strcpy(text,v.value.boolean ? "True" : "False");
        else {
            error=number(&v,&n); if (error) return error;
            if (!isfinite(n)) return 6;
            snprintf(text,sizeof(text),v.vt==V_R4 ? "%.7g" : "%.15g",n);
        }
        size_t len=strlen(text);
        uint16_t *b=vba_internal_bstr_alloc(NULL,(uint32_t)len);
        if (!b) return 14;
        for (size_t i=0;i<len;i++) b[i]=(unsigned char)text[i];
        result.value.pointer=b;
    } else {
        if (v.vt==V_BSTR) {
            uint32_t len=vba_internal_bstr_len(v.value.pointer);
            if (!len || len>1024) return 13;
            char text[1025];
            const uint16_t *b=v.value.pointer;
            for (uint32_t i=0;i<len;i++) { if (b[i]>127 || b[i]==0) return 13; text[i]=(char)b[i]; }
            text[len]=0;
            if (tag==V_BOOL && (!strcmp(text,"True") || !strcmp(text,"False"))) n=!strcmp(text,"True") ? -1 : 0;
            else {
                char *end=NULL; errno=0; n=strtod(text,&end);
                if (end==text) return 13;
                while (*end==' ' || *end=='\t' || *end=='\r' || *end=='\n') end++;
                if (*end) return 13;
                if (!isfinite(n)) return 6;
            }
        } else { error=number(&v,&n); if (error) return error; }
        if (!isfinite(n)) return 6;
        if (tag==V_BOOL) result.value.boolean=n!=0 ? -1 : 0;
        else if (tag==V_R8) result.value.r8=n;
        else if (tag==V_R4) { if(fabs(n)>FLT_MAX) return 6; result.value.r4=(float)n; }
        else {
            /* Round to nearest, ties to even, independent of ambient C fenv. */
            double floor_n=floor(n), fraction=n-floor_n;
            double rounded=fraction<0.5 ? floor_n : fraction>0.5 ? floor_n+1 : fmod(floor_n,2)==0 ? floor_n : floor_n+1;
            if (tag==V_UI1) { if (rounded < 0 || rounded > 255) return 6; result.value.byte=(uint8_t)rounded; }
            else if (tag==V_I2) { if (rounded < INT16_MIN || rounded > INT16_MAX) return 6; result.value.i2=(int16_t)rounded; }
            else if (tag==V_I4) { if (rounded < INT32_MIN || rounded > INT32_MAX) return 6; result.value.i4=(int32_t)rounded; }
            else return 13;
        }
    }
    *out=result; return 0;
#endif
}
int32_t vba_variant_to(void *out, const VbaVariant *source, int32_t tag) {
    VbaVariant value={0};
    int error=vba_internal_convert(&value,source,(uint16_t)tag);
    if (error) return error;
    if (tag==V_BSTR) error=string_from_bstr(out,value.value.pointer);
    else if (tag==V_UI1) memcpy(out,&value.value.byte,1);
    else if (tag==V_BOOL || tag==V_I2) memcpy(out,&value.value.i2,2);
    else if (tag==V_I4) memcpy(out,&value.value.i4,4);
    else if (tag==V_R4) memcpy(out,&value.value.r4,4);
    else if (tag==V_R8 || tag==V_DATE || tag==V_CY) memcpy(out,&value.value.r8,8);
    else if (tag==V_DECIMAL) {memcpy(out,&value.decimal,16);memset(out,0,2);}
    else error=13;
    vba_variant_clear(&value);
    return error;
}
int32_t vba_variant_store(VbaVariant *destination, const VbaVariant *source) {
    if (destination->vt==(V_VARIANT|V_BYREF)) {
        VbaVariant *p=destination->value.pointer;
        if (!p || p->vt==(V_VARIANT|V_BYREF)) return 13;
        return vba_variant_store(p,source);
    }
    VbaVariant value={0};
    if (!(destination->vt&V_BYREF)) {
        if ((destination->vt&V_ARRAY) && vba_array_check_unlocked(destination->value.pointer)) return 10;
        int error=vba_variant_load(&value,source);
        if (error) return error;
        VbaVariant old=*destination; *destination=value;vba_variant_clear(&old);return 0;
    }
    uint16_t tag=destination->vt&~V_BYREF;
    if (!destination->value.pointer) return 13;
    if(tag==V_PRIVATE_OBJECT) {
        void *object=NULL;int error=vba_variant_object(&object,source);if(error)return error;
        if(destination->value.record.info) error=vba_class_check(object,destination->value.record.info,1);
        if(!error && destination->reserved1)error=vba_object_check(object,destination->reserved1);
        if(error){vba_object_clear((VbaObject**)&object);return error;}
        VbaObject *old=*(VbaObject**)destination->value.pointer;
        *(VbaObject**)destination->value.pointer=object;vba_object_clear(&old);return 0;
    }
    if (tag&V_ARRAY) {
        int error=vba_internal_borrow(&value,source);
        if(error) return error;
        if(tag==(V_ARRAY|V_UI1) && value.vt==V_BSTR) {
            VbaArray *array=NULL;
            error=vba_internal_byte_array_from_bstr(&array,value.value.pointer);
            if(!error) error=vba_array_store(destination->value.pointer,array);
            vba_array_destroy(array);
            return error;
        }
        if(value.vt!=tag) return 13;
        if(tag==(V_ARRAY|V_PRIVATE_OBJECT)) {
            VbaObjectArrayType type={destination->value.record.info,destination->reserved1};VbaArray *array=NULL;
            error=vba_object_array_unbox(&array,&value,&type);
            if(!error)error=vba_array_store(destination->value.pointer,array);
            vba_array_destroy(array);return error;
        }
        return vba_array_store(destination->value.pointer,value.value.pointer);
    }
    int error=vba_internal_convert(&value,source,tag);
    if (error) return error;
    if (tag==V_BSTR) {
        void **slot=destination->value.pointer;
        void *old=*slot; *slot=value.value.pointer;
        value.vt=V_EMPTY; vba_bstr_free(old);
    } else if (tag==V_UI1) memcpy(destination->value.pointer,&value.value.byte,1);
    else if (tag==V_I2 || tag==V_BOOL) memcpy(destination->value.pointer,&value.value.i2,2);
    else if (tag==V_I4) memcpy(destination->value.pointer,&value.value.i4,4);
    else if (tag==V_R4) memcpy(destination->value.pointer,&value.value.r4,4);
    else if (tag==V_R8 || tag==V_DATE || tag==V_CY) memcpy(destination->value.pointer,&value.value.r8,8);
    else if (tag==V_DECIMAL) {memcpy(destination->value.pointer,&value.decimal,16);memset(destination->value.pointer,0,2);}
    else { vba_variant_clear(&value); return 13; }
    vba_variant_clear(&value); return 0;
}

/* Operation IDs are a runtime interface, independent of ast.Kind or ir.Op. */
static int32_t variant_binary(VbaVariant *out, const VbaVariant *left, const VbaVariant *right, int32_t op) {
    VbaVariant a,b,result={0};
    int error=vba_internal_borrow(&a,left); if (error) return error;
    error=vba_internal_borrow(&b,right); if (error) return error;
    /* Error operands fail before Null propagation, for every arithmetic and
       concatenation operator (MS-VBAL 5.6.9.3 and 5.6.9.4). */
    if ((a.vt|b.vt)&V_ARRAY || a.vt==V_ERROR || b.vt==V_ERROR) return 13;
#ifdef _WIN32
    HRESULT hr;
    switch(op) {
    case 1: hr=VarAdd((VARIANT*)&a,(VARIANT*)&b,(VARIANT*)&result); break;
    case 2: hr=VarSub((VARIANT*)&a,(VARIANT*)&b,(VARIANT*)&result); break;
    case 3: hr=VarMul((VARIANT*)&a,(VARIANT*)&b,(VARIANT*)&result); break;
    case 4: hr=VarCat((VARIANT*)&a,(VARIANT*)&b,(VARIANT*)&result); break;
    case 5: hr=VarDiv((VARIANT*)&a,(VARIANT*)&b,(VARIANT*)&result); break;
    case 6: hr=VarIdiv((VARIANT*)&a,(VARIANT*)&b,(VARIANT*)&result); break;
    case 7: hr=VarMod((VARIANT*)&a,(VARIANT*)&b,(VARIANT*)&result); break;
    default: return 13;
    }
    error=hresult(hr);
    if (!error) *out=result;
    return error;
#else
    if (op==4 || (op==1 && ((a.vt==V_BSTR && (b.vt==V_BSTR || b.vt==V_EMPTY)) || (b.vt==V_BSTR && a.vt==V_EMPTY)))) {
        if (op==1 && (a.vt==V_NULL || b.vt==V_NULL)) { result.vt=V_NULL; *out=result; return 0; }
        VbaVariant sa={0},sb={0};
        if (a.vt!=V_NULL) error=vba_internal_convert(&sa,&a,V_BSTR);
        if (!error && b.vt!=V_NULL) error=vba_internal_convert(&sb,&b,V_BSTR);
        if (error) { vba_variant_clear(&sa); return error; }
        if (a.vt==V_NULL && b.vt==V_NULL) { result.vt=V_NULL; *out=result; return 0; }
        uint32_t na=vba_internal_bstr_len(sa.value.pointer), nb=vba_internal_bstr_len(sb.value.pointer);
        if ((uint64_t)na+nb>INT32_MAX) { error=14; goto cat_done; }
        result.vt=V_BSTR; result.value.pointer=vba_internal_bstr_alloc(NULL,na+nb);
        if (!result.value.pointer) { error=14; goto cat_done; }
        if (na) memcpy(result.value.pointer,sa.value.pointer,(size_t)na*2);
        if (nb) memcpy((uint16_t*)result.value.pointer+na,sb.value.pointer,(size_t)nb*2);
        *out=result;
cat_done:
        vba_variant_clear(&sa); vba_variant_clear(&sb); return error;
    }
    if (a.vt==V_NULL || b.vt==V_NULL) { result.vt=V_NULL; *out=result; return 0; }
    if(op!=6 && op!=7 && (a.vt==V_DECIMAL || b.vt==V_DECIMAL || a.vt==V_DATE || b.vt==V_DATE ||
       ((a.vt==V_CY || b.vt==V_CY) && op!=5 && !(op==3 && (a.vt==V_R4 || a.vt==V_R8 || a.vt==V_BSTR || b.vt==V_R4 || b.vt==V_R8 || b.vt==V_BSTR)))))
        return vba_internal_numeric_binary(out,&a,&b,op);
    if(op==6 || op==7) {
        int narrow_left=a.vt==V_I2 || a.vt==V_BOOL;
        int floating_left=a.vt==V_R4 || a.vt==V_R8 || a.vt==V_BSTR || a.vt==V_CY || a.vt==V_DATE || a.vt==V_DECIMAL;
        int floating_right=b.vt==V_R4 || b.vt==V_R8 || b.vt==V_BSTR || b.vt==V_CY || b.vt==V_DATE || b.vt==V_DECIMAL;
        uint16_t tag=(narrow_left && floating_right) ? V_I2 :
            (floating_left || floating_right || a.vt==V_I4 || b.vt==V_I4) ? V_I4 : V_I2;
        VbaVariant x={0},y={0};
        error=vba_internal_convert(&x,&a,tag); if(error) return error;
        error=vba_internal_convert(&y,&b,tag); if(error) return error;
        int64_t nx=tag==V_I2 ? x.value.i2 : x.value.i4;
        int64_t ny=tag==V_I2 ? y.value.i2 : y.value.i4;
        if(!ny) return 11;
        int64_t z=op==6 ? nx/ny : nx%ny;
        if(tag==V_I2 && (z<INT16_MIN || z>INT16_MAX)) tag=V_I4;
        if(tag==V_I4 && (z<INT32_MIN || z>INT32_MAX)) tag=V_R8;
        if(a.vt==V_UI1 && b.vt==V_UI1) tag=V_UI1;
        result.vt=tag;
        if(tag==V_UI1) result.value.byte=(uint8_t)z;
        else if(tag==V_I2) result.value.i2=(int16_t)z;
        else if(tag==V_I4) result.value.i4=(int32_t)z;
        else result.value.r8=(double)z;
        *out=result; return 0;
    }
    VbaVariant na={0},nb={0};
    error=vba_internal_convert(&na,&a,V_R8); if (error) return error;
    error=vba_internal_convert(&nb,&b,V_R8); if (error) return error;
    double x=na.value.r8,y=nb.value.r8,z;
    if (op==1) z=x+y; else if (op==2) z=x-y; else if (op==3) z=x*y;
    else if(op==5) {
        if(y==0) return x!=0 || (b.vt==V_EMPTY && (a.vt==V_R4 || a.vt==V_R8 || a.vt==V_BSTR)) ? 11 : 6;
        z=x/y;
    } else return 13;
    if (!isfinite(z)) return 6;
    int single=a.vt==V_R4 || b.vt==V_R4;
    uint16_t tag=(a.vt==V_R8 || b.vt==V_R8 || a.vt==V_CY || b.vt==V_CY || a.vt==V_BSTR || b.vt==V_BSTR || (single && (a.vt==V_I4 || b.vt==V_I4)) || (op==5 && !single)) ? V_R8 : single ? V_R4 :
                 (a.vt==V_I4 || b.vt==V_I4) ? V_I4 : V_I2;
    if (tag==V_I2 && (z<INT16_MIN || z>INT16_MAX)) tag=V_I4;
    if (tag==V_I4 && (z<INT32_MIN || z>INT32_MAX)) tag=V_R8;
    if(tag==V_R4 && fabs(z)>FLT_MAX) tag=V_R8;
    result.vt=tag;
    if (tag==V_I2) result.value.i2=(int16_t)z;
    else if (tag==V_I4) result.value.i4=(int32_t)z;
    else if (tag==V_R4) result.value.r4=(float)z;
    else result.value.r8=z;
    *out=result; return 0;
#endif
}
int vba_internal_object_operand(VbaVariant *owned,const VbaVariant **source) {
    VbaVariant borrowed={0};int error=vba_internal_borrow(&borrowed,*source);if(error)return error;
    if(borrowed.vt!=V_PRIVATE_OBJECT)return 0;
    error=vba_object_value(owned,*source);if(!error)*source=owned;return error;
}
int32_t vba_variant_binary(VbaVariant *out,const VbaVariant *left,const VbaVariant *right,int32_t op) {
    VbaVariant a={0},b={0};int error=vba_internal_object_operand(&a,&left);
    if(!error)error=vba_internal_object_operand(&b,&right);
    if(!error)error=variant_binary(out,left,right,op);
    vba_variant_clear(&b);vba_variant_clear(&a);return error;
}
static int32_t variant_unary(VbaVariant *out,const VbaVariant *source,int32_t op) {
    VbaVariant v;
    int error=vba_internal_borrow(&v,source); if (error) return error;
    if (v.vt&V_ARRAY) return 13;
    if (v.vt==V_NULL) { *out=v; return 0; }
    if(op==1 && v.vt==V_CY){if(v.value.i8==INT64_MIN)return 6;v.value.i8=-v.value.i8;*out=v;return 0;}
    if(op==1 && v.vt==V_DECIMAL){if(v.decimal.lo || v.decimal.hi)v.decimal.sign^=0x80;*out=v;return 0;}
    if(op==1 && v.vt==V_DATE){v.value.r8=-v.value.r8;if(!vba_internal_date_valid(v.value.r8))v.vt=V_R8;*out=v;return 0;}
#ifdef _WIN32
    VbaVariant result={0};
    if (op!=1) return 13;
    error=hresult(VarNeg((VARIANT*)&v,(VARIANT*)&result));
    if (!error) *out=result;
    return error;
#else
    if (op!=1) return 13;
    VbaVariant zero={0}; zero.vt=V_I2;
    return vba_variant_binary(out,&zero,&v,2);
#endif
}
int32_t vba_variant_unary(VbaVariant *out,const VbaVariant *source,int32_t op) {
    VbaVariant value={0};int error=vba_internal_object_operand(&value,&source);
    if(!error)error=variant_unary(out,source,op);
    vba_variant_clear(&value);return error;
}
int32_t vba_variant_query(const VbaVariant *source,int32_t op) {
    if (source->vt==(V_VARIANT|V_BYREF)) source=source->value.pointer;
    if (!source || source->vt==(V_VARIANT|V_BYREF)) return 0;
    uint16_t tag=source->vt&~V_BYREF;
    if (op==1) return tag==V_PRIVATE_OBJECT ? 9 : tag==(V_ARRAY|V_PRIVATE_OBJECT) ? V_ARRAY|9 : tag;
    if (op==2) return tag==V_EMPTY ? -1 : 0;
    if (op==3) return tag==V_NULL ? -1 : 0;
    if (op==4) {
        VbaVariant v;
        return !vba_internal_borrow(&v,source) && v.vt==V_ERROR && (uint32_t)v.value.error==UINT32_C(0x80020004) ? -1 : 0;
    }
    if (op==5) return tag&V_ARRAY ? -1 : 0;
    return 0;
}
int32_t vba_variant_len(VbaVariant *out,const VbaVariant *source,int32_t bytes) {
    VbaVariant v,result={0};
    int error=vba_internal_borrow(&v,source); if (error) return error;
    if (v.vt==V_NULL) { result.vt=V_NULL; *out=result; return 0; }
    VbaVariant text={0};
    if (v.vt!=V_BSTR) { error=vba_internal_convert(&text,&v,V_BSTR); if (error) return error; v=text; }
    int64_t length=(int64_t)vba_internal_bstr_len(v.value.pointer)*(bytes?2:1);
    vba_variant_clear(&text);
    if (length>INT32_MAX) return 6;
    result.vt=V_I4; result.value.i4=(int32_t)length; *out=result; return 0;
}
