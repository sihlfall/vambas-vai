#include "collection.h"
#include "runtime_internal.h"
#include <stdlib.h>
#include <string.h>


typedef struct VbaCollectionNode VbaCollectionNode;
typedef struct VbaCollectionCursor VbaCollectionCursor;
typedef struct {
    VbaObject object;
    int32_t count;
    VbaCollectionNode *first,*last;
    VbaCollectionCursor *cursors;
} VbaCollection;
struct VbaCollectionNode {
    VbaCollectionNode *previous,*next;
    uint16_t *key;
    int keyed,object_item;
    VbaVariant value;
    VbaObject *object;
};
struct VbaCollectionCursor {
    VbaCollection *collection;
    VbaCollectionNode *next;
    VbaCollectionCursor *previous_cursor,*next_cursor;
    int ended;
};
int32_t vba_collection_new(VbaObject **out) {
    static const VbaObjectType type={.size=sizeof(VbaCollection),.dispose=vba_collection_dispose,.invoke=vba_builtin_invoke};
    return vba_object_allocate(out,&type,O_COLLECTION);
}
static void collection_node_clear(VbaCollectionNode *node) {
    vba_bstr_free(node->key);
    vba_variant_clear(&node->value);vba_object_clear(&node->object);free(node);
}
void vba_collection_dispose(VbaObject *object) {
    VbaCollection *c=(VbaCollection*)object;
    VbaCollectionNode *node=c->first;
    while(node){VbaCollectionNode *next=node->next;collection_node_clear(node);node=next;}
}
/* The current portable profile folds ASCII keys independently of Option Compare.
   Other UTF-16 units remain exact; locale-sensitive Unicode collation is separate. */
