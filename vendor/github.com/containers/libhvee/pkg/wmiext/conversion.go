//go:build windows

package wmiext

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
)

var (
	unixEpoch = time.Unix(0, 0)
	zeroTime  = time.Time{}
)

// Automation variants do not follow the OLE rules, instead they use the following mapping:
// sint8	VT_I2	Signed 8-bit integer.
// sint16	VT_I2	Signed 16-bit integer.
// sint32	VT_I4	Signed 32-bit integer.
// sint64	VT_BSTR	Signed 64-bit integer in string form. This type follows hexadecimal or decimal format
//
//	according to the American National Standards Institute (ANSI) C rules.
//
// real32	VT_R4	4-byte floating-point value that follows the Institute of Electrical and Electronics
//
//	Engineers, Inc. (IEEE) standard.
//
// real64	VT_R8	8-byte floating-point value that follows the IEEE standard.
// uint8	VT_UI1	Unsigned 8-bit integer.
// uint16	VT_I4	Unsigned 16-bit integer.
// uint32	VT_I4	Unsigned 32-bit integer.
// uint64	VT_BSTR	Unsigned 64-bit integer in string form. This type follows hexadecimal or decimal format
//
//	according to ANSI C rules.

// NewAutomationVariant returns a new VARIANT com
//
//gocyclo:ignore
func NewAutomationVariant(value interface{}) (ole.VARIANT, error) {
	switch cast := value.(type) {
	case bool:
		if cast {
			return ole.NewVariant(ole.VT_BOOL, 0xffff), nil
		} else {
			return ole.NewVariant(ole.VT_BOOL, 0), nil
		}
	case int8:
		return ole.NewVariant(ole.VT_I2, int64(cast)), nil
	case []int8:
		return CreateNumericArrayVariant(cast, ole.VT_I2)
	case int16:
		return ole.NewVariant(ole.VT_I2, int64(cast)), nil
	case []int16:
		return CreateNumericArrayVariant(cast, ole.VT_I2)
	case int32:
		return ole.NewVariant(ole.VT_I4, int64(cast)), nil
	case []int32:
		return CreateNumericArrayVariant(cast, ole.VT_I4)
	case int64:
		s := fmt.Sprintf("%d", cast)
		return ole.NewVariant(ole.VT_BSTR, int64(uintptr(unsafe.Pointer(ole.SysAllocStringLen(s))))), nil
	case []int64:
		strs := make([]string, len(cast))
		for i, num := range cast {
			strs[i] = fmt.Sprintf("%d", num)
		}
		return CreateStringArrayVariant(strs)
	case float32:
		return ole.NewVariant(ole.VT_R4, int64(math.Float32bits(cast))), nil
	case float64:
		return ole.NewVariant(ole.VT_R8, int64(math.Float64bits(cast))), nil
	case uint8:
		return ole.NewVariant(ole.VT_UI1, int64(cast)), nil
	case []uint8:
		return CreateNumericArrayVariant(cast, ole.VT_UI1)
	case uint16:
		return ole.NewVariant(ole.VT_I4, int64(cast)), nil
	case []uint16:
		return CreateNumericArrayVariant(cast, ole.VT_I4)
	case uint32:
		return ole.NewVariant(ole.VT_I4, int64(cast)), nil
	case []uint32:
		return CreateNumericArrayVariant(cast, ole.VT_I4)
	case uint64:
		s := fmt.Sprintf("%d", cast)
		return ole.NewVariant(ole.VT_BSTR, int64(uintptr(unsafe.Pointer(ole.SysAllocStringLen(s))))), nil
	case []uint64:
		strs := make([]string, len(cast))
		for i, num := range cast {
			strs[i] = fmt.Sprintf("%d", num)
		}
		return CreateStringArrayVariant(strs)

	// Assume 32 bit for generic (u)ints
	case int:
		return ole.NewVariant(ole.VT_I4, int64(cast)), nil
	case uint:
		return ole.NewVariant(ole.VT_I4, int64(cast)), nil
	case []int:
		return CreateNumericArrayVariant(cast, ole.VT_I4)
	case []uint:
		return CreateNumericArrayVariant(cast, ole.VT_I4)

	case string:
		return ole.NewVariant(ole.VT_BSTR, int64(uintptr(unsafe.Pointer(ole.SysAllocStringLen(value.(string)))))), nil
	case []string:
		if len(cast) == 0 {
			return ole.NewVariant(ole.VT_NULL, 0), nil
		}
		return CreateStringArrayVariant(cast)

	case time.Time:
		return convertTimeToDataTime(&cast), nil
	case *time.Time:
		return convertTimeToDataTime(cast), nil
	case time.Duration:
		return convertDurationToDateTime(cast), nil
	case nil:
		return ole.NewVariant(ole.VT_NULL, 0), nil
	case *ole.IUnknown:
		if cast == nil {
			return ole.NewVariant(ole.VT_NULL, 0), nil
		}
		return ole.NewVariant(ole.VT_UNKNOWN, int64(uintptr(unsafe.Pointer(cast)))), nil
	case *Instance:
		if cast == nil {
			return ole.NewVariant(ole.VT_NULL, 0), nil
		}
		return ole.NewVariant(ole.VT_UNKNOWN, int64(uintptr(unsafe.Pointer(cast.object)))), nil
	default:
		return ole.VARIANT{}, fmt.Errorf("unsupported type for automation variants %T", value)
	}
}

