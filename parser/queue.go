package parser

type unsigned interface {
	uint8 | uint16 | uint32 | uint64
}

/* NOTE: capacity must be a power of 2 */
type tinyQueueBuffer[T any] interface {
	~[2]T | ~[4]T | ~[8]T | ~[16]T | ~[32]T | ~[64]T | ~[128]T | ~[256]T | ~[512]T |
		~[1024]T | ~[2048]T | ~[4096]T // etc., but not needed
}

type tinyQueue[B tinyQueueBuffer[T], I unsigned, T any] struct {
	buffer B
	first  I
	last   I
}

func (q *tinyQueue[B, I, T]) capacity() I {
	return I(len(q.buffer))
}

func (q *tinyQueue[B, I, T]) enqueueUnchecked(t T) {
	q.buffer[q.last] = t
	q.last = (q.last + 1) & (q.capacity() - 1)
}

func (q *tinyQueue[B, I, T]) dequeueUnchecked() T {
	t := q.buffer[q.first]
	q.first = (q.first + 1) & (q.capacity() - 1)
	return t
}

func (q *tinyQueue[B, I, T]) isEmpty() bool {
	return q.first == q.last
}

func (q *tinyQueue[B, I, T]) n() I {
	nn := q.last - q.first
	if nn < 0 {
		nn += I(len(q.buffer))
	}
	return nn
}

func (q *tinyQueue[B, I, T]) discard() {
	q.last = q.first
}
