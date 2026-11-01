#include "conversion.h"
#include "runtime_internal.h"
#include <stdio.h>
#include <string.h>

/* MS-VBAL 6.1.2.3: explicit conversion functions differ from Let coercion for
   Error values. Keep that exception out of vba_internal_convert(), assignment and ByRef stores. */
int32_t vba_convert_explicit(void *out,const VbaVariant *source,int32_t tag) {
    switch(tag) {
    case V_UI1: case V_I2: case V_I4: case V_BOOL: case V_R4: case V_R8: case V_CY: case V_DATE: case V_BSTR: break;
    default: return 13;
    }
    VbaVariant value={0};
    int error=vba_internal_borrow(&value,source);
    if(error) return error;
    if(value.vt==V_ERROR) {
        if(tag==V_DATE)return 13;
        if(tag==V_BSTR) {
            char text[32];
            int length=snprintf(text,sizeof(text),"Error %ld",(long)value.value.error);
            uint16_t units[32];
            for(int i=0;i<length;i++) units[i]=(unsigned char)text[i];
            VbaString string_value={.data=units,.length=length};
            VbaVariant owned={0};
            error=vba_variant_box_string(&owned,&string_value);
            if(!error) error=vba_variant_to(out,&owned,V_BSTR);
            vba_variant_clear(&owned);
            return error;
        }
        value.vt=V_I4; // The signed Long error code is explicitly converted.
    }
    return vba_variant_to(out,&value,tag);
}

int32_t vba_cvdate(VbaVariant *out,const VbaVariant *source) {return vba_internal_convert(out,source,V_DATE);}
int32_t vba_cdec(VbaVariant *out,const VbaVariant *source) {return vba_internal_convert(out,source,V_DECIMAL);}

int32_t vba_cverr(VbaVariant *out,const VbaVariant *source) {
    VbaVariant value={0};
    int error=vba_internal_borrow(&value,source);
    if(error) return error;
    if(value.vt==V_ERROR) { *out=value; return 0; }
    int32_t code=0;
    error=vba_variant_to(&code,&value,V_I4);
    if(error) return error;
    if(code<0 || code>65535) return 5;
    *out=(VbaVariant){.vt=V_ERROR,.value.error=code};
    return 0;
}

int32_t vba_iserror(int16_t *out,const VbaVariant *source) {
    VbaVariant value={0};
    int error=vba_internal_borrow(&value,source);
    if(!error) *out=value.vt==V_ERROR ? -1 : 0;
    return error;
}