func convertToGoType(variant *ole.VARIANT, outputValue reflect.Value, outputType reflect.Type) (value interface{}, err error) {
	if variant.VT&ole.VT_ARRAY == ole.VT_ARRAY {
		return convertVariantToArray(variant, outputType)
	}

	if variant.VT == ole.VT_UNKNOWN {
		return convertVariantToStruct(variant, outputType)
	}

	switch cast := outputValue.Interface().(type) {
	case bool:
		return variant.Val != 0, nil
	case time.Time:
		return convertDataTimeToTime(variant)
	case *time.Time:
		x, err := convertDataTimeToTime(variant)
		return &x, err
	case time.Duration:
		return convertIntervalToDuration(variant)
	case uint, uint8, uint16, uint32, uint64, int, int8, int16, int32, int64:
		return convertVariantToInt(variant, outputType)
	case float32, float64:
		return convertVariantToFloat(variant, outputType)
	case string:
		return variant.ToString(), nil
	default:
		if variant.VT == ole.VT_NULL {
			return nil, nil
		}
		return nil, fmt.Errorf("could not convert %d to %v", variant.VT, cast)
	}
}

func convertInt64ToInt(value int64, outputType reflect.Type) (interface{}, error) {
	switch outputType.Kind() {
	case reflect.Int:
		return int(value), nil
	case reflect.Int8:
		return int8(value), nil
	case reflect.Int16:
		return int16(value), nil
	case reflect.Int32:
		return int32(value), nil
	case reflect.Int64:
		return int64(value), nil
	case reflect.Uint:
		return uint(value), nil
	case reflect.Uint8:
		return uint8(value), nil
	case reflect.Uint16:
		return uint16(value), nil
	case reflect.Uint32:
		return uint32(value), nil
	case reflect.Uint64:
		return uint64(value), nil
	default:
		return 0, fmt.Errorf("could not convert int64 to %v", outputType)
	}
}

func convertStringToInt64(str string, unsigned bool) (int64, error) {
	if unsigned {
		val, err := strconv.ParseUint(str, 0, 64)
		return int64(val), err
	}

	return strconv.ParseInt(str, 0, 64)
}

func convertVariantToInt(variant *ole.VARIANT, outputType reflect.Type) (interface{}, error) {
	var value int64
	switch variant.VT {
	case ole.VT_NULL:
		fallthrough
	case ole.VT_BOOL:
		fallthrough
	case ole.VT_I1, ole.VT_I2, ole.VT_I4, ole.VT_I8, ole.VT_INT:
		fallthrough
	case ole.VT_UI1, ole.VT_UI2, ole.VT_UI4, ole.VT_UI8, ole.VT_UINT:
		value = variant.Val
	case ole.VT_R4:
		// not necessarily a useful conversion but handle it anyway
		value = int64(*(*float32)(unsafe.Pointer(&variant.Val)))
	case ole.VT_R8:
		value = int64(*(*float64)(unsafe.Pointer(&variant.Val)))
	case ole.VT_BSTR:
		var err error
		value, err = convertStringToInt64(variant.ToString(), outputType.Kind() == reflect.Uint64)
		if err != nil {
			return value, err
		}
	default:
		return nil, fmt.Errorf("could not convert variant type %d to %v", variant.VT, outputType)
	}

	return convertInt64ToInt(value, outputType)
}

