; Private String runtime. Mutable variables hold value descriptors; buffers may
; be shared until Mid assignment detaches them before writing. Raw pointer
; boundaries remain unsupported. A ByRef aliases the descriptor's slot.
; {data, owner, length, inline unit}; length -1 preserves a null String.
%vba.string = type { ptr, ptr, i32, i16 }

declare noalias ptr @malloc(i64) allocsize(0) allockind("alloc,uninitialized") "alloc-family"="malloc"
declare void @free(ptr allocptr) allockind("free") "alloc-family"="malloc"

; A borrowed String array element stores a BSTR* rather than a BSTR in data.
define internal ptr @vba.str.data(%vba.string %s) alwaysinline {
entry:
  %length = extractvalue %vba.string %s, 2
  %data = extractvalue %vba.string %s, 0
  %borrowed = icmp eq i32 %length, -3
  br i1 %borrowed, label %indirect, label %direct
indirect:
  %bstr = load ptr, ptr %data, align 1
  ret ptr %bstr
direct:
  ret ptr %data
}

define internal i32 @vba.str.len(%vba.string %s) alwaysinline {
entry:
  %n = extractvalue %vba.string %s, 2
  %foreign = icmp sle i32 %n, -2
  br i1 %foreign, label %bstr, label %private
bstr:
  %data = call ptr @vba.str.data(%vba.string %s)
  %missing = icmp eq ptr %data, null
  br i1 %missing, label %null, label %prefix
null:
  ret i32 0
prefix:
  %p = getelementptr i8, ptr %data, i64 -4
  %bytes = load i32, ptr %p, align 1
  %units = lshr i32 %bytes, 1
  ret i32 %units
private:
  %positive = icmp sgt i32 %n, 0
  %length = select i1 %positive, i32 %n, i32 0
  ret i32 %length
}

define internal i16 @vba.str.unit(%vba.string %s, i32 %index) alwaysinline {
entry:
  %n = extractvalue %vba.string %s, 2
  %single = icmp eq i32 %n, 1
  br i1 %single, label %inline, label %buffer
inline:
  %unit = extractvalue %vba.string %s, 3
  ret i16 %unit
buffer:
  %data = call ptr @vba.str.data(%vba.string %s)
  %p = getelementptr i16, ptr %data, i32 %index
  %value = load i16, ptr %p, align 1
  ret i16 %value
}

define internal void @vba.str.retain(%vba.string %s) alwaysinline {
entry:
  %owner = extractvalue %vba.string %s, 1
  %managed = icmp ne ptr %owner, null
  br i1 %managed, label %acquire, label %done
acquire:
  %old = load i64, ptr %owner, align 1
  %new = add i64 %old, 1
  store i64 %new, ptr %owner, align 1
  br label %done
done:
  ret void
}

define internal void @vba.str.drop(%vba.string %s) alwaysinline {
entry:
  %length = extractvalue %vba.string %s, 2
  %foreign = icmp eq i32 %length, -2
  br i1 %foreign, label %bstr, label %private
bstr:
  %data = extractvalue %vba.string %s, 0
  call void @vba.str.foreign.drop(ptr %data)
  ret void
private:
  %owner = extractvalue %vba.string %s, 1
  %managed = icmp ne ptr %owner, null
  br i1 %managed, label %release, label %done
release:
  %old = load i64, ptr %owner, align 1
  %new = sub i64 %old, 1
  store i64 %new, ptr %owner, align 1
  %last = icmp eq i64 %new, 0
  br i1 %last, label %destroy, label %done
destroy:
  call void @free(ptr %owner)
  br label %done
done:
  ret void
}

define internal void @vba.str.assign(ptr %slot, %vba.string %value) alwaysinline {
entry:
  ; Acquire before release: self-assignment and aliases are safe.
  call void @vba.str.retain(%vba.string %value)
  %old = load %vba.string, ptr %slot, align 1
  store %vba.string %value, ptr %slot, align 1
  call void @vba.str.drop(%vba.string %old)
  ret void
}

