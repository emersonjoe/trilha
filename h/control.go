package h

// If renders n only when cond is true.
func If(cond bool, n Node) Node {
	if cond {
		return n
	}
	return Nil
}

// IfElse renders a when cond is true, b otherwise.
func IfElse(cond bool, a, b Node) Node {
	if cond {
		return a
	}
	return b
}

// Map renders f(item) for each item, in order.
func Map[T any](items []T, f func(T) Node) Node {
	out := make(fragment, 0, len(items))
	for _, it := range items {
		out = append(out, f(it))
	}
	return out
}

// MapIndex is Map with the index available to f.
func MapIndex[T any](items []T, f func(int, T) Node) Node {
	out := make(fragment, 0, len(items))
	for i, it := range items {
		out = append(out, f(i, it))
	}
	return out
}
