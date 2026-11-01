#include "control.h"
#include "runtime_internal.h"
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <limits.h>
#ifdef _WIN32
#include <windows.h>
#include <oleauto.h>
#endif

/* Truth coercion differs from CBool: Null is False in control conditions. */
int32_t vba_condition(int16_t *out,const VbaVariant *source) {
    VbaVariant value={0};
    int error=vba_internal_borrow(&value,source);
    if(error) return error;
    if(value.vt==V_NULL) { *out=0; return 0; }
    VbaVariant boolean={0};
    error=vba_internal_convert(&boolean,&value,V_BOOL);
    if(!error) *out=boolean.value.boolean;
    vba_variant_clear(&boolean);
    return error;
}
int vba_internal_compare_bstr(const uint16_t *a,const uint16_t *b) {
    uint32_t na=vba_internal_bstr_len(a),nb=vba_internal_bstr_len(b),n=na<nb ? na : nb;
    for(uint32_t i=0;i<n;i++) { if(a[i]!=b[i]) return a[i]<b[i] ? -1 : 1; }
    return na<nb ? -1 : na>nb ? 1 : 0;
}
/* mode 0: both operands declared Variant; 1: numeric declared operand;
   2: String declared operand. Comparisons use binary UTF-16 String ordering. */
static int32_t variant_compare(VbaVariant *out,const VbaVariant *left,const VbaVariant *right,int32_t op,int32_t mode) {
    if(mode<0 || mode>2) return 13;
    VbaVariant a,b,result={0};
    int error=vba_internal_borrow(&a,left); if(error) return error;
    error=vba_internal_borrow(&b,right); if(error) return error;
    if((a.vt|b.vt)&V_ARRAY) return 13;
    int comparison=0;
    if(a.vt==V_ERROR || b.vt==V_ERROR) {
        if(a.vt!=V_ERROR || b.vt!=V_ERROR) return 13;
        /* Standard Error codes compare numerically. MS-VBAL leaves comparison
           of implementation-defined Error values unspecified; reject it here. */
        if(a.value.error<0 || a.value.error>65535 || b.value.error<0 || b.value.error>65535) return 13;
        comparison=a.value.error<b.value.error ? -1 : a.value.error>b.value.error ? 1 : 0;
    } else if(a.vt==V_NULL || b.vt==V_NULL) { result.vt=V_NULL; *out=result; return 0; }
    else if(mode==2 || (mode==0 && (a.vt==V_BSTR || a.vt==V_EMPTY) && (b.vt==V_BSTR || b.vt==V_EMPTY))) {
        VbaVariant sa={0},sb={0};
        error=vba_internal_convert(&sa,&a,V_BSTR);
        if(!error) error=vba_internal_convert(&sb,&b,V_BSTR);
        if(!error) comparison=vba_internal_compare_bstr(sa.value.pointer,sb.value.pointer);
        vba_variant_clear(&sa); vba_variant_clear(&sb);
        if(error) return error;
    } else if(mode==0 && (a.vt==V_BSTR || b.vt==V_BSTR)) {
        comparison=a.vt==V_BSTR ? 1 : -1;
    } else if(a.vt==V_DECIMAL || b.vt==V_DECIMAL || a.vt==V_CY || b.vt==V_CY || a.vt==V_DATE || b.vt==V_DATE) {
        error=vba_internal_numeric_compare(&a,&b,&comparison);if(error)return error;
    } else {
        VbaVariant na={0},nb={0},ca={0},cb={0};
        uint16_t tag=V_R8;
        if(a.vt==V_BSTR || b.vt==V_BSTR) {
            uint16_t other=a.vt==V_BSTR ? b.vt : a.vt;
            if(other==V_I2 || other==V_I4 || other==V_BOOL || other==V_R4) tag=other;
        }
        error=vba_internal_convert(&ca,&a,tag); if(error) return error;
        error=vba_internal_convert(&cb,&b,tag); if(error) return error;
        error=vba_internal_convert(&na,&ca,V_R8); if(error) return error;
        error=vba_internal_convert(&nb,&cb,V_R8); if(error) return error;
        if(isnan(na.value.r8) || isnan(nb.value.r8)) return 6;
        comparison=na.value.r8<nb.value.r8 ? -1 : na.value.r8>nb.value.r8 ? 1 : 0;
    }
    int truth;
    switch(op) {
    case 1: truth=comparison==0; break; case 2: truth=comparison!=0; break;
    case 3: truth=comparison<0; break; case 4: truth=comparison<=0; break;
    case 5: truth=comparison>0; break; case 6: truth=comparison>=0; break;
    default: return 13;
    }
    result.vt=V_BOOL; result.value.boolean=truth ? -1 : 0;
    *out=result; return 0;
}
int32_t vba_variant_compare(VbaVariant *out,const VbaVariant *left,const VbaVariant *right,int32_t op,int32_t mode) {
    VbaVariant a={0},b={0};int error=vba_internal_object_operand(&a,&left);
    if(!error)error=vba_internal_object_operand(&b,&right);
    if(!error)error=variant_compare(out,left,right,op,mode);
    vba_variant_clear(&b);vba_variant_clear(&a);return error;
}
static int for_number(const VbaVariant *source,double *out) {
    VbaVariant value={0},number_value={0};
    int error=vba_internal_borrow(&value,source); if(error) return error;
    switch(value.vt) {
    case V_EMPTY: case V_I2: case V_I4: case V_BOOL: case V_UI1: case V_R4: case V_R8: break;
    default: return 13;
    }
    error=vba_internal_convert(&number_value,&value,V_R8);
    if(error) return error;
    if(!isfinite(number_value.value.r8)) return 13;
    *out=number_value.value.r8; return 0;
}
int32_t vba_for_validate(const VbaVariant *start,const VbaVariant *limit,const VbaVariant *step) {
    double n=0;
    int error=for_number(start,&n); if(error) return error;
    error=for_number(limit,&n); if(error) return error;
    return for_number(step,&n);
}
int32_t vba_for_test(int16_t *out,const VbaVariant *counter,const VbaVariant *limit,const VbaVariant *step) {
    double a,b,c;
    int error=for_number(counter,&a); if(error) return error;
    error=for_number(limit,&b); if(error) return error;
    error=for_number(step,&c); if(error) return error;
    *out=(c>=0 ? a<=b : a>=b) ? -1 : 0;
    return 0;
}
void vba_enum_clear(VbaEnumerator *state) {
 if(state->initialized==2){vba_collection_enum_clear(state);return;}
    if(state->array) {
        vba_internal_array_unlock(state->array);
        if(state->owns) vba_array_destroy(state->array);
    }
    memset(state,0,sizeof(*state));
}
int32_t vba_enum_start(VbaEnumerator *out,void *slot,int32_t declared,int32_t clone) {
    VbaArray **location=NULL; VbaVariant *variant=NULL; int constraint=0;
    int error=vba_internal_array_location(slot,declared,&location,&variant,&constraint);
    if(error) return error;
    if(variant && !(variant->vt&V_ARRAY)) return 13;
    VbaEnumerator state={0}; state.initialized=1; state.owns=clone!=0;
    state.array=*location;
    if(clone) { state.array=NULL; error=vba_array_copy(&state.array,*location); if(error) return error; }
    if(state.array) {
        error=vba_internal_array_lock(state.array);
        if(error){if(clone)vba_array_destroy(state.array);return error;}
    }
    vba_enum_clear(out); *out=state; return 0;
}
int32_t vba_enum_next(int16_t *out,VbaEnumerator *state,VbaVariant *counter) {
 if(state->initialized==2)return vba_collection_enum_next(out,state,counter);
    if(!state->initialized) return 92;
    VbaArray *a=state->array;
    if(!a || state->index>=vba_internal_array_count(a)) { *out=0; return 0; }
    uint16_t tag=vba_internal_array_tag(a);
    if(!vba_internal_array_element_size(tag)) return 13;
    void *cell=(unsigned char*)a->data+(size_t)state->index*a->element_size;
    VbaVariant view={0}; view.vt=V_BYREF|tag; view.value.pointer=cell;
    const VbaVariant *source=tag==V_VARIANT ? cell : &view;
    if(counter->vt==(V_BYREF|V_PRIVATE_OBJECT)) {
        VbaVariant value={0};int e=vba_internal_borrow(&value,source);if(e)return e;
        if(value.vt!=V_PRIVATE_OBJECT)return 13;
    }
    int error=vba_variant_store(counter,source);
    if(error) return error;
    state->index++; *out=-1; return 0;
}

int32_t vba_gosub_push_ptr(VbaGosubStack *stack,void *address) {
    if(stack->count==stack->capacity) {
        uint64_t capacity=stack->capacity ? stack->capacity*2 : 8;
        if(capacity<stack->capacity || capacity>SIZE_MAX/sizeof(void*)) return 7;
        void **entries=malloc((size_t)capacity*sizeof(void*));
        if(!entries) return 7;
        if(stack->count) memcpy(entries,stack->entries,(size_t)stack->count*sizeof(void*));
        if(stack->entries) free(stack->entries);stack->entries=entries;stack->capacity=capacity;
    }
    stack->entries[stack->count++]=address;return 0;
}
int32_t vba_gosub_pop_ptr(void **out,VbaGosubStack *stack) {
    if(!stack->count) return 3;
    *out=stack->entries[--stack->count];return 0;
}
void vba_gosub_clear(VbaGosubStack *stack) {
    if(stack->entries) free(stack->entries);memset(stack,0,sizeof(*stack));
}
