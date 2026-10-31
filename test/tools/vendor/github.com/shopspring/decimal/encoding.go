package decimal

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"reflect"
)

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *Decimal) UnmarshalJSON(decimalBytes []byte) error {
	if string(decimalBytes) == "null" {
		return nil
	}

	decimal, err := NewFromString(unquoteIfQuoted(string(decimalBytes)))
	*d = decimal
	if err != nil {
		return fmt.Errorf("error decoding string '%s': %s", string(decimalBytes), err)
	}
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (d Decimal) MarshalJSON() ([]byte, error) {
	str := d.String()
	if MarshalJSONWithoutQuotes {
		return []byte(str), nil
	}
	b := make([]byte, 0, len(str)+2)
	b = append(b, '"')
	b = append(b, str...)
	return append(b, '"'), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface. As a string representation
// is already used when encoding to text, this method stores that string as []byte
func (d *Decimal) UnmarshalBinary(data []byte) error {
	// Verify we have at least 4 bytes for the exponent. The GOB encoded value
	// may be empty.
	if len(data) < 4 {
		return fmt.Errorf("error decoding binary %v: expected at least 4 bytes, got %d", data, len(data))
	}

	// Extract the exponent
	exp := int32(binary.BigEndian.Uint32(data[:4]))
	if int64(exp) > int64(MaxDecodeExponent) || int64(exp) < -int64(MaxDecodeExponent) {
		return fmt.Errorf("error decoding binary: exponent %d exceeds MaxDecodeExponent (%d)", exp, MaxDecodeExponent)
	}
	d.exp = exp

	// Extract the value
	d.value = new(big.Int)
	if err := d.value.GobDecode(data[4:]); err != nil {
		return fmt.Errorf("error decoding binary %v: %s", data, err)
	}

	return nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (d Decimal) MarshalBinary() (data []byte, err error) {
	// exp is written first, but encode value first to know output size
	var valueData []byte
	if valueData, err = d.getValue().GobEncode(); err != nil {
		return nil, err
	}

	// Write the exponent in front, since it's a fixed size
	expData := make([]byte, 4, len(valueData)+4)
	binary.BigEndian.PutUint32(expData, uint32(d.exp))

	// Return the byte array
	return append(expData, valueData...), nil
}

// Scan implements the sql.Scanner interface for database deserialization.
func (d *Decimal) Scan(value interface{}) error {
	// first try to see if the data is stored in database as a Numeric datatype
	switch v := value.(type) {

	case nil:
		return fmt.Errorf("can't scan NULL into a Decimal, use NullDecimal or *Decimal")

	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return fmt.Errorf("could not convert value '%v' to decimal", v)
		}
		*d = NewFromFloat32(v)
		return nil

	case float64:
		// numeric in sqlite3 sends us float64
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("could not convert value '%v' to decimal", v)
		}
		*d = NewFromFloat(v)
		return nil

	case int64:
		// at least in sqlite3 when the value is 0 in db, the data is sent
		// to us as an int64 instead of a float64 ...
		*d = New(v, 0)
		return nil

	case uint64:
		// while clickhouse may send 0 in db as uint64
		*d = NewFromUint64(v)
		return nil

	case int, int8, int16, int32:
		// clickhouse and test mocks send the other integer sizes
		*d = New(reflect.ValueOf(v).Int(), 0)
		return nil

	case uint, uint8, uint16, uint32:
		*d = NewFromUint64(reflect.ValueOf(v).Uint())
		return nil

	case big.Int:
		// clickhouse sends Int128, UInt128 and Int256 through database/sql as big.Int
		*d = NewFromBigInt(&v, 0)
		return nil

	case *big.Int:
		if v != nil {
			*d = NewFromBigInt(v, 0)
			return nil
		}

	case Decimal:
		// clickhouse sends Decimal columns through its native API as Decimal
		*d = v
		return nil

	case string:
		var err error
		*d, err = NewFromString(unquoteIfQuoted(v))
		return err

	case []byte:
		var err error
		*d, err = NewFromString(unquoteIfQuoted(string(v)))
		return err
	}
	return fmt.Errorf("could not convert value '%+v' to any known type", value)
}

// Value implements the driver.Valuer interface for database serialization.
func (d Decimal) Value() (driver.Value, error) {
	return d.String(), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for XML
// deserialization.
func (d *Decimal) UnmarshalText(text []byte) error {
	str := string(text)

	dec, err := NewFromString(str)
	*d = dec
	if err != nil {
		return fmt.Errorf("error decoding string '%s': %s", str, err)
	}

	return nil
}

// MarshalText implements the encoding.TextMarshaler interface for XML
// serialization.
func (d Decimal) MarshalText() (text []byte, err error) {
	return []byte(d.String()), nil
}

// GobEncode implements the gob.GobEncoder interface for gob serialization.
func (d Decimal) GobEncode() ([]byte, error) {
	return d.MarshalBinary()
}

// GobDecode implements the gob.GobDecoder interface for gob serialization.
func (d *Decimal) GobDecode(data []byte) error {
	return d.UnmarshalBinary(data)
}

// DecodeSpanner decodes a Spanner value into a Decimal
func (d *Decimal) DecodeSpanner(val interface{}) error {
	return d.Scan(val)
}

// EncodeSpanner encodes a Decimal into a Spanner value
func (d Decimal) EncodeSpanner() (interface{}, error) {
	return d.String(), nil
}

// BSON type numbers, see https://bsonspec.org/spec.html
const (
	bsonTypeDouble     byte = 0x01
	bsonTypeString     byte = 0x02
	bsonTypeUndefined  byte = 0x06
	bsonTypeNull       byte = 0x0A
	bsonTypeInt32      byte = 0x10
	bsonTypeInt64      byte = 0x12
	bsonTypeDecimal128 byte = 0x13
)

// MarshalBSONValue implements the bson.ValueMarshaler interface of the MongoDB Go driver v2. It stores
// the decimal as a BSON Decimal128, MongoDB's decimal type, which holds 34 significant digits and
// exponents from -6176 to 6111. Trailing zeros move between the coefficient and the exponent to fit, so
// Exponent() can change in a round trip but the value can't. Decimals that still don't fit return an
// error instead of being rounded.
//
// The v1 driver declares this interface with its own bsontype.Type, so it doesn't call this method.
func (d Decimal) MarshalBSONValue() (byte, []byte, error) {
	coef := new(big.Int).Abs(d.getValue())
	exp := d.exp

	// Zero fits at any exponent, and the loops below could take 2^31 steps to get there.
	if coef.Sign() == 0 {
		if exp < decimal128MinExp {
			exp = decimal128MinExp
		} else if exp > decimal128MaxExp {
			exp = decimal128MaxExp
		}
	}

	quo, rem := new(big.Int), new(big.Int)
	for (coef.Cmp(decimal128MaxCoef) > 0 || exp < decimal128MinExp) && exp < decimal128MaxExp {
		if quo.QuoRem(coef, tenInt, rem); rem.Sign() != 0 {
			break
		}
		coef, quo = quo, coef
		exp++
	}
	for exp > decimal128MaxExp && coef.Cmp(decimal128MaxCoef) <= 0 {
		coef.Mul(coef, tenInt)
		exp--
	}
	if coef.Cmp(decimal128MaxCoef) > 0 || exp < decimal128MinExp {
		return 0, nil, fmt.Errorf("can't encode %se%d as BSON Decimal128 without rounding: it holds 34 significant digits and exponents from %d to %d",
			d.getValue(), d.exp, decimal128MinExp, decimal128MaxExp)
	}
	return bsonTypeDecimal128, encodeDecimal128(d.Sign() < 0, coef, exp), nil
}

// UnmarshalBSONValue implements the bson.ValueUnmarshaler interface of the MongoDB Go driver v2.
// It decodes BSON Decimal128, string, double, int32 and int64 values. BSON null and undefined decode
// to zero, as they do for the driver's own number types.
func (d *Decimal) UnmarshalBSONValue(typ byte, data []byte) error {
	var err error
	switch {
	case typ == bsonTypeNull || typ == bsonTypeUndefined:
		*d = Decimal{}
	case typ == bsonTypeDecimal128 && len(data) == 16:
		*d, err = decodeDecimal128(data)
	case typ == bsonTypeString && len(data) >= 5:
		// int32 length, the string, then a NUL byte
		*d, err = NewFromString(string(data[4 : len(data)-1]))
	case typ == bsonTypeDouble && len(data) == 8:
		f := math.Float64frombits(binary.LittleEndian.Uint64(data))
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("can't convert BSON double %v to decimal", f)
		}
		*d = NewFromFloat(f)
	case typ == bsonTypeInt32 && len(data) == 4:
		*d = New(int64(int32(binary.LittleEndian.Uint32(data))), 0)
	case typ == bsonTypeInt64 && len(data) == 8:
		*d = New(int64(binary.LittleEndian.Uint64(data)), 0)
	default:
		return fmt.Errorf("can't convert BSON value of type 0x%02x and length %d to decimal", typ, len(data))
	}
	return err
}

// BSON stores a Decimal128 as a little-endian IEEE 754-2008 decimal128 in the binary integer decimal
// encoding: from the top bit down, a sign bit, a 14-bit exponent biased by 6176 and a 113-bit coefficient.
const (
	decimal128MinExp = -6176
	decimal128MaxExp = 6111
)

// decimal128MaxCoef is 10^34 - 1, the largest Decimal128 coefficient.
var decimal128MaxCoef = new(big.Int).Sub(new(big.Int).Exp(tenInt, big.NewInt(34), nil), oneInt)

// encodeDecimal128 expects a coefficient and exponent that are already in range.
func encodeDecimal128(neg bool, coef *big.Int, exp int32) []byte {
	var buf [16]byte
	b := coef.Bytes()
	copy(buf[16-len(b):], b)
	hi := binary.BigEndian.Uint64(buf[:8]) | uint64(exp-decimal128MinExp)<<49
	if neg {
		hi |= 1 << 63
	}
	data := make([]byte, 16)
	binary.LittleEndian.PutUint64(data, binary.BigEndian.Uint64(buf[8:]))
	binary.LittleEndian.PutUint64(data[8:], hi)
	return data
}

func decodeDecimal128(data []byte) (Decimal, error) {
	lo := binary.LittleEndian.Uint64(data)
	hi := binary.LittleEndian.Uint64(data[8:])
	neg := hi>>63 == 1

	var exp int32
	switch {
	case hi>>59&0xF == 0xF:
		return Decimal{}, fmt.Errorf("can't convert BSON Decimal128 NaN or Infinity to decimal")
	case hi>>61&3 == 3:
		// The exponent starts 2 bits lower, and the coefficient is always above 10^34 - 1.
		exp, hi, lo = int32(hi>>47&0x3FFF), 0, 0
	default:
		exp, hi = int32(hi>>49&0x3FFF), hi&(1<<49-1)
	}
	exp += decimal128MinExp
	if int64(exp) > int64(MaxDecodeExponent) || int64(exp) < -int64(MaxDecodeExponent) {
		return Decimal{}, fmt.Errorf("can't convert BSON Decimal128 to decimal: exponent %d exceeds MaxDecodeExponent (%d)", exp, MaxDecodeExponent)
	}

	var buf [16]byte
	binary.BigEndian.PutUint64(buf[:8], hi)
	binary.BigEndian.PutUint64(buf[8:], lo)
	value := new(big.Int).SetBytes(buf[:])
	// IEEE 754 reads coefficients above 10^34 - 1 as zero.
	if value.Cmp(decimal128MaxCoef) > 0 {
		value.SetInt64(0)
	}
	if neg {
		value.Neg(value)
	}
	return Decimal{value: value, exp: exp}, nil
}

func unquoteIfQuoted(value string) string {
	// If the amount is quoted, strip the quotes
	if len(value) > 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}

	return value
}