define internal void @vba.str.clear(ptr %slot) alwaysinline {
entry:
  %old = load %vba.string, ptr %slot, align 1
  store %vba.string { ptr null, ptr null, i32 -1, i16 0 }, ptr %slot, align 1
  call void @vba.str.drop(%vba.string %old)
  ret void
}

define internal i32 @vba.str.concat(ptr %output, %vba.string %a, %vba.string %b) {
entry:
  %na = call i32 @vba.str.len(%vba.string %a)
  %nb = call i32 @vba.str.len(%vba.string %b)
  %wa = zext i32 %na to i64
  %wb = zext i32 %nb to i64
  %sum = add i64 %wa, %wb
  %large = icmp ugt i64 %sum, 2147483647
  br i1 %large, label %failure, label %size
size:
  %n = trunc i64 %sum to i32
  %empty = icmp eq i32 %n, 0
  br i1 %empty, label %zero, label %single.test
zero:
  store %vba.string zeroinitializer, ptr %output, align 1
  ret i32 0
single.test:
  %single = icmp eq i32 %n, 1
  br i1 %single, label %one, label %reuse.test
one:
  %from.a = icmp eq i32 %na, 1
  %a.unit = extractvalue %vba.string %a, 3
  %b.unit = extractvalue %vba.string %b, 3
  %unit = select i1 %from.a, i16 %a.unit, i16 %b.unit
  %one.value = insertvalue %vba.string { ptr null, ptr null, i32 1, i16 0 }, i16 %unit, 3
  store %vba.string %one.value, ptr %output, align 1
  ret i32 0
reuse.test:
  %a.empty = icmp eq i32 %na, 0
  %b.empty = icmp eq i32 %nb, 0
  %reuse = or i1 %a.empty, %b.empty
  br i1 %reuse, label %shared, label %allocate
shared:
  %existing = select i1 %a.empty, %vba.string %b, %vba.string %a
  call void @vba.str.retain(%vba.string %existing)
  store %vba.string %existing, ptr %output, align 1
  ret i32 0
allocate:
  %terminated = add i64 %sum, 1
  %bytes = mul i64 %terminated, 2
  %size.bytes = add i64 %bytes, 8
  %owner = call ptr @malloc(i64 %size.bytes)
  %missing = icmp eq ptr %owner, null
  br i1 %missing, label %failure, label %initialize
initialize:
  store i64 1, ptr %owner, align 1
  %data = getelementptr i8, ptr %owner, i64 8
  br label %copy
copy:
  %i = phi i32 [ 0, %initialize ], [ %next, %write ]
  %in.a = icmp ult i32 %i, %na
  br i1 %in.a, label %read.a, label %read.b
read.a:
  %av = call i16 @vba.str.unit(%vba.string %a, i32 %i)
  br label %write
read.b:
  %bi = sub i32 %i, %na
  %bv = call i16 @vba.str.unit(%vba.string %b, i32 %bi)
  br label %write
write:
  %v = phi i16 [ %av, %read.a ], [ %bv, %read.b ]
  %destination = getelementptr i16, ptr %data, i32 %i
  store i16 %v, ptr %destination, align 1
  %next = add i32 %i, 1
  %more = icmp ult i32 %next, %n
  br i1 %more, label %copy, label %complete
complete:
  %terminator = getelementptr i16, ptr %data, i32 %n
  store i16 0, ptr %terminator, align 1
  %r0 = insertvalue %vba.string zeroinitializer, ptr %data, 0
  %r1 = insertvalue %vba.string %r0, ptr %owner, 1
  %r2 = insertvalue %vba.string %r1, i32 %n, 2
  store %vba.string %r2, ptr %output, align 1
  ret i32 0
failure:
  ; Out of string space. Output is untouched; no error dispatcher is involved.
  ret i32 14
}
