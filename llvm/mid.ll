; Mid reads produce owned views (or inline units); writes detach shared/literal
; storage. Lengths and positions count UTF-16 units, including isolated surrogates.
define internal i32 @vba.str.min(i32 %a, i32 %b) alwaysinline {
  %less = icmp ult i32 %a, %b
  %value = select i1 %less, i32 %a, i32 %b
  ret i32 %value
}

define internal i32 @vba.str.mid(ptr %output, %vba.string %s, i32 %start, i32 %length) {
entry:
  %bad.start = icmp sle i32 %start, 0
  %bad.length = icmp slt i32 %length, 0
  %invalid = or i1 %bad.start, %bad.length
  br i1 %invalid, label %failure, label %bounds
bounds:
  %n = call i32 @vba.str.len(%vba.string %s)
  %past = icmp sgt i32 %start, %n
  br i1 %past, label %empty, label %clip
clip:
  %offset = sub i32 %start, 1
  %remaining = sub i32 %n, %offset
  %count = call i32 @vba.str.min(i32 %remaining, i32 %length)
  %zero = icmp eq i32 %count, 0
  br i1 %zero, label %empty, label %single.test
single.test:
  %single = icmp eq i32 %count, 1
  br i1 %single, label %one, label %view
one:
  %unit = call i16 @vba.str.unit(%vba.string %s, i32 %offset)
  %character = insertvalue %vba.string {ptr null, ptr null, i32 1, i16 0}, i16 %unit, 3
  store %vba.string %character, ptr %output, align 1
  ret i32 0
view:
  %data = extractvalue %vba.string %s, 0
  %begin = getelementptr i16, ptr %data, i32 %offset
  %v0 = insertvalue %vba.string %s, ptr %begin, 0
  %v1 = insertvalue %vba.string %v0, i32 %count, 2
  call void @vba.str.retain(%vba.string %v1)
  store %vba.string %v1, ptr %output, align 1
  ret i32 0
empty:
  store %vba.string zeroinitializer, ptr %output, align 1
  ret i32 0
failure:
  ret i32 5
}

; The caller guarantees a valid length greater than one. Output is untouched on
; allocation failure; copying source units cannot raise a VBA error.
define internal i32 @vba.str.copy(ptr %output, %vba.string %s) {
entry:
  %n = call i32 @vba.str.len(%vba.string %s)
  %wide = zext i32 %n to i64
  %terminated = add i64 %wide, 1
  %bytes = mul i64 %terminated, 2
  %size = add i64 %bytes, 8
  %owner = call ptr @malloc(i64 %size)
  %missing = icmp eq ptr %owner, null
  br i1 %missing, label %failure, label %initialize
initialize:
  store i64 1, ptr %owner, align 1
  %data = getelementptr i8, ptr %owner, i64 8
  br label %copy
copy:
  %i = phi i32 [0, %initialize], [%next, %copy]
  %unit = call i16 @vba.str.unit(%vba.string %s, i32 %i)
  %destination = getelementptr i16, ptr %data, i32 %i
  store i16 %unit, ptr %destination, align 1
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
  ret i32 14
}

define internal i32 @vba.str.midcheck(ptr %target, i32 %start, i32 %length) alwaysinline {
entry:
  %s = load %vba.string, ptr %target, align 1
  %n = call i32 @vba.str.len(%vba.string %s)
  %low = icmp sle i32 %start, 0
  %high = icmp sgt i32 %start, %n
  %negative = icmp slt i32 %length, 0
  %bad.start = or i1 %low, %high
  %bad = or i1 %bad.start, %negative
  %status = select i1 %bad, i32 5, i32 0
  ret i32 %status
}

define internal i32 @vba.str.midassign(ptr %target, i32 %start, i32 %length, %vba.string %replacement) {
entry:
  %copied.slot = alloca %vba.string
  ; The RHS may have changed target since the initial check. Never use a stale
  ; pointer or write beyond its current length. No write happens before validation.
  %status = call i32 @vba.str.midcheck(ptr %target, i32 %start, i32 %length)
  %bad = icmp ne i32 %status, 0
  br i1 %bad, label %invalid, label %bounds
bounds:
  %current = load %vba.string, ptr %target, align 1
  %n = call i32 @vba.str.len(%vba.string %current)
  %rn = call i32 @vba.str.len(%vba.string %replacement)
  %offset = sub i32 %start, 1
  %remaining = sub i32 %n, %offset
  %limit = call i32 @vba.str.min(i32 %length, i32 %rn)
  %count = call i32 @vba.str.min(i32 %limit, i32 %remaining)
  %empty = icmp eq i32 %count, 0
  br i1 %empty, label %done, label %single.test
single.test:
  %single = icmp eq i32 %n, 1
  br i1 %single, label %one, label %owner.test
one:
  %unit = call i16 @vba.str.unit(%vba.string %replacement, i32 0)
  %current.length = extractvalue %vba.string %current, 2
  %foreign.single = icmp sle i32 %current.length, -2
  br i1 %foreign.single, label %foreign.one, label %private.one
foreign.one:
  %bstr = call ptr @vba.str.data(%vba.string %current)
  store i16 %unit, ptr %bstr, align 1
  ret i32 0
private.one:
  %value = insertvalue %vba.string {ptr null, ptr null, i32 1, i16 0}, i16 %unit, 3
  store %vba.string %value, ptr %target, align 1
  call void @vba.str.drop(%vba.string %current)
  ret i32 0
owner.test:
  %storage.length = extractvalue %vba.string %current, 2
  %foreign = icmp sle i32 %storage.length, -2
  br i1 %foreign, label %writable, label %owner.private
owner.private:
  %owner = extractvalue %vba.string %current, 1
  %managed = icmp ne ptr %owner, null
  br i1 %managed, label %references, label %detach
references:
  %refcount = load i64, ptr %owner, align 1
  %unique = icmp eq i64 %refcount, 1
  br i1 %unique, label %writable, label %detach
detach:
  %copy.status = call i32 @vba.str.copy(ptr %copied.slot, %vba.string %current)
  %copy.failed = icmp ne i32 %copy.status, 0
  br i1 %copy.failed, label %allocation.failure, label %copied
copied:
  %copy.value = load %vba.string, ptr %copied.slot, align 1
  br label %writable
writable:
  %working = phi %vba.string [%current, %references], [%copy.value, %copied], [%current, %owner.test]
  %detached = phi i1 [false, %references], [true, %copied], [false, %owner.test]
  %data = call ptr @vba.str.data(%vba.string %working)
  br label %write
write:
  %i = phi i32 [0, %writable], [%next, %write]
  %source = call i16 @vba.str.unit(%vba.string %replacement, i32 %i)
  %index = add i32 %offset, %i
  %destination = getelementptr i16, ptr %data, i32 %index
  store i16 %source, ptr %destination, align 1
  %next = add i32 %i, 1
  %more = icmp ult i32 %next, %count
  br i1 %more, label %write, label %commit
commit:
  store %vba.string %working, ptr %target, align 1
  br i1 %detached, label %release, label %done
release:
  call void @vba.str.drop(%vba.string %current)
  br label %done
done:
  ret i32 0
invalid:
  ret i32 5
allocation.failure:
  ret i32 %copy.status
}
