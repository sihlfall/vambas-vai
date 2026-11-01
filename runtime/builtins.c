#include "builtins.h"
#include "runtime_internal.h"
#include <stdlib.h>
#include <string.h>
#include <limits.h>

#ifdef _WIN32
#include <windows.h>
#endif

/* Library operations share Automation coercions and ownership with Variant,
   String and SAFEARRAY operations. Outputs commit only after success. */
static int builtin_string(const VbaVariant *source,VbaVariant *owner,const uint16_t **text) {
    VbaVariant value={0};
    int error=vba_internal_borrow(&value,source); if(error) return error;
    if(value.vt==V_BSTR) { *text=value.value.pointer; return 0; }
    error=vba_internal_convert(owner,&value,V_BSTR);
    if(!error) *text=owner->value.pointer;
    return error;
}
static int builtin_compare_mode(const VbaVariant *source) {
    VbaVariant value={0};
    int error=vba_internal_convert(&value,source,V_I4);
    if(error) return error;
    /* This compiler profile implements binary comparison. Option Compare Text,
       database and LCID comparisons require an explicit locale-aware profile. */
    return value.value.i4==0 || value.value.i4==-1 ? 0 : 5;
}
static int builtin_string_result(void *out,VbaVariant *result,int dollar) {
    int error=0;
    if(dollar) error=vba_variant_to(out,result,V_BSTR);
    else { *(VbaVariant*)out=*result; memset(result,0,sizeof(*result)); }
    vba_variant_clear(result);
    return error;
}
int32_t vba_ascw(int16_t *out,const VbaVariant *source) {
    VbaVariant owner={0}; const uint16_t *text=NULL;
    int error=builtin_string(source,&owner,&text);
    if(!error) {
        if(!vba_internal_bstr_len(text)) error=5;
        else { uint16_t unit=text[0]; memcpy(out,&unit,2); }
    }
    vba_variant_clear(&owner); return error;
}
int32_t vba_strcomp(VbaVariant *out,const VbaVariant *left,const VbaVariant *right,const VbaVariant *mode) {
    int error=builtin_compare_mode(mode); if(error) return error;
    VbaVariant a={0},b={0},oa={0},ob={0},result={0};
    error=vba_internal_borrow(&a,left); if(error) return error;
    error=vba_internal_borrow(&b,right); if(error) return error;
    if(a.vt==V_NULL || b.vt==V_NULL) { result.vt=V_NULL; *out=result; return 0; }
    const uint16_t *sa=NULL,*sb=NULL;
    error=builtin_string(&a,&oa,&sa);
    if(!error) error=builtin_string(&b,&ob,&sb);
    if(!error) { result.vt=V_I2; result.value.i2=(int16_t)vba_internal_compare_bstr(sa,sb); *out=result; }
    vba_variant_clear(&oa); vba_variant_clear(&ob); return error;
}
/* Search exact UTF-16 units, including embedded zero and unpaired surrogates. */
static int64_t builtin_find(const uint16_t *text,uint32_t length,const uint16_t *needle,uint32_t size,uint32_t start) {
    if(size>length || start>length-size) return -1;
    for(uint32_t i=start;i<=length-size;i++) {
        if(!memcmp(text+i,needle,(size_t)size*2)) return i;
    }
    return -1;
}
int32_t vba_instr(VbaVariant *out,const VbaVariant *start,const VbaVariant *text,const VbaVariant *needle,const VbaVariant *mode) {
    VbaVariant from={0};
    int error=vba_internal_convert(&from,start,V_I4); if(error) return error;
    if(from.value.i4<1) return 5;
    error=builtin_compare_mode(mode); if(error) return error;
    VbaVariant a={0},b={0},oa={0},ob={0},result={0};
    error=vba_internal_borrow(&a,text); if(error) return error;
    error=vba_internal_borrow(&b,needle); if(error) return error;
    if(a.vt==V_NULL || b.vt==V_NULL) { result.vt=V_NULL; *out=result; return 0; }
    const uint16_t *sa=NULL,*sb=NULL;
    error=builtin_string(&a,&oa,&sa);
    if(!error) error=builtin_string(&b,&ob,&sb);
    if(!error) {
        uint32_t na=vba_internal_bstr_len(sa),nb=vba_internal_bstr_len(sb),index=(uint32_t)from.value.i4-1;
        result.vt=V_I4;
        if(na && !nb) result.value.i4=from.value.i4;
        else if(na && index<na) {
            int64_t found=builtin_find(sa,na,sb,nb,index);
            if(found>=0) result.value.i4=(int32_t)(found+1);
        }
        *out=result;
    }
    vba_variant_clear(&oa); vba_variant_clear(&ob); return error;
}
int32_t vba_left(void *out,const VbaVariant *source,const VbaVariant *length,int32_t dollar) {
    VbaVariant count={0},value={0},owner={0},result={0};
    int error=vba_internal_convert(&count,length,V_I4); if(error) return error;
    if(count.value.i4<0) return 5;
    error=vba_internal_borrow(&value,source); if(error) return error;
    if(value.vt==V_NULL) { result.vt=V_NULL; return builtin_string_result(out,&result,dollar); }
    const uint16_t *text=NULL;
    error=builtin_string(&value,&owner,&text);
    if(!error) {
        uint32_t n=vba_internal_bstr_len(text);
        if((uint32_t)count.value.i4<n) n=(uint32_t)count.value.i4;
        result.vt=V_BSTR; result.value.pointer=vba_internal_bstr_alloc(text,n);
        if(!result.value.pointer) error=14;
    }
    vba_variant_clear(&owner);
    if(error) return error;
    return builtin_string_result(out,&result,dollar);
}
/* Numeric String characters use the ANSI code page, unlike ChrW. Windows uses
   its ACP; the portable runtime's explicit ANSI profile is Windows-1252. */
