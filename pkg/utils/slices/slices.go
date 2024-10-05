package slices

func FirstNotZero[T comparable](slice ...T) T {
	var zero T

	for _, v := range slice {
		if v != zero {
			return v
		}
	}
	return zero
}