func convertVariantToFloat(variant *ole.VARIANT, outputType reflect.Type) (interface{}, error) {
	var value float64
	switch variant.VT {
	case ole.VT_NULL:
		fallthrough
	case ole.VT_BOOL:
		fallthrough
	case ole.VT_I1, ole.VT_I2, ole.VT_I4, ole.VT_I8, ole.VT_INT:
		fallthrough
	case ole.VT_UI1, ole.VT_UI2, ole.VT_UI4, ole.VT_UI8, ole.VT_UINT:
		value = float64(variant.Val)
	case ole.VT_R4:
		value = float64(*(*float32)(unsafe.Pointer(&variant.Val)))
	case ole.VT_R8:
		value = *(*float64)(unsafe.Pointer(&variant.Val))
	case ole.VT_BSTR:
		var err error
		value, err = strconv.ParseFloat(variant.ToString(), 64)
		if err != nil {
			return value, err
		}
	default:
		return nil, fmt.Errorf("could not convert variant type %d to %v", variant.VT, outputType)
	}

	if outputType.Kind() == reflect.Float32 {
		return float32(value), nil
	}

	return value, nil
}

func convertVariantToStruct(variant *ole.VARIANT, outputType reflect.Type) (interface{}, error) {
	if variant.VT != ole.VT_UNKNOWN {
		return nil, fmt.Errorf("could not convert non-IUnknown variant type %d to %v", variant.VT, outputType)
	}

	ptr := variant.ToIUnknown()

	var rawInstance struct {
		*ole.IUnknown
		*IWbemClassObjectVtbl
	}

	rawInstance.IUnknown = ptr
	rawInstance.IWbemClassObjectVtbl = (*IWbemClassObjectVtbl)(unsafe.Pointer(ptr.RawVTable))

	instance := (*Instance)(unsafe.Pointer(&rawInstance))
	val := reflect.New(outputType)
	err := instance.GetAll(val.Interface())
	return val.Elem().Interface(), err
}

func convertVariantToArray(variant *ole.VARIANT, outputType reflect.Type) (interface{}, error) {
	if variant.VT&ole.VT_ARRAY != ole.VT_ARRAY {
		return nil, fmt.Errorf("could not convert non-array variant type %d to %v", variant.VT, outputType)
	}

	safeArrayConversion := ole.SafeArrayConversion{Array: *(**ole.SafeArray)(unsafe.Pointer(&variant.Val))}

	arrayLen, err := safeArrayConversion.TotalElements(0)
	if err != nil {
		return nil, err
	}
	elemVT := (^ole.VT_ARRAY) & variant.VT
	slice := reflect.MakeSlice(reflect.SliceOf(outputType.Elem()), int(arrayLen), int(arrayLen))

	for i := 0; i < int(arrayLen); i++ {
		elemVariant := ole.VARIANT{VT: elemVT}
		elemSrc, err := safeArrayGetAsVariantVal(safeArrayConversion.Array, int64(i), elemVariant)
		if err != nil {
			return nil, err
		}
		elemVariant.Val = int64(elemSrc)
		elemDest, err := convertToGoType(&elemVariant, slice.Index(i), outputType.Elem())
		if err != nil {
			return nil, err
		}

		slice.Index(i).Set(reflect.ValueOf(elemDest))
	}

	return slice.Interface(), nil
}

func convertToGenericValue(variant *ole.VARIANT) interface{} {
	var result interface{}
	if variant.VT&ole.VT_ARRAY == ole.VT_ARRAY {
		safeArrayConversion := ole.SafeArrayConversion{Array: *(**ole.SafeArray)(unsafe.Pointer(&variant.Val))}
		result = safeArrayConversion.ToValueArray()
	} else {
		result = variant.Value()
	}
	return result
}

