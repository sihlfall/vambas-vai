#include "errors.h"
#include "runtime_internal.h"
#include <string.h>

/* Intrinsic Err state is independent of each activation's handler policy.
   It is private runtime state; no object/COM pointer is stored in VARIANT. */
#include <uchar.h>
static VbaErrState current_err;
static int raise_committed;
#define ERR_TEXT(s) ((VbaString){.data=(uint16_t*)u##s,.length=(int32_t)(sizeof(u##s)/2-1)})
static VbaString err_message(int32_t number_value) {
 switch(number_value) {
 case 457:return ERR_TEXT("This key is already associated with an element of this collection");
 case 438:return ERR_TEXT("Object does not support this property or method");
 case 3:return ERR_TEXT("Return without GoSub");
 case 5:return ERR_TEXT("Invalid procedure call or argument");case 6:return ERR_TEXT("Overflow");
 case 7:return ERR_TEXT("Out of memory");case 9:return ERR_TEXT("Subscript out of range");
 case 10:return ERR_TEXT("This array is fixed or temporarily locked");case 11:return ERR_TEXT("Division by zero");
 case 13:return ERR_TEXT("Type mismatch");case 14:return ERR_TEXT("Out of string space");
 case 20:return ERR_TEXT("Resume without error");case 91:return ERR_TEXT("Object variable or With block variable not set");
 case 92:return ERR_TEXT("For loop not initialized");case 94:return ERR_TEXT("Invalid use of Null");
 default:return ERR_TEXT("Application-defined or object-defined error");
 }
}
void vba_err_clear(void) {
 vba_internal_string_drop(current_err.source);vba_internal_string_drop(current_err.description);vba_internal_string_drop(current_err.help_file);
 memset(&current_err,0,sizeof(current_err));
}
void vba_err_capture(int32_t code,const VbaString *source) {
 vba_err_clear();current_err.number=code;current_err.description=err_message(code);
 if(source) current_err.source=*source; /* Compiler-owned immutable literal. */
}
static int err_missing(const VbaVariant *value) {VbaVariant v={0};return !vba_internal_borrow(&v,value) && v.vt==V_ERROR && v.value.error==(int32_t)0x80020004;}
int32_t vba_err_raise(const VbaVariant *number_value,const VbaVariant *source,const VbaVariant *description,const VbaVariant *help_file,const VbaVariant *help_context,const VbaString *origin) {
 raise_committed=0;
 VbaErrState next={0};int e=vba_variant_to(&next.number,number_value,V_I4);if(e)return e;
 if(!next.number)return 5;
 if(err_missing(source)) e=vba_string_load(&next.source,vba_internal_slen(&current_err.source)?&current_err.source:origin);
 else e=vba_variant_to(&next.source,source,V_BSTR);
 if(!e) {if(err_missing(description)) {VbaString d=vba_internal_slen(&current_err.description)?current_err.description:err_message(next.number);e=vba_string_load(&next.description,&d);} else e=vba_variant_to(&next.description,description,V_BSTR);}
 if(!e) {if(err_missing(help_file)) e=vba_string_load(&next.help_file,&current_err.help_file);else e=vba_variant_to(&next.help_file,help_file,V_BSTR);}
 if(!e) {if(err_missing(help_context)) next.help_context=current_err.help_context;else e=vba_variant_to(&next.help_context,help_context,V_I4);}
 if(e){vba_internal_string_drop(next.source);vba_internal_string_drop(next.description);vba_internal_string_drop(next.help_file);return e;}
 vba_err_clear();current_err=next;raise_committed=1;return next.number;
}
int32_t vba_err_get(void *out,int32_t property) {
 switch(property) {
 case 0:memcpy(out,&current_err.number,4);return 0;
 case 1:return vba_string_load(out,&current_err.description);
 case 2:return vba_string_load(out,&current_err.source);
 case 3:return vba_string_load(out,&current_err.help_file);
 case 4:memcpy(out,&current_err.help_context,4);return 0;
 default:return 5;
 }
}
int32_t vba_err_set(int32_t property,const VbaVariant *value) {
 if(property==0 || property==4){int32_t n=0;int e=vba_variant_to(&n,value,V_I4);if(e)return e;if(property==0)current_err.number=n;else current_err.help_context=n;return 0;}
 VbaString next={0};int e=vba_variant_to(&next,value,V_BSTR);if(e)return e;
 VbaString *slot=property==1?&current_err.description:property==2?&current_err.source:property==3?&current_err.help_file:NULL;
 if(!slot){vba_internal_string_drop(next);return 5;}vba_internal_string_drop(*slot);*slot=next;return 0;
}
#undef ERR_TEXT

void vba_err_raise_failure(int32_t code,const VbaString *source){if(!raise_committed)vba_err_capture(code,source);}

VbaString vba_internal_err_description(int32_t code) {
 return current_err.number==code ? current_err.description : err_message(code);
}

VbaErrState vba_internal_err_save(void) {
 VbaErrState saved=current_err;
 vba_internal_string_retain(saved.source);vba_internal_string_retain(saved.description);vba_internal_string_retain(saved.help_file);
 return saved;
}

void vba_internal_err_restore(VbaErrState saved) {
 vba_err_clear();current_err=saved;
}
