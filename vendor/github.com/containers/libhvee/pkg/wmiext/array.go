//go:build windows

package wmiext

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/go-ole/go-ole"
)

func CreateStringArrayVariant(array []string) (ole.VARIANT, error) {
	safeByteArray, err := safeArrayFromStringSlice(array)
	if err != nil {
		return ole.VARIANT{}, err
	}
	arrayVariant := ole.NewVariant(ole.VT_ARRAY|ole.VT_BSTR, int64(uintptr(unsafe.Pointer(safeByteArray))))
	return arrayVariant, nil
}

func CreateNumericArrayVariant(array interface{}, itemType ole.VT) (ole.VARIANT, error) {
	safeArray, err := safeArrayFromNumericSlice(array, itemType)
	if err != nil {
		return ole.VARIANT{}, err
	}
	arrayVariant := ole.NewVariant(ole.VT_ARRAY|itemType, int64(uintptr(unsafe.Pointer(safeArray))))
	return arrayVariant, nil
}

// The following safearray routines are unfortunately not yet exported from go-ole,
// so replicate them for now
func safeArrayCreateVector(variantType ole.VT, lowerBound int32, length uint32) (safearray *ole.SafeArray, err error) {
	ret, _, _ := procSafeArrayCreateVector.Call(
		uintptr(variantType),
		uintptr(lowerBound),
		uintptr(length))

	if ret == 0 { // NULL return value
		err = fmt.Errorf("could not create safe array")
	}
	safearray = (*ole.SafeArray)(unsafe.Pointer(ret)) //nolint:govet
	return
}

func safeArrayFromNumericSlice(slice interface{}, itemType ole.VT) (*ole.SafeArray, error) {
	sliceType := reflect.TypeOf(slice)
	if sliceType.Kind() != reflect.Slice {
		return nil, errors.New("expected a slice converting to safe array")
	}

	val := reflect.ValueOf(slice)

	array, err := safeArrayCreateVector(itemType, 0, uint32(val.Len()))
	if err != nil {
		return nil, err
	}

	// assignable holders used for conversion
	var (
		vui1 uint8
		vui2 uint16
		vui4 uint32
		vui8 uint64
		vi1  int8
		vi2  int16
		vi4  int32
		vi8  int64
	)

	for i := 0; i < val.Len(); i++ {
		var data uintptr
		item := val.Index(i)
		switch itemType {
		case ole.VT_UI1:
			data = convertToUnsafeAddr(item, &vui1)
		case ole.VT_UI2:
			data = convertToUnsafeAddr(item, &vui2)
		case ole.VT_UI4:
			data = convertToUnsafeAddr(item, &vui4)
		case ole.VT_UI8:
			data = convertToUnsafeAddr(item, &vui8)
		case ole.VT_I1:
			data = convertToUnsafeAddr(item, &vi1)
		case ole.VT_I2:
			data = convertToUnsafeAddr(item, &vi2)
		case ole.VT_I4:
			data = convertToUnsafeAddr(item, &vi4)
		case ole.VT_I8:
			data = convertToUnsafeAddr(item, &vi8)
		}

		err = safeArrayPutElement(array, int64(i), data)
		if err != nil {
			_ = safeArrayDestroy(array)
			return nil, err
		}
	}

	return array, nil
}

func convertToUnsafeAddr(src reflect.Value, target interface{}) uintptr {
	val := reflect.ValueOf(target)
	val = val.Elem()
	val.Set(src.Convert(val.Type()))
	return val.UnsafeAddr()
}

func safeArrayDestroy(safearray *ole.SafeArray) (err error) {
	ret, _, _ := procSafeArrayDestroy.Call(uintptr(unsafe.Pointer(safearray)))

	if ret != 0 {
		return NewWmiError(ret)
	}

	return nil
}

func safeArrayPutElement(safearray *ole.SafeArray, index int64, element uintptr) (err error) {

	ret, _, _ := procSafeArrayPutElement.Call(
		uintptr(unsafe.Pointer(safearray)),
		uintptr(unsafe.Pointer(&index)),
		element)

	if ret != 0 {
		return NewWmiError(ret)
	}

	return nil
}

func safeArrayGetElement(safearray *ole.SafeArray, index int64, element unsafe.Pointer) error {

	ret, _, _ := procSafeArrayGetElement.Call(
		uintptr(unsafe.Pointer(safearray)),
		uintptr(unsafe.Pointer(&index)),
		uintptr(element))

	if ret != 0 {
		return NewWmiError(ret)
	}

	return nil
}

func isVariantValConvertible(variant ole.VARIANT) bool {
	if variant.VT == ole.VT_RECORD || variant.VT == ole.VT_VARIANT {
		return false
	}
	return true
}

func safeArrayGetAsVariantVal(safeArray *ole.SafeArray, index int64, variant ole.VARIANT) (int64, error) {
	var block int64

	if !isVariantValConvertible(variant) {
		return 0, fmt.Errorf("numeric call on a non-numeric value: %d", variant.VT)
	}

	if err := safeArrayGetElement(safeArray, index, unsafe.Pointer(&block)); err != nil {
		return 0, err
	}

	switch variant.VT {
	case ole.VT_UI1:
		return int64(uint64(*(*uint8)(unsafe.Pointer(&block)))), nil
	case ole.VT_UI2:
		return int64(uint64(*(*uint16)(unsafe.Pointer(&block)))), nil
	case ole.VT_UI4:
		return int64(uint64(*(*uint32)(unsafe.Pointer(&block)))), nil
	case ole.VT_I1:
		return int64(*(*int8)(unsafe.Pointer(&block))), nil
	case ole.VT_I2:
		return int64(*(*int16)(unsafe.Pointer(&block))), nil
	case ole.VT_I4:
		return int64(*(*int32)(unsafe.Pointer(&block))), nil
	case ole.VT_UI8, ole.VT_I8:
		fallthrough
	case ole.VT_R4, ole.VT_R8:
		fallthrough
	default:
		return block, nil
	}
}

func safeArrayFromStringSlice(slice []string) (*ole.SafeArray, error) {
	array, err := safeArrayCreateVector(ole.VT_BSTR, 0, uint32(len(slice)))

	if err != nil {
		return nil, err
	}

	for i, v := range slice {
		err = safeArrayPutElement(array, int64(i), uintptr(unsafe.Pointer(ole.SysAllocStringLen(v))))
		if err != nil {
			_ = safeArrayDestroy(array)
			return nil, err
		}
	}
	return array, nil
}
