#ifndef VBA_COLLECTION_H
#define VBA_COLLECTION_H
#include "objects.h"
#include "control.h"
int32_t vba_collection_new(VbaObject **out);
void vba_collection_dispose(VbaObject *object);
int32_t vba_collection_invoke(void *out,VbaObject *object,int32_t method,int32_t count,const VbaObjectArg *args,int32_t mode);
int32_t vba_collection_enum_start(VbaEnumerator *out,VbaObject *object);
int32_t vba_collection_enum_next(int16_t *out,VbaEnumerator *state,VbaVariant *counter);
int32_t vba_collection_enum_object_next(int16_t *out,VbaEnumerator *state,VbaObject **counter,int32_t kind);
void vba_collection_enum_clear(VbaEnumerator *state);
#endif
