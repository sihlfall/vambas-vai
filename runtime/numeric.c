#include "runtime_internal.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <math.h>
#include <limits.h>

/* Portable fixed-point arithmetic. Decimal coefficients are never represented
   by a Double. Eight little-endian limbs hold all aligned 96-bit operands and
   their 192-bit products, with room for division guard digits. */
typedef struct { uint32_t w[8]; } NumericBig;
typedef struct { NumericBig magnitude; int scale, negative; } NumericDecimal;
static int nb_compare(NumericBig a,NumericBig b) {
    for(int i=7;i>=0;i--) if(a.w[i]!=b.w[i]) return a.w[i]<b.w[i]?-1:1;
    return 0;
}
static int nb_zero(NumericBig a) { NumericBig zero={0};return !nb_compare(a,zero); }
static NumericBig nb_u64(uint64_t n) { NumericBig a={{(uint32_t)n,(uint32_t)(n>>32)}};return a; }
static int nb_mul_small(NumericBig *a,uint32_t n) {
    uint64_t carry=0;
    for(int i=0;i<8;i++){uint64_t v=(uint64_t)a->w[i]*n+carry;a->w[i]=(uint32_t)v;carry=v>>32;}
    return carry!=0;
}
static uint32_t nb_div_small(NumericBig *a,uint32_t n) {
    uint64_t rem=0;
    for(int i=7;i>=0;i--){uint64_t v=(rem<<32)|a->w[i];a->w[i]=(uint32_t)(v/n);rem=v%n;}
    return (uint32_t)rem;
}
static NumericBig nb_add(NumericBig a,NumericBig b) {
    uint64_t carry=0;
    for(int i=0;i<8;i++){uint64_t v=(uint64_t)a.w[i]+b.w[i]+carry;a.w[i]=(uint32_t)v;carry=v>>32;}
    return a;
}
static NumericBig nb_sub(NumericBig a,NumericBig b) {
    uint64_t borrow_value=0;
    for(int i=0;i<8;i++){uint64_t v=(uint64_t)b.w[i]+borrow_value;uint32_t old=a.w[i];a.w[i]=old-(uint32_t)v;borrow_value=(uint64_t)old<v;}
    return a;
}
static NumericBig nb_multiply(NumericBig a,NumericBig b) {
    NumericBig result={0};
    for(int i=0;i<8;i++) {
        uint64_t carry=0;
        for(int j=0;j+i<8;j++){uint64_t v=(uint64_t)a.w[i]*b.w[j]+result.w[i+j]+carry;result.w[i+j]=(uint32_t)v;carry=v>>32;}
    }
    return result;
}
static void nb_divide(NumericBig n,NumericBig d,NumericBig *q,NumericBig *r) {
    *q=(NumericBig){0};*r=(NumericBig){0};
    for(int bit=255;bit>=0;bit--) {
        nb_mul_small(r,2);r->w[0]|=(n.w[bit/32]>>(bit%32))&1;
        if(nb_compare(*r,d)>=0){*r=nb_sub(*r,d);q->w[bit/32]|=UINT32_C(1)<<(bit%32);}
    }
}
static int nb_fits96(NumericBig a) {return !(a.w[3]|a.w[4]|a.w[5]|a.w[6]|a.w[7]);}
static int nb_fits64(NumericBig a) {return nb_fits96(a) && !a.w[2];}
static uint64_t nb_low64(NumericBig a) {return ((uint64_t)a.w[1]<<32)|a.w[0];}
static NumericBig nb_round_scale(NumericBig n,int *scale,int target,int *digit,int *sticky) {
    while(*scale>target){*sticky|=*digit!=0;*digit=(int)nb_div_small(&n,10);--*scale;}
    return n;
}
static int decimal_finish(VbaVariant *out,NumericDecimal d,int digit,int sticky) {
    for(;;) {
        while(d.scale>28 || !nb_fits96(d.magnitude)) {
            if(d.scale<=0)return 6;
            sticky|=digit!=0;digit=(int)nb_div_small(&d.magnitude,10);d.scale--;
        }
        NumericBig rounded=d.magnitude;
        if(digit>5 || (digit==5 && (sticky || (rounded.w[0]&1)))) rounded=nb_add(rounded,nb_u64(1));
        if(nb_fits96(rounded)) {
            VbaVariant result={0};result.vt=V_DECIMAL;
            result.decimal.lo=nb_low64(rounded);result.decimal.hi=rounded.w[2];
            result.decimal.scale=(uint8_t)d.scale;
            result.decimal.sign=d.negative && !nb_zero(rounded)?0x80:0;
            *out=result;return 0;
        }
        if(!d.scale)return 6;
        sticky|=digit!=0;digit=(int)nb_div_small(&d.magnitude,10);d.scale--;
    }
}
static int numeric_parse(NumericDecimal *out,const uint16_t *text,uint32_t length) {
    if(!length || length>1024)return 13;
    char ascii[1025];
    for(uint32_t i=0;i<length;i++){if(text[i]>127 || !text[i])return 13;ascii[i]=(char)text[i];}
    ascii[length]=0;char *p=ascii;
    while(*p==' '||*p=='\t'||*p=='\r'||*p=='\n')p++;
    NumericDecimal d={0};if(*p=='-'||*p=='+')d.negative=*p++=='-';
    int digits=0,point=0,overflow=0;
    while((*p>='0'&&*p<='9') || (*p=='.'&&!point)) {
        if(*p=='.'){point=1;p++;continue;}
        overflow|=nb_mul_small(&d.magnitude,10);NumericBig next=nb_add(d.magnitude,nb_u64((unsigned)(*p-'0')));overflow|=nb_compare(next,d.magnitude)<0;d.magnitude=next;
        digits++;d.scale+=point;p++;
    }
    if(!digits)return 13;
    if(*p=='e'||*p=='E'||*p=='d'||*p=='D') {
        p++;int negative=0,exponent=0;
        if(*p=='-'||*p=='+')negative=*p++=='-';
        if(*p<'0'||*p>'9')return 13;
        while(*p>='0'&&*p<='9'){if(exponent<10000)exponent=exponent*10+(*p-'0');p++;}
        if(exponent>10000)exponent=10000;
        d.scale+=negative?exponent:-exponent;
    }
    while(*p==' '||*p=='\t'||*p=='\r'||*p=='\n')p++;
    if(*p)return 13;
    if(overflow)return 6;
    while(d.scale<0){if(nb_mul_small(&d.magnitude,10))return 6;d.scale++;}
    *out=d;return 0;
}
static int numeric_decimal(NumericDecimal *out,const VbaVariant *v) {
    NumericDecimal d={0};int64_t signed_value=0;
    switch(v->vt) {
    case V_DECIMAL:
        d.magnitude=nb_u64(v->decimal.lo);d.magnitude.w[2]=v->decimal.hi;
        d.scale=v->decimal.scale;d.negative=v->decimal.sign!=0;break;
    case V_CY:signed_value=v->value.i8;d.scale=4;goto integral_value;
    case V_EMPTY:break;
    case V_UI1:d.magnitude=nb_u64(v->value.byte);break;
    case V_BOOL:signed_value=v->value.boolean?-1:0;goto integral_value;
    case V_I2:signed_value=v->value.i2;goto integral_value;
    case V_I4:signed_value=v->value.i4;
integral_value:
        d.negative=signed_value<0;d.magnitude=nb_u64(d.negative?UINT64_C(0)-(uint64_t)signed_value:(uint64_t)signed_value);break;
    case V_R4:case V_R8:case V_DATE: {
        double n=v->vt==V_R4?v->value.r4:v->value.r8;if(!isfinite(n))return 6;
        char ascii[64];int length=snprintf(ascii,sizeof(ascii),v->vt==V_R4?"%.7g":"%.15g",n);
        uint16_t units[64];for(int i=0;i<length;i++)units[i]=(unsigned char)ascii[i];
        return numeric_parse(out,units,(uint32_t)length);
    }
    case V_BSTR:return numeric_parse(out,v->value.pointer,vba_internal_bstr_len(v->value.pointer));
    case V_NULL:return 94;
    default:return 13;
    }
    *out=d;return 0;
}
static int numeric_format(char *out,NumericDecimal d) {
    if(nb_zero(d.magnitude))d.negative=0;
    char reverse[80];int count=0;
    do {reverse[count++]=(char)('0'+nb_div_small(&d.magnitude,10));}while(!nb_zero(d.magnitude));
    int used=0;if(d.negative)out[used++]='-';
    if(count<=d.scale){out[used++]='0';out[used++]='.';for(int i=count;i<d.scale;i++)out[used++]='0';}
    for(int i=count-1;i>=0;i--){out[used++]=reverse[i];if(i==d.scale && d.scale && count>d.scale)out[used++]='.';}
    if(d.scale){while(used && out[used-1]=='0')used--;if(used && out[used-1]=='.')used--;}
    if(used==0 || (used==1&&out[0]=='-'))out[used++]='0';out[used]=0;return used;
}
static int numeric_bstr(VbaVariant *out,const char *text) {
    size_t n=strlen(text);uint16_t *b=vba_internal_bstr_alloc(NULL,(uint32_t)n);if(!b)return 14;
    for(size_t i=0;i<n;i++)b[i]=(unsigned char)text[i];
    *out=(VbaVariant){.vt=V_BSTR,.value.pointer=b};return 0;
}
static int numeric_currency(VbaVariant *out,const VbaVariant *v) {
    NumericDecimal d={0};int error=numeric_decimal(&d,v);if(error)return error;
    if(v->vt==V_R4 || v->vt==V_R8 || v->vt==V_DATE) {
        /* Exact binary input times 10,000, rounded once using integer bits. */
        double n=v->vt==V_R4?v->value.r4:v->value.r8;uint64_t bits;memcpy(&bits,&n,8);
        int exponent=(int)((bits>>52)&2047)-1023-52;
        uint64_t fraction=bits&UINT64_C(0xfffffffffffff);if((bits>>52)&2047)fraction|=UINT64_C(1)<<52;else exponent=-1074;
        unsigned __int128 magnitude=(unsigned __int128)fraction*10000;
        if(exponent>=0){if(exponent>60 || magnitude>((unsigned __int128)UINT64_MAX>>exponent))return 6;magnitude<<=exponent;}
        else {int shift=-exponent;if(shift>=128)magnitude=0;else {unsigned __int128 quotient=magnitude>>shift,mask=(((unsigned __int128)1)<<shift)-1,rem=magnitude&mask,half=((unsigned __int128)1)<<(shift-1);magnitude=quotient+(rem>half || (rem==half && (quotient&1)));}}
        if(magnitude>(unsigned __int128)INT64_MAX+(n<0))return 6;
        uint64_t raw=(uint64_t)magnitude;if(n<0)raw=UINT64_C(0)-raw;
        VbaVariant result={.vt=V_CY};memcpy(&result.value.i8,&raw,8);*out=result;return 0;
    }
    while(d.scale<4){if(nb_mul_small(&d.magnitude,10))return 6;d.scale++;}
    int digit=0,sticky=0;d.magnitude=nb_round_scale(d.magnitude,&d.scale,4,&digit,&sticky);
    if(digit>5 || (digit==5 && (sticky || (d.magnitude.w[0]&1))))d.magnitude=nb_add(d.magnitude,nb_u64(1));
    uint64_t limit=(uint64_t)INT64_MAX+(unsigned)d.negative;
    if(!nb_fits64(d.magnitude) || nb_low64(d.magnitude)>limit)return 6;
    uint64_t raw=nb_low64(d.magnitude);if(d.negative)raw=UINT64_C(0)-raw;
    VbaVariant result={.vt=V_CY};memcpy(&result.value.i8,&raw,8);*out=result;return 0;
}
int vba_internal_date_valid(double n) {return isfinite(n) && n>-657435.0 && n<2958466.0;}
static int date_leap(int y){return !(y%4) && (y%100 || !(y%400));}
static int64_t date_year_days(int y){int64_t n=y-1;return n*365+n/4-n/100+n/400;}
static const int date_months[]={31,28,31,30,31,30,31,31,30,31,30,31};
static int date_parse(double *out,const uint16_t *units,uint32_t n) {
    if(!n || n>128)return 13;char text[129];
    for(uint32_t i=0;i<n;i++){if(units[i]>127 || !units[i])return 13;text[i]=(char)units[i];}text[n]=0;
    int y=1899,m=12,d=30,h=0,minute=0,s=0,used=0,date=0;
    if(n>=10 && text[4]=='-' && text[7]=='-') {
        if(sscanf(text,"%4d-%2d-%2d%n",&y,&m,&d,&used)!=3)return 13;date=1;
    }
    char *p=text+used;if(*p=='T'||*p==' ')p++;
    if(*p) {int count=0;if(sscanf(p,"%2d:%2d:%2d%n",&h,&minute,&s,&count)!=3)return 13;p+=count;}
    while(*p==' '||*p=='\t')p++;
    if(*p || (!date && !used && p==text) || y<100 || y>9999 || m<1 || m>12 || d<1 || d>date_months[m-1]+(m==2&&date_leap(y)) || h<0 || h>23 || minute<0 || minute>59 || s<0 || s>59)return 13;
    int64_t days=date_year_days(y)+d-1;
    for(int i=1;i<m;i++)days+=date_months[i-1]+(i==2&&date_leap(y));
    int64_t epoch=date_year_days(1899)+363;
    double fraction=(h*3600+minute*60+s)/86400.0;
    *out=(double)(days-epoch)+(days<epoch?-fraction:fraction);return 0;
}
static int date_format(char *out,double n) {
    int64_t days=(int64_t)trunc(n)+date_year_days(1899)+363;
    int y=100;while(y<9999 && date_year_days(y+1)<=days)y++;
    int day=(int)(days-date_year_days(y)),m=1;
    while(m<12 && day>=date_months[m-1]+(m==2&&date_leap(y))){day-=date_months[m-1]+(m==2&&date_leap(y));m++;}
    int seconds=(int)floor(fabs(n-trunc(n))*86400.0+0.00001);if(seconds>86399)seconds=86399;
    return snprintf(out,80,"%04d-%02d-%02d %02d:%02d:%02d",y,m,day+1,seconds/3600,(seconds/60)%60,seconds%60);
}
int vba_internal_numeric_fixed_convert(VbaVariant *out,const VbaVariant *v,uint16_t tag) {
    if(tag==V_DATE) {
        double n=0;VbaVariant number_value={0};
        if(v->vt==V_BSTR && !date_parse(&n,v->value.pointer,vba_internal_bstr_len(v->value.pointer))) {}
        else {int error=vba_internal_convert(&number_value,v,V_R8);if(error)return error;n=number_value.value.r8;}
        if(!vba_internal_date_valid(n))return v->vt==V_BSTR?13:6;
        *out=(VbaVariant){.vt=V_DATE,.value.r8=n};return 0;
    }
    if(tag==V_CY)return numeric_currency(out,v);
    if(v->vt==V_DATE && tag!=V_DECIMAL && tag!=V_BSTR) {VbaVariant n=*v;n.vt=V_R8;return vba_internal_convert(out,&n,tag);}
    NumericDecimal d={0};int error=numeric_decimal(&d,v);if(error)return error;
    if(tag==V_DECIMAL)return decimal_finish(out,d,0,0);
    char text[100];
    if(tag==V_BSTR){if(v->vt==V_DATE)date_format(text,v->value.r8);else numeric_format(text,d);return numeric_bstr(out,text);}
    if(tag==V_R4 || tag==V_R8){numeric_format(text,d);VbaVariant result={.vt=tag};if(tag==V_R4){result.value.r4=strtof(text,NULL);if(!isfinite(result.value.r4))return 6;}else result.value.r8=strtod(text,NULL);*out=result;return 0;}
    if(tag==V_BOOL){*out=(VbaVariant){.vt=V_BOOL,.value.boolean=nb_zero(d.magnitude)?0:-1};return 0;}
    int digit=0,sticky=0;d.magnitude=nb_round_scale(d.magnitude,&d.scale,0,&digit,&sticky);
    if(digit>5 || (digit==5 && (sticky || (d.magnitude.w[0]&1))))d.magnitude=nb_add(d.magnitude,nb_u64(1));
    if(!nb_fits64(d.magnitude))return 6;
    uint64_t magnitude=nb_low64(d.magnitude),limit=tag==V_UI1?255:tag==V_I2?(uint64_t)INT16_MAX+(unsigned)d.negative:tag==V_I4?(uint64_t)INT32_MAX+(unsigned)d.negative:0;
    if((tag==V_UI1 && d.negative && magnitude) || magnitude>limit)return 6;
    int64_t n=d.negative?-(int64_t)magnitude:(int64_t)magnitude;VbaVariant result={.vt=tag};
    if(tag==V_UI1)result.value.byte=(uint8_t)n;else if(tag==V_I2)result.value.i2=(int16_t)n;else if(tag==V_I4)result.value.i4=(int32_t)n;else return 13;
    *out=result;return 0;
}
int vba_internal_numeric_binary(VbaVariant *out,const VbaVariant *a,const VbaVariant *b,int op) {
    if(a->vt==V_DECIMAL || b->vt==V_DECIMAL) {
        NumericDecimal x={0},y={0};VbaVariant ax={0},by={0};int error=vba_internal_convert(&ax,a,V_DECIMAL);if(error)return error;error=vba_internal_convert(&by,b,V_DECIMAL);if(error)return error;
        numeric_decimal(&x,&ax);numeric_decimal(&y,&by);
        NumericDecimal r={0};int digit=0,sticky=0;
        if(op==3){r.magnitude=nb_multiply(x.magnitude,y.magnitude);r.scale=x.scale+y.scale;r.negative=x.negative!=y.negative;}
        else if(op==5) {
            if(nb_zero(y.magnitude))return 11;
            r.scale=28;r.negative=x.negative!=y.negative;
            int exponent=r.scale+y.scale-x.scale;NumericBig numerator=x.magnitude,denominator=y.magnitude;
            while(exponent<0){if(nb_mul_small(&denominator,10))return 6;exponent++;}
            while(exponent>0){NumericBig next=numerator;if(nb_mul_small(&next,10)){r.scale--;exponent--;if(r.scale<0)return 6;}else {numerator=next;exponent--;}}
            NumericBig rem={0};nb_divide(numerator,denominator,&r.magnitude,&rem);
            nb_mul_small(&rem,10);NumericBig q={0},tail={0};nb_divide(rem,denominator,&q,&tail);digit=(int)q.w[0];sticky=!nb_zero(tail);
        } else if(op==1 || op==2) {
            r.scale=x.scale>y.scale?x.scale:y.scale;
            while(x.scale<r.scale){nb_mul_small(&x.magnitude,10);x.scale++;}
            while(y.scale<r.scale){nb_mul_small(&y.magnitude,10);y.scale++;}
            if(op==2)y.negative=!y.negative;
            if(x.negative==y.negative){r.magnitude=nb_add(x.magnitude,y.magnitude);r.negative=x.negative;}
            else if(nb_compare(x.magnitude,y.magnitude)>=0){r.magnitude=nb_sub(x.magnitude,y.magnitude);r.negative=x.negative;}
            else {r.magnitude=nb_sub(y.magnitude,x.magnitude);r.negative=y.negative;}
        } else return 13;
        return decimal_finish(out,r,digit,sticky);
    }
    if(a->vt==V_DATE || b->vt==V_DATE) {
        VbaVariant x={0},y={0};int e=vba_internal_convert(&x,a,V_R8);if(e)return e;e=vba_internal_convert(&y,b,V_R8);if(e)return e;
        double n=op==1?x.value.r8+y.value.r8:op==2?x.value.r8-y.value.r8:op==3?x.value.r8*y.value.r8:op==5?x.value.r8/y.value.r8:NAN;
        if(op==5 && y.value.r8==0)return x.value.r8==0 && b->vt!=V_EMPTY?6:11;if(!isfinite(n))return 6;
        int date=(op==1 || op==2 || op==3) && !(op==2 && a->vt==V_DATE && b->vt==V_DATE);
        *out=(VbaVariant){.vt=date && vba_internal_date_valid(n)?V_DATE:V_R8,.value.r8=n};return 0;
    }
    VbaVariant x={0},y={0};int e=vba_internal_convert(&x,a,V_CY);if(e)return e;e=vba_internal_convert(&y,b,V_CY);if(e)return e;
    __int128 n=0;
    if(op==1)n=(__int128)x.value.i8+y.value.i8;
    else if(op==2)n=(__int128)x.value.i8-y.value.i8;
    else if(op==3) {
        __int128 product=(__int128)x.value.i8*y.value.i8;int negative=product<0;unsigned __int128 mag=negative?(unsigned __int128)(-product):(unsigned __int128)product;
        unsigned __int128 quotient=mag/10000,rem=mag%10000;quotient+=rem>5000 || (rem==5000 && (quotient&1));n=negative?-(__int128)quotient:(__int128)quotient;
    } else return 13;
    if(n<INT64_MIN || n>INT64_MAX)return 6;*out=(VbaVariant){.vt=V_CY,.value.i8=(int64_t)n};return 0;
}
int vba_internal_numeric_compare(const VbaVariant *a,const VbaVariant *b,int *comparison) {
    if((a->vt==V_DATE || b->vt==V_DATE) && a->vt!=V_DECIMAL && b->vt!=V_DECIMAL) {
        VbaVariant x={0},y={0};int e=vba_internal_convert(&x,a,V_R8);if(e)return e;e=vba_internal_convert(&y,b,V_R8);if(e)return e;
        if(!isfinite(x.value.r8) || !isfinite(y.value.r8))return 6;
        *comparison=x.value.r8<y.value.r8?-1:x.value.r8>y.value.r8?1:0;return 0;
    }
    uint16_t tag=a->vt==V_DECIMAL || b->vt==V_DECIMAL?V_DECIMAL:V_CY;
    VbaVariant ax={0},by={0};int e=vba_internal_convert(&ax,a,tag);if(e)return e;e=vba_internal_convert(&by,b,tag);if(e)return e;
    NumericDecimal x={0},y={0};numeric_decimal(&x,&ax);numeric_decimal(&y,&by);
    int scale=x.scale>y.scale?x.scale:y.scale;
    while(x.scale<scale){if(nb_mul_small(&x.magnitude,10))return 6;x.scale++;}
    while(y.scale<scale){if(nb_mul_small(&y.magnitude,10))return 6;y.scale++;}
    if(nb_zero(x.magnitude))x.negative=0;if(nb_zero(y.magnitude))y.negative=0;
    *comparison=x.negative!=y.negative?(x.negative?-1:1):nb_compare(x.magnitude,y.magnitude)*(x.negative?-1:1);return 0;
}