func convertTimeToDataTime(time *time.Time) ole.VARIANT {
	if time == nil || !time.After(WindowsEpoch) {
		return ole.NewVariant(ole.VT_NULL, 0)
	}
	_, offset := time.Zone()
	// convert to minutes
	offset /= 60
	// yyyymmddHHMMSS.mmmmmmsUUU
	s := fmt.Sprintf("%s%+04d", time.Format("20060102150405.000000"), offset)
	return ole.NewVariant(ole.VT_BSTR, int64(uintptr(unsafe.Pointer(ole.SysAllocStringLen(s)))))
}

func convertDurationToDateTime(duration time.Duration) ole.VARIANT {
	const dayTime = time.Second * 86400

	if duration == 0 {
		return ole.NewVariant(ole.VT_NULL, 0)
	}

	days := duration / dayTime
	duration = duration % dayTime

	hours := duration / time.Hour
	duration = duration % time.Hour

	mins := duration / time.Minute
	duration = duration % time.Minute

	seconds := duration / time.Second
	duration = duration % time.Second

	micros := duration / time.Microsecond

	s := fmt.Sprintf("%08d%02d%02d%02d.%06d:000", days, hours, mins, seconds, micros)
	return ole.NewVariant(ole.VT_BSTR, int64(uintptr(unsafe.Pointer(ole.SysAllocStringLen(s)))))
}

func extractDateTimeString(variant *ole.VARIANT) (string, error) {
	switch variant.VT {
	case ole.VT_BSTR:
		return variant.ToString(), nil
	case ole.VT_NULL:
		return "", nil
	default:
		return "", errors.New("variant not compatible with dateTime field")
	}
}

func convertDataTimeToTime(variant *ole.VARIANT) (time.Time, error) {
	var err error
	dateTime, err := extractDateTimeString(variant)
	if err != nil || len(dateTime) == 0 {
		return zeroTime, err
	}

	dLen := len(dateTime)
	if dLen < 5 {
		return zeroTime, errors.New("invalid datetime string")
	}

	if strings.HasPrefix(dateTime, "00000000000000.000000") {
		// Zero time
		return zeroTime, nil
	}

	zoneStart := dLen - 4
	timePortion := dateTime[0:zoneStart]

	var zoneMinutes int64
	if dateTime[zoneStart] == ':' {
		// interval ends in :000
		return parseIntervalTime(dateTime)
	}

	zoneSuffix := dateTime[zoneStart:dLen]
	zoneMinutes, err = strconv.ParseInt(zoneSuffix, 10, 0)
	if err != nil {
		return zeroTime, errors.New("invalid datetime string, zone did not parse")
	}

	timePortion = fmt.Sprintf("%s%+03d%02d", timePortion, zoneMinutes/60, abs(int(zoneMinutes%60)))
	return time.Parse("20060102150405.000000-0700", timePortion)
}

// parseIntervalTime encodes an interval time as an offset to Unix time
// allowing a duration to be computed without precision loss
func parseIntervalTime(interval string) (time.Time, error) {
	if len(interval) < 25 || interval[21:22] != ":" {
		return time.Time{}, fmt.Errorf("invalid interval time: %s", interval)
	}

	days, err := parseUintChain(interval[0:8], nil)
	hours, err := parseUintChain(interval[8:10], err)
	mins, err := parseUintChain(interval[10:12], err)
	secs, err := parseUintChain(interval[12:14], err)
	micros, err := parseUintChain(interval[15:21], err)

	if err != nil {
		return time.Time{}, err
	}

	var stamp = secs
	stamp += days * 86400
	stamp += hours * 3600
	stamp += mins * 60

	return time.Unix(int64(stamp), int64(micros*1000)), nil
}

func convertIntervalToDuration(variant *ole.VARIANT) (time.Duration, error) {
	var err error
	interval, err := extractDateTimeString(variant)
	if err != nil || len(interval) == 0 {
		return 0, err
	}

	t, err := parseIntervalTime(interval)
	if err != nil {
		return 0, nil
	}

	return t.Sub(unixEpoch), nil
}

func parseUintChain(str string, err error) (uint64, error) {
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(str, 10, 0)
}

func abs(num int) int {
	if num < 0 {
		return -num
	}

	return num
}