// NullDecimal represents a nullable decimal with compatibility for
// scanning null values from the database.
type NullDecimal struct {
	Decimal Decimal
	Valid   bool
}

// NewNullDecimal returns a valid NullDecimal holding d.
func NewNullDecimal(d Decimal) NullDecimal {
	return NullDecimal{
		Decimal: d,
		Valid:   true,
	}
}

// Scan implements the sql.Scanner interface for database deserialization.
func (d *NullDecimal) Scan(value interface{}) error {
	if value == nil {
		d.Valid = false
		return nil
	}
	err := d.Decimal.Scan(value)
	d.Valid = err == nil
	return err
}

// Value implements the driver.Valuer interface for database serialization.
func (d NullDecimal) Value() (driver.Value, error) {
	if !d.Valid {
		return nil, nil
	}
	return d.Decimal.Value()
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *NullDecimal) UnmarshalJSON(decimalBytes []byte) error {
	if string(decimalBytes) == "null" {
		d.Valid = false
		return nil
	}
	err := d.Decimal.UnmarshalJSON(decimalBytes)
	d.Valid = err == nil
	return err
}

// MarshalJSON implements the json.Marshaler interface.
func (d NullDecimal) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return d.Decimal.MarshalJSON()
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for XML
// deserialization
func (d *NullDecimal) UnmarshalText(text []byte) error {
	str := string(text)

	// check for empty XML or XML without body e.g., <tag></tag>
	if str == "" {
		d.Valid = false
		return nil
	}
	if err := d.Decimal.UnmarshalText(text); err != nil {
		d.Valid = false
		return err
	}
	d.Valid = true
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface for XML
// serialization.
func (d NullDecimal) MarshalText() (text []byte, err error) {
	if !d.Valid {
		return []byte{}, nil
	}
	return d.Decimal.MarshalText()
}

// DecodeSpanner decodes a Spanner value into a Decimal
func (d *NullDecimal) DecodeSpanner(value interface{}) error {
	switch t := value.(type) {
	case nil:
		d.Valid = false
		return nil
	case *string:
		if t == nil {
			d.Valid = false
			return nil
		}
		value = *t
	}

	err := d.Decimal.Scan(value)
	d.Valid = err == nil
	return err
}

// EncodeSpanner encodes a Decimal into a Spanner value
func (d NullDecimal) EncodeSpanner() (interface{}, error) {
	if !d.Valid {
		return nil, nil
	}
	return d.Decimal.String(), nil
}

// MarshalBSONValue implements the bson.ValueMarshaler interface of the MongoDB Go driver v2.
// An invalid NullDecimal is stored as BSON null, see Decimal.MarshalBSONValue for the rest.
func (d NullDecimal) MarshalBSONValue() (byte, []byte, error) {
	if !d.Valid {
		return bsonTypeNull, nil, nil
	}
	return d.Decimal.MarshalBSONValue()
}

// UnmarshalBSONValue implements the bson.ValueUnmarshaler interface of the MongoDB Go driver v2.
// BSON null and undefined make the NullDecimal invalid, see Decimal.UnmarshalBSONValue for the rest.
func (d *NullDecimal) UnmarshalBSONValue(typ byte, data []byte) error {
	err := d.Decimal.UnmarshalBSONValue(typ, data)
	d.Valid = err == nil && typ != bsonTypeNull && typ != bsonTypeUndefined
	return err
}
