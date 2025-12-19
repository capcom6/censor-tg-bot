package plugin

import (
	"fmt"

	"github.com/samber/lo"
)

func SliceFromAnyOrDefault[T any](params map[string]any, key string, defaultValue []T) ([]T, error) {
	v, ok := params[key]
	if !ok {
		return defaultValue, nil
	}

	vSlice, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s must be a slice", ErrInvalidConfig, key)
	}

	vTyped, ok := lo.FromAnySlice[T](vSlice)
	if !ok {
		return nil, fmt.Errorf("%w: invalid type of items in %s", ErrInvalidConfig, key)
	}

	return vTyped, nil
}
