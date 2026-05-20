package utils

import (
	"errors"
	"fmt"
	"reflect"
)

// TypeMismatchError reports an attempt to merge two structs of different
// concrete types. Surfaced as a typed error (errors.Is / errors.As
// friendly) so callers can distinguish a programming bug from a content
// error.
type TypeMismatchError struct {
	Want, Got reflect.Type
}

func (e *TypeMismatchError) Error() string {
	return fmt.Sprintf("merge type mismatch: want %v, got %v", e.Want, e.Got)
}

// ErrMergeNilDestination is returned when the destination passed to
// MergeStructs is nil or not a pointer.
var ErrMergeNilDestination = errors.New("merge destination must be a non-nil pointer to a struct")

// MergeStructs merges multiple structs of the same type, with later
// values taking precedence. Only non-zero values from later structs
// override earlier values. Pre-validates the destination and each source
// type so that a programming bug surfaces as a typed error instead of a
// reflect panic. A last-ditch recover preserves the error chain via %w
// when something exotic still panics.
//
// Rationale (deep-review-hardening-plan, T2.1): the previous form
// `recover() { err = fmt.Errorf("config merge failed: %v", r) }` violated
// Rule 3 (no %w) and Rule 8 (don't swallow exceptions) of CLAUDE.md.
func MergeStructs(dst interface{}, srcs ...interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = fmt.Errorf("merge panic: %w", e)
				return
			}
			err = fmt.Errorf("merge panic: %w", fmt.Errorf("%v", r))
		}
	}()

	if dst == nil {
		return ErrMergeNilDestination
	}
	dstType := reflect.TypeOf(dst)
	if dstType.Kind() != reflect.Pointer || dstType.Elem().Kind() != reflect.Struct {
		return ErrMergeNilDestination
	}
	dstElem := dstType.Elem()

	for _, src := range srcs {
		if src == nil {
			continue
		}
		srcType := reflect.TypeOf(src)
		if srcType.Kind() == reflect.Pointer {
			srcType = srcType.Elem()
		}
		if srcType != dstElem {
			return &TypeMismatchError{Want: dstElem, Got: srcType}
		}
		mergeStruct(dst, src)
	}
	return nil
}

func mergeStruct(dst, src interface{}) {
	dstValue := reflect.ValueOf(dst).Elem()
	srcValue := reflect.ValueOf(src)

	if srcValue.Kind() == reflect.Pointer {
		srcValue = srcValue.Elem()
	}

	if srcValue.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < srcValue.NumField(); i++ {
		srcField := srcValue.Field(i)
		dstField := dstValue.Field(i)

		switch srcField.Kind() {
		case reflect.Map:
			mergeMap(dstField, srcField)

		case reflect.Slice:
			if !srcField.IsNil() {
				dstField.Set(srcField)
			}

		case reflect.Pointer:
			mergePtr(dstField, srcField)

		case reflect.Struct:
			mergeStruct(dstField.Addr().Interface(), srcField.Interface())

		default:
			if !isZeroValue(srcField) {
				dstField.Set(srcField)
			}
		}
	}
}

func mergeMap(dst, src reflect.Value) {
	if src.IsNil() {
		return
	}

	if dst.IsNil() {
		dst.Set(reflect.MakeMap(src.Type()))
	}

	for _, key := range src.MapKeys() {
		srcValue := src.MapIndex(key)
		dstValue := dst.MapIndex(key)

		if srcValue.Kind() == reflect.Pointer && srcValue.Elem().Kind() == reflect.Struct {
			if !dstValue.IsValid() {
				dst.SetMapIndex(key, srcValue)
			} else {
				mergeStruct(dstValue.Interface(), srcValue.Interface())
			}
			continue
		}

		dst.SetMapIndex(key, srcValue)
	}
}

func mergePtr(dst, src reflect.Value) {
	if src.IsNil() {
		return
	}

	switch src.Elem().Kind() {
	case reflect.Struct:
		if dst.IsNil() {
			dst.Set(reflect.New(src.Elem().Type()))
		}
		mergeStruct(dst.Interface(), src.Interface())

	case reflect.Slice:
		if dst.IsNil() || !src.Elem().IsNil() {
			dst.Set(src)
		}

	default:
		dst.Set(src)
	}
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	}
	return false
}
