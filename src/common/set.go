package common

type Set[T comparable] struct {
	set map[T]any
}

func NewSet[T comparable]() Set[T] {
	return Set[T]{
		set: make(map[T]any),
	}
}

func (set *Set[T]) Add(item T) {
	set.set[item] = struct{}{}
}

func (set *Set[T]) Remove(item T) {
	delete(set.set, item)
}

func (set *Set[T]) Has(item T) bool {
	_, ok := set.set[item]
	return ok
}
