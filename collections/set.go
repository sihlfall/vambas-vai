package collections

// Set follows typescript-go's map-backed set. Its zero value is ready to use.
type Set[T comparable] struct {
	M map[T]struct{}
}

func (s *Set[T]) Has(key T) bool {
	if s == nil {
		return false
	}
	_, ok := s.M[key]
	return ok
}

func (s *Set[T]) Add(key T) {
	if s.M == nil {
		s.M = make(map[T]struct{})
	}
	s.M[key] = struct{}{}
}

func (s *Set[T]) Delete(key T) {
	delete(s.M, key)
}

func (s *Set[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.M)
}

func (s *Set[T]) Keys() map[T]struct{} {
	if s == nil {
		return nil
	}
	return s.M
}

func (s *Set[T]) Clear() {
	if s == nil {
		return
	}
	clear(s.M)
}

// AddIfAbsent returns true if the key was not already present in the set.
func (s *Set[T]) AddIfAbsent(key T) bool {
	if s.Has(key) {
		return false
	}
	s.Add(key)
	return true
}