static int collection_key_equal(const uint16_t *a,const uint16_t *b) {
    uint32_t n=vba_internal_bstr_len(a);if(n!=vba_internal_bstr_len(b))return 0;
    for(uint32_t i=0;i<n;i++){uint16_t x=a[i],y=b[i];if(x>='A'&&x<='Z')x+=32;if(y>='A'&&y<='Z')y+=32;if(x!=y)return 0;}return 1;
}
static int collection_missing(const VbaVariant *value) {
    VbaVariant v={0};return !vba_internal_borrow(&v,value) && v.vt==V_ERROR && v.value.error==(int32_t)0x80020004;
}
static int collection_find(VbaCollection *c,const VbaVariant *index,VbaCollectionNode **out) {
    VbaVariant v={0};int e=vba_internal_borrow(&v,index);if(e)return e;
    if(v.vt==V_BSTR){for(VbaCollectionNode *n=c->first;n;n=n->next)if(n->keyed&&collection_key_equal(n->key,v.value.pointer)){*out=n;return 0;}return 5;}
    int32_t position=0;e=vba_variant_to(&position,&v,V_I4);if(e)return e;
    if(position<1||position>c->count)return 5;
    VbaCollectionNode *n=c->first;for(int32_t i=1;i<position;i++)n=n->next;*out=n;return 0;
}
static int collection_add(VbaCollection *c,const VbaObjectArg *item,const VbaVariant *key,const VbaVariant *before,const VbaVariant *after) {
    int has_before=!collection_missing(before),has_after=!collection_missing(after);
    if(has_before&&has_after)return 5;
    if(c->count==INT32_MAX)return 6;
    VbaCollectionNode *following=NULL;
    int e=0;
    if(has_before)e=collection_find(c,before,&following);
    if(has_after){VbaCollectionNode *previous=NULL;e=collection_find(c,after,&previous);if(!e)following=previous->next;}
    if(e)return e;
    VbaVariant owned_key={0};int keyed=!collection_missing(key);
    if(keyed){e=vba_internal_convert(&owned_key,key,V_BSTR);if(e)return e;for(VbaCollectionNode *n=c->first;n;n=n->next)if(n->keyed&&collection_key_equal(n->key,owned_key.value.pointer)){vba_variant_clear(&owned_key);return 457;}}
    VbaCollectionNode *node=malloc(sizeof(*node));if(!node){vba_variant_clear(&owned_key);return 7;}
    memset(node,0,sizeof(*node));node->keyed=keyed;node->key=owned_key.value.pointer;owned_key.vt=V_EMPTY;
    if(item->kind==1){node->object_item=1;node->object=*(VbaObject*const*)item->value;vba_object_retain(node->object);}
    else e=vba_variant_load(&node->value,item->value);
    if(e){collection_node_clear(node);return e;}
    node->next=following;node->previous=following?following->previous:c->last;
    if(node->previous)node->previous->next=node;else c->first=node;
    if(following)following->previous=node;else c->last=node;
    for(VbaCollectionCursor *it=c->cursors;it;it=it->next_cursor)if(!it->ended&&it->next==following)it->next=node;
    c->count++;return 0;
}
static int collection_remove(VbaCollection *c,const VbaVariant *index) {
    VbaCollectionNode *node=NULL;int e=collection_find(c,index,&node);if(e)return e;
    for(VbaCollectionCursor *it=c->cursors;it;it=it->next_cursor)if(it->next==node)it->next=node->next;
    if(node->previous)node->previous->next=node->next;else c->first=node->next;
    if(node->next)node->next->previous=node->previous;else c->last=node->previous;
    c->count--;collection_node_clear(node);return 0;
}
static int collection_arg_name(const char *a,const char *b) {
    while(*a&&*b){unsigned char x=(unsigned char)*a++,y=(unsigned char)*b++;if(x>='A'&&x<='Z')x+=32;if(y>='A'&&y<='Z')y+=32;if(x!=y)return 0;}return *a==*b;
}
int32_t vba_collection_invoke(void *out,VbaObject *object,int32_t method,int32_t count,const VbaObjectArg *args,int32_t mode) {
    if(!object)return 91;if(object->kind!=O_COLLECTION)return 438;
    if(method<1||method>4)return 438;
    const char *names[]={"Item","Key","Before","After"};
    int maximum=method==1?4:method==4?0:1,position=0;
    if(method!=1)names[0]="Index";
    const VbaObjectArg *bound[4]={0};
    for(int i=0;i<count;i++){
        int slot=-1;
        if(!*args[i].name)slot=position++;
        else {for(int j=0;j<maximum;j++)if(collection_arg_name(args[i].name,names[j])){slot=j;break;}if(slot<0)return 448;}
        if(slot<0||slot>=maximum||bound[slot])return 450;
        if(args[i].kind!=0&&args[i].kind!=1)return 13;
        if(args[i].kind==1&&(method!=1||slot!=0))return 13;
        bound[slot]=args+i;
    }
    VbaCollection *c=(VbaCollection*)object;
    if(method==4){if(mode==3){memcpy(out,&c->count,4);return 0;}if(mode==12)return vba_variant_box_number(out,V_I4,c->count);return 13;}
    if(!bound[0] || (bound[0]->kind==0 && collection_missing(bound[0]->value)))return 450;
    if(method==1){
        VbaVariant missing={.vt=V_ERROR,.value.error=(int32_t)0x80020004};
        return collection_add(c,bound[0],bound[1]?bound[1]->value:&missing,bound[2]?bound[2]->value:&missing,bound[3]?bound[3]->value:&missing);
    }
    if(method==3)return collection_remove(c,bound[0]->value);
    VbaCollectionNode *node=NULL;int e=collection_find(c,bound[0]->value,&node);if(e)return e;
    if(mode==9){if(!node->object_item)return 13;vba_object_retain(node->object);*(VbaObject**)out=node->object;return 0;}
    if(mode!=12)return 13;
    if(node->object_item)return vba_variant_box_object(out,node->object);
    return vba_variant_load(out,&node->value);
}
int32_t vba_collection_enum_start(VbaEnumerator *out,VbaObject *object) {
    if(!object)return 91;if(object->kind!=O_COLLECTION)return 438;
    VbaCollectionCursor *it=malloc(sizeof(*it));if(!it)return 7;
    memset(it,0,sizeof(*it));it->collection=(VbaCollection*)object;it->next=it->collection->first;
    vba_object_retain(object);vba_enum_clear(out);
    it->next_cursor=it->collection->cursors;
    if(it->next_cursor)it->next_cursor->previous_cursor=it;
    it->collection->cursors=it;
    *out=(VbaEnumerator){.array=(VbaArray*)it,.initialized=2};return 0;
}
void vba_collection_enum_clear(VbaEnumerator *state) {
    VbaCollectionCursor *it=(VbaCollectionCursor*)state->array;
    if(it){VbaObject *owner=&it->collection->object;
        if(it->previous_cursor)it->previous_cursor->next_cursor=it->next_cursor;else it->collection->cursors=it->next_cursor;
        if(it->next_cursor)it->next_cursor->previous_cursor=it->previous_cursor;
        free(it);memset(state,0,sizeof(*state));vba_object_clear(&owner);
    }else memset(state,0,sizeof(*state));
}
int32_t vba_collection_enum_next(int16_t *out,VbaEnumerator *state,VbaVariant *counter) {
    VbaCollectionCursor *it=(VbaCollectionCursor*)state->array;
    VbaCollectionNode *node=it->next;
    if(!node){it->ended=1;*out=0;return 0;}
    if(counter->vt==(V_BYREF|V_PRIVATE_OBJECT) && !node->object_item)return 13;
    VbaVariant object={.vt=V_PRIVATE_OBJECT,.value.pointer=node->object};
    int e=vba_variant_store(counter,node->object_item?&object:&node->value);if(e)return e;
    it->next=node->next;*out=-1;return 0;
}
int32_t vba_collection_enum_object_next(int16_t *out,VbaEnumerator *state,VbaObject **counter,int32_t kind) {
    if(!state->initialized)return 92;if(state->initialized!=2)return 13;
    VbaCollectionCursor *it=(VbaCollectionCursor*)state->array;
    VbaCollectionNode *node=it->next;
    if(!node){it->ended=1;*out=0;return 0;}
    if(!node->object_item)return 13;
    int e=vba_object_check(node->object,kind);if(e)return e;
    vba_object_store(counter,node->object);it->next=node->next;*out=-1;return 0;
}