static uint16_t builtin_ansi(uint8_t byte) {
#ifdef _WIN32
    char c=(char)byte; WCHAR unit=0;
    if(!MultiByteToWideChar(CP_ACP,0,&c,1,&unit,1)) return byte;
    return (uint16_t)unit;
#else
    static const uint16_t cp1252[32]={0x20ac,0x81,0x201a,0x192,0x201e,0x2026,0x2020,0x2021,0x2c6,0x2030,0x160,0x2039,0x152,0x8d,0x17d,0x8f,0x90,0x2018,0x2019,0x201c,0x201d,0x2022,0x2013,0x2014,0x2dc,0x2122,0x161,0x203a,0x153,0x9d,0x17e,0x178};
    return byte>=128 && byte<160 ? cp1252[byte-128] : byte;
#endif
}
int32_t vba_string_repeat(void *out,const VbaVariant *number_value,const VbaVariant *character,int32_t dollar) {
    VbaVariant n={0},c={0},count={0},code={0},result={0};
    int error=vba_internal_borrow(&n,number_value); if(error) return error;
    error=vba_internal_borrow(&c,character); if(error) return error;
    if(n.vt==V_NULL || c.vt==V_NULL) { result.vt=V_NULL; return builtin_string_result(out,&result,dollar); }
    error=vba_internal_convert(&count,&n,V_I4); if(error) return error;
    if(count.value.i4<0) return 5;
    uint16_t unit=0;
    if(c.vt==V_BSTR) {
        if(!vba_internal_bstr_len(c.value.pointer)) return 5;
        unit=((const uint16_t*)c.value.pointer)[0];
    } else {
        error=vba_internal_convert(&code,&c,V_I4); if(error) return error;
        if(code.value.i4<0) return 5;
        unit=builtin_ansi((uint8_t)code.value.i4);
    }
    result.vt=V_BSTR; result.value.pointer=vba_internal_bstr_alloc(NULL,(uint32_t)count.value.i4);
    if(!result.value.pointer) return 14;
    uint16_t *p=result.value.pointer;
    for(int32_t i=0;i<count.value.i4;i++) p[i]=unit;
    return builtin_string_result(out,&result,dollar);
}
int32_t vba_split(VbaArray **out,const VbaVariant *expression,const VbaVariant *delimiter,const VbaVariant *limit,const VbaVariant *mode) {
    VbaVariant max={0},oe={0},od={0}; const uint16_t *text=NULL,*sep=NULL;
    int error=vba_internal_convert(&max,limit,V_I4); if(error) return error;
    if(max.value.i4 < -1) return 5;
    error=builtin_compare_mode(mode); if(error) return error;
    error=builtin_string(expression,&oe,&text);
    if(!error) error=builtin_string(delimiter,&od,&sep);
    VbaArray *result=NULL;
    if(!error) {
        uint32_t n=vba_internal_bstr_len(text),d=vba_internal_bstr_len(sep),count=n && max.value.i4!=0 ? 1 : 0;
        uint32_t cap=max.value.i4<0 ? INT32_MAX : (uint32_t)max.value.i4;
        uint32_t at=0;
        if(count && d) {
            while(count<cap) {
                int64_t match=builtin_find(text,n,sep,d,at);
                if(match<0) break;
                count++; at=(uint32_t)match+d;
            }
        }
        VbaArrayBound bound={count,0};
        error=vba_internal_array_create(&result,V_BSTR,1,&bound);
        if(!error) {
            uint16_t **elements=result->data; at=0;
            for(uint32_t i=0;i<count;i++) {
                int64_t match=i+1<count ? builtin_find(text,n,sep,d,at) : -1;
                uint32_t end=match<0 ? n : (uint32_t)match;
                elements[i]=vba_internal_bstr_alloc(text+at,end-at);
                if(!elements[i]) { error=14; break; }
                at=end+d;
            }
        }
    }
    vba_variant_clear(&oe); vba_variant_clear(&od);
    if(error) { if(result) vba_array_destroy(result); return error; }
    *out=result; return 0;
}
int32_t vba_iif(VbaVariant *out,const VbaVariant *expression,const VbaVariant *truepart,const VbaVariant *falsepart) {
    int16_t condition=0;
    int error=vba_condition(&condition,expression); if(error) return error;
    return vba_variant_load(out,condition ? truepart : falsepart);
}
