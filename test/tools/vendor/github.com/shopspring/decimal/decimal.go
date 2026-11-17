// Package decimal implements an arbitrary precision fixed-point decimal.
//
// The zero-value of a Decimal is 0, as you would expect.
//
// The best way to create a new Decimal is to use decimal.NewFromString, ex:
//
//	n, err := decimal.NewFromString("-123.4567")
//	n.String() // output: "-123.4567"
//
// To use Decimal as part of a struct:
//
//	type StructName struct {
//	    Number Decimal
//	}
//
// Note: This can "only" represent numbers with a maximum of 2^31 digits after the decimal point.
package decimal

import (
	"fmt"
	"io"
	"math"
	"math/big"
	"math/bits"
	"regexp"
	"strconv"
	"strings"
)

// DivisionPrecision is the number of decimal places in the result when it
// doesn't divide exactly. Div and Sqrt read it, and so do Avg, Atan and Tan through Div.
//
// Set it once at program start. Changing it while other goroutines use the package is a data race.
// To choose the precision per call, use DivRound instead of Div and SqrtRound instead of Sqrt.
//
// Example:
//
//	d1 := decimal.NewFromFloat(2).Div(decimal.NewFromFloat(3))
//	d1.String() // output: "0.6666666666666667"
//	d2 := decimal.NewFromFloat(2).Div(decimal.NewFromFloat(30000))
//	d2.String() // output: "0.0000666666666667"
//	d3 := decimal.NewFromFloat(20000).Div(decimal.NewFromFloat(3))
//	d3.String() // output: "6666.6666666666666667"
//	decimal.DivisionPrecision = 3
//	d4 := decimal.NewFromFloat(2).Div(decimal.NewFromFloat(3))
//	d4.String() // output: "0.667"
var DivisionPrecision = 16

// PowPrecisionNegativeExponent specifies the number of digits after the decimal point in the results of
// Pow, PowInt32 and PowBigInt for negative or non-integer exponents. A result that would round to 0 keeps
// PowPrecisionNegativeExponent significant digits instead (at least one).
// PowWithPrecision is not affected, it takes the precision as an argument.
//
// Set it once at program start. Changing it while other goroutines use the package is a data race.
// To choose the precision per call, use PowWithPrecision.
//
// Example:
//
//	d1, err := decimal.NewFromFloat(15.2).PowInt32(-2)
//	d1.String() // output: "0.0043282548476454"
//
//	decimal.PowPrecisionNegativeExponent = 24
//	d2, err := decimal.NewFromFloat(15.2).PowInt32(-2)
//	d2.String() // output: "0.004328254847645429362881"
var PowPrecisionNegativeExponent = 16

// MarshalJSONWithoutQuotes should be set to true if you want the decimal to
// be JSON marshaled as a number, instead of as a string.
// WARNING: this is dangerous for decimals with many digits, since many JSON
// unmarshallers (ex: Javascript's) will unmarshal JSON numbers to IEEE 754
// double-precision floating point numbers, which means you can potentially
// silently lose precision.
//
// Set it once at program start. Changing it while other goroutines use the package is a data race.
var MarshalJSONWithoutQuotes = false

// TrimTrailingZeros specifies whether trailing zeroes should be trimmed from a string representation of decimal.
// If set to true, trailing zeroes will be truncated (2.00 -> 2, 3.11 -> 3.11, 13.000 -> 13),
// otherwise trailing zeroes will be preserved (2.00 -> 2.00, 3.11 -> 3.11, 13.000 -> 13.000).
// Setting this value to false can be useful for APIs where exact decimal string representation matters.
// It affects String and everything built on it, such as MarshalJSON, MarshalText and Value.
//
// Set it once at program start. Changing it while other goroutines use the package is a data race.
// For a fixed number of decimal places per call, use StringFixed.
var TrimTrailingZeros = true

// UseScientificNotation specifies whether scientific notation should be used when a decimal is turned
// into a string that has a "negative" precision.
//
// For example, 1200 rounded to the nearest 100 cannot accurately be shown as "1200" because the last two
// digits are unknown. With this set to true, that number would be expressed as "1.2E3" instead.
//
// Set it once at program start. Changing it while other goroutines use the package is a data race.
// StringFixed never uses scientific notation.
var UseScientificNotation = false

// ExpMaxIterations specifies the maximum number of iterations needed to calculate
// precise natural exponent value using ExpHullAbrham method.
//
// Set it once at program start. Changing it while other goroutines use the package is a data race.
var ExpMaxIterations = 1000

// MaxDecodeExponent is the largest exponent magnitude that NewFromString and UnmarshalBinary accept,
// and so UnmarshalJSON, UnmarshalText, Scan, GobDecode, DecodeSpanner, UnmarshalBSONValue and their
// NullDecimal variants too.
// Operations like String, Add and Float64 take time and memory that grow with the exponent, so without
// this limit a short input like "1e-2000000000" can stall the process or exhaust its memory.
// The default covers the float64 and IEEE 754 decimal128 exponent ranges.
//
// Values created with New, NewFromBigInt or arithmetic are not checked, so they may not decode back from
// their own String or MarshalBinary output. The limit does not bound the cost of Pow or ExpTaylor
// arguments or of very long inputs; validate those separately.
//
// Set it once at program start, before any decoding. Changing it while other goroutines decode is a
// data race. Raise it only for trusted input.
var MaxDecodeExponent = 10000

// Zero constant, to make computations faster.
// Zero should never be compared with == or != directly, please use decimal.Equal or decimal.Cmp instead.
var Zero = Decimal{}

// Decimal represents a fixed-point decimal. It is immutable.
// number = value * 10 ^ exp
type Decimal struct {
	value *big.Int

	// NOTE(vadim): this must be an int32, because we cast it to float64 during
	// calculations. If exp is 64 bit, we might lose precision.
	// If we cared about being able to represent every possible decimal, we
	// could make exp a *big.Int but it would hurt performance and numbers
	// like that are unrealistic.
	exp int32
}

func (d Decimal) getValue() *big.Int {
	if d.value == nil {
		return zeroInt
	}
	return d.value
}

// New returns a new fixed-point decimal, value * 10 ^ exp.
func New(value int64, exp int32) Decimal {
	return Decimal{
		value: big.NewInt(value),
		exp:   exp,
	}
}

// NewFromInt converts an int64 to Decimal.
//
// Example:
//
//	NewFromInt(123).String() // output: "123"
//	NewFromInt(-10).String() // output: "-10"
func NewFromInt(value int64) Decimal {
	return Decimal{
		value: big.NewInt(value),
		exp:   0,
	}
}

// NewFromInt32 converts an int32 to Decimal.
//
// Example:
//
//	NewFromInt(123).String() // output: "123"
//	NewFromInt(-10).String() // output: "-10"
func NewFromInt32(value int32) Decimal {
	return Decimal{
		value: big.NewInt(int64(value)),
		exp:   0,
	}
}

// NewFromUint64 converts an uint64 to Decimal.
//
// Example:
//
//	NewFromUint64(123).String() // output: "123"
func NewFromUint64(value uint64) Decimal {
	return Decimal{
		value: new(big.Int).SetUint64(value),
		exp:   0,
	}
}

// NewFromBigInt returns a new Decimal from a big.Int, value * 10 ^ exp
func NewFromBigInt(value *big.Int, exp int32) Decimal {
	return Decimal{
		value: new(big.Int).Set(value),
		exp:   exp,
	}
}

// NewFromBigRat returns a new Decimal from a big.Rat. The numerator and
// denominator are divided and rounded to the given precision.
//
// Example:
//
//	d1 := NewFromBigRat(big.NewRat(0, 1), 0)    // output: "0"
//	d2 := NewFromBigRat(big.NewRat(4, 5), 1)    // output: "0.8"
//	d3 := NewFromBigRat(big.NewRat(1000, 3), 3) // output: "333.333"
//	d4 := NewFromBigRat(big.NewRat(2, 7), 4)    // output: "0.2857"
func NewFromBigRat(value *big.Rat, precision int32) Decimal {
	return Decimal{
		value: new(big.Int).Set(value.Num()),
		exp:   0,
	}.DivRound(Decimal{
		value: new(big.Int).Set(value.Denom()),
		exp:   0,
	}, precision)
}

// NewFromBigFloat returns a new Decimal from a big.Float. The result is the shortest decimal
// that rounds back to value at value's precision, as NewFromFloat does for a float64.
// It returns an error for nil and infinite values, and for exponents beyond MaxDecodeExponent.
//
// Example:
//
//	f, _ := new(big.Float).SetString("-123.45")
//	d, err := NewFromBigFloat(f)
//	d.String() // output: "-123.45"
func NewFromBigFloat(value *big.Float) (Decimal, error) {
	if value == nil {
		return Decimal{}, fmt.Errorf("can't convert nil *big.Float to decimal")
	}
	if value.IsInf() {
		return Decimal{}, fmt.Errorf("can't convert infinite *big.Float to decimal")
	}
	if value.Sign() == 0 {
		return New(0, 0), nil
	}
	return NewFromString(value.Text('g', -1))
}

// NewFromString returns a new Decimal from a string representation.
// Trailing zeroes are not trimmed.
//
// Example:
//
//	d, err := NewFromString("-123.45")
//	d2, err := NewFromString(".0001")
//	d3, err := NewFromString("1.47000")
func NewFromString(value string) (Decimal, error) {
	originalInput := value
	var exp int64

	// Accept Unicode minus sign (U+2212) in addition to ASCII '-'.
	value = strings.Replace(value, "\u2212", "-", -1)

	// Check if number is using scientific notation and find dots
	eIndex := -1
	pIndex := -1
	for i, r := range value {
		if r == 'E' || r == 'e' {
			if eIndex > -1 {
				return Decimal{}, fmt.Errorf("can't convert %s to decimal: multiple 'E' characters found", value)
			}
			eIndex = i
			continue
		}

		if r == '.' {
			if pIndex > -1 {
				return Decimal{}, fmt.Errorf("can't convert %s to decimal: too many .s", value)
			}
			pIndex = i
		}
	}

	if eIndex != -1 {
		expInt, err := strconv.ParseInt(value[eIndex+1:], 10, 32)
		if err != nil {
			if e, ok := err.(*strconv.NumError); ok && e.Err == strconv.ErrRange {
				return Decimal{}, fmt.Errorf("can't convert %s to decimal: fractional part too long", value)
			}
			return Decimal{}, fmt.Errorf("can't convert %s to decimal: exponent is not numeric", value)
		}
		value = value[:eIndex]
		exp = expInt
	}

	numLen := len(value)
	if pIndex != -1 {
		if pIndex+1 < len(value) && (value[pIndex+1] == '-' || value[pIndex+1] == '+') {
			// ParseInt and SetString would accept the sign once the point is removed, as in ".-5"
			return Decimal{}, fmt.Errorf("can't convert %s to decimal", value)
		}
		numLen--
		expInt := -len(value[pIndex+1:])
		exp += int64(expInt)
	}

	var dValue *big.Int
	// parsing in an int64 is faster than new(big.Int).SetString so this is just a shortcut for strings we know won't overflow
	if numLen <= 18 {
		parsed64, ok := parseInt64SkipIndex(value, pIndex)
		if !ok {
			return Decimal{}, fmt.Errorf("can't convert %s to decimal", value)
		}
		dValue = big.NewInt(parsed64)
	} else {
		intString := value
		if pIndex != -1 {
			intString = value[:pIndex] + value[pIndex+1:]
		}
		dValue = new(big.Int)
		_, ok := dValue.SetString(intString, 10)
		if !ok {
			return Decimal{}, fmt.Errorf("can't convert %s to decimal", value)
		}
	}

	if exp < math.MinInt32 || exp > math.MaxInt32 {
		// NOTE(vadim): I doubt a string could realistically be this long
		return Decimal{}, fmt.Errorf("can't convert %s to decimal: fractional part too long", originalInput)
	}
	if exp > int64(MaxDecodeExponent) || exp < -int64(MaxDecodeExponent) {
		return Decimal{}, fmt.Errorf("can't convert %s to decimal: exponent %d exceeds MaxDecodeExponent (%d)", originalInput, exp, MaxDecodeExponent)
	}

	return Decimal{
		value: dValue,
		exp:   int32(exp),
	}, nil
}

// parseInt64SkipIndex parses s without the byte at index skip as strconv.ParseInt(s, 10, 64) would,
// s must have at most 18 other bytes so that the result can't overflow.
func parseInt64SkipIndex(s string, skip int) (int64, bool) {
	i, neg := 0, false
	if len(s) > 0 && skip != 0 && (s[0] == '+' || s[0] == '-') {
		i, neg = 1, s[0] == '-'
	}
	var n int64
	digits := 0
	for ; i < len(s); i++ {
		if i == skip {
			continue
		}
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
		digits++
	}
	if neg {
		n = -n
	}
	return n, digits > 0
}

// NewFromFormattedString returns a new Decimal from a formatted string representation.
// The second argument - replRegexp, is a regular expression that is used to find characters that should be
// removed from given decimal string representation. All matched characters will be replaced with an empty string.
//
// Example:
//
//	r := regexp.MustCompile("[$,]")
//	d1, err := NewFromFormattedString("$5,125.99", r)
//
//	r2 := regexp.MustCompile("[_]")
//	d2, err := NewFromFormattedString("1_000_000", r2)
//
//	r3 := regexp.MustCompile("[USD\\s]")
//	d3, err := NewFromFormattedString("5000 USD", r3)
func NewFromFormattedString(value string, replRegexp *regexp.Regexp) (Decimal, error) {
	parsedValue := replRegexp.ReplaceAllString(value, "")
	d, err := NewFromString(parsedValue)
	if err != nil {
		return Decimal{}, err
	}
	return d, nil
}

// RequireFromString returns a new Decimal from a string representation
// or panics if NewFromString had returned an error.
//
// Example:
//
//	d := RequireFromString("-123.45")
//	d2 := RequireFromString(".0001")
func RequireFromString(value string) Decimal {
	dec, err := NewFromString(value)
	if err != nil {
		panic(err)
	}
	return dec
}

// NewFromFloat converts a float64 to Decimal.
//
// The result is the shortest decimal that converts back to the same float64,
// the same number that strconv.FormatFloat(value, 'f', -1, 64) prints.
// This is typically 15 digits, but may be more in some cases.
// See https://www.exploringbinary.com/decimal-precision-of-binary-floating-point-numbers/ for more information.
//
// This is not always the exact value stored in the float. Whole numbers larger
// than 2^53 can change: float64(1<<62) is exactly 4611686018427387904, but
// NewFromFloat returns 4611686018427388000. To convert whole numbers exactly,
// use NewFromFloatWithExponent(value, 0).
//
// For slightly faster conversion, use NewFromFloatWithExponent where you can specify the precision in absolute terms.
//
// NOTE: this will panic on NaN, +/-inf
func NewFromFloat(value float64) Decimal {
	if value == 0 {
		return New(0, 0)
	}
	return newFromFloat(value, 64)
}

// NewFromFloat32 converts a float32 to Decimal.
//
// The result is the shortest decimal that converts back to the same float32.
// This is typically 6-8 digits depending on the input.
// See https://www.exploringbinary.com/decimal-precision-of-binary-floating-point-numbers/ for more information.
//
// As with NewFromFloat, whole numbers larger than 2^24 can change. To convert
// them exactly, use NewFromFloatWithExponent(float64(value), 0).
//
// For slightly faster conversion, use NewFromFloatWithExponent where you can specify the precision in absolute terms.
//
// NOTE: this will panic on NaN, +/-inf
func NewFromFloat32(value float32) Decimal {
	if value == 0 {
		return New(0, 0)
	}
	return newFromFloat(float64(value), 32)
}

// newFromFloat converts val to a Decimal holding the shortest decimal digit
// string that round-trips back to val when parsed at the given bit size.
//
// strconv already computes exactly that string, so we ask it for the value in
// 'e' format with precision -1 and read the digits back out. The 'e' layout is
// [-]d[.ddd]e±dd, which is bounded (at most 24 bytes) and trivial to scan,
// unlike 'f' which can run to over 750 bytes for extreme exponents.
func newFromFloat(val float64, bitSize int) Decimal {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		panic(fmt.Sprintf("Cannot create a Decimal from %v", val))
	}

	var buf [32]byte
	b := strconv.AppendFloat(buf[:0], val, 'e', -1, bitSize)

	neg := b[0] == '-'
	if neg {
		b = b[1:]
	}

	// Split the significand from the exponent. b always contains an 'e'.
	var i int
	for b[i] != 'e' {
		i++
	}
	digits, expDigits := b[:i], b[i+1:]

	// The shortest round-trip form has at most 17 significant digits for a
	// float64 and 9 for a float32, so the significand always fits in an int64.
	var mant int64
	nd := 0
	for _, c := range digits {
		if c == '.' {
			continue
		}
		mant = mant*10 + int64(c-'0')
		nd++
	}
	if neg {
		mant = -mant
	}

	esign := 1
	switch expDigits[0] {
	case '-':
		esign = -1
		expDigits = expDigits[1:]
	case '+':
		expDigits = expDigits[1:]
	}
	e := 0
	for _, c := range expDigits {
		e = e*10 + int(c-'0')
	}

	// digits is d.ddd, i.e. the significand scaled by 10^(nd-1).
	return Decimal{value: big.NewInt(mant), exp: int32(esign*e - nd + 1)}
}

// NewFromFloatWithExponent converts a float64 to Decimal, with an arbitrary
// number of fractional digits.
//
// Example:
//
//	NewFromFloatWithExponent(123.456, -2).String() // output: "123.46"
func NewFromFloatWithExponent(value float64, exp int32) Decimal {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		panic(fmt.Sprintf("Cannot create a Decimal from %v", value))
	}

	bits := math.Float64bits(value)
	mant := bits & (1<<52 - 1)
	exp2 := int32((bits >> 52) & (1<<11 - 1))
	sign := bits >> 63

	if exp2 == 0 {
		// specials
		if mant == 0 {
			return Decimal{}
		}
		// subnormal
		exp2++
	} else {
		// normal
		mant |= 1 << 52
	}

	exp2 -= 1023 + 52

	// normalizing base-2 values
	for mant&1 == 0 {
		mant = mant >> 1
		exp2++
	}

	// maximum number of fractional base-10 digits to represent 2^N exactly cannot be more than -N if N<0
	if exp < 0 && exp < exp2 {
		if exp2 < 0 {
			exp = exp2
		} else {
			exp = 0
		}
	}

	// representing 10^M * 2^N as 5^M * 2^(M+N)
	exp2 -= exp

	temp := big.NewInt(1)
	dMant := big.NewInt(int64(mant))

	// applying 5^M
	if exp > 0 {
		temp = temp.SetInt64(int64(exp))
		temp = temp.Exp(fiveInt, temp, nil)
	} else if exp < 0 {
		temp = temp.SetInt64(-int64(exp))
		temp = temp.Exp(fiveInt, temp, nil)
		dMant = dMant.Mul(dMant, temp)
		temp = temp.SetUint64(1)
	}

	// applying 2^(M+N)
	if exp2 > 0 {
		dMant = dMant.Lsh(dMant, uint(exp2))
	} else if exp2 < 0 {
		temp = temp.Lsh(temp, uint(-exp2))
	}

	// rounding and downscaling
	if exp > 0 || exp2 < 0 {
		halfDown := new(big.Int).Rsh(temp, 1)
		dMant = dMant.Add(dMant, halfDown)
		dMant = dMant.Quo(dMant, temp)
	}

	if sign == 1 {
		dMant = dMant.Neg(dMant)
	}

	return Decimal{
		value: dMant,
		exp:   exp,
	}
}

// Copy returns a copy of decimal with the same value and exponent, but a different pointer to value.
func (d Decimal) Copy() Decimal {
	return Decimal{
		value: new(big.Int).Set(d.getValue()),
		exp:   d.exp,
	}
}

// rescale returns a rescaled version of the decimal. Returned
// decimal may be less precise if the given exponent is bigger
// than the initial exponent of the Decimal.
// NOTE: this will truncate, NOT round
//
// Example:
//
//	d1 := New(12345, -4)
//	d2 := d1.rescale(-1)
//	d3 := d2.rescale(-4)
//	fmt.Println(d1)
//	fmt.Println(d2)
//	fmt.Println(d3)
//
// Output:
//
//	1.2345
//	1.2
//	1.2
func (d Decimal) rescale(exp int32) Decimal {
	if d.exp == exp {
		return Decimal{
			new(big.Int).Set(d.getValue()),
			d.exp,
		}
	}

	// NOTE(vadim): must convert exps to float64 before - to prevent overflow
	diff := math.Abs(float64(exp) - float64(d.exp))
	value := new(big.Int)
	if exp > d.exp {
		value.Quo(d.getValue(), pow10(int64(diff)))
		if value.Sign() == 0 && d.getValue().Sign() != 0 {
			// reflect.DeepEqual tells an empty word slice from nil, keep the empty one that
			// dividing a copy of d.value in place used to leave
			value.SetBits([]big.Word{})
		}
	} else {
		value.Mul(d.getValue(), pow10(int64(diff)))
	}

	return Decimal{
		value: value,
		exp:   exp,
	}
}

// Abs returns the absolute value of the decimal.
func (d Decimal) Abs() Decimal {
	if !d.IsNegative() {
		return d
	}
	d2Value := new(big.Int).Abs(d.getValue())
	return Decimal{
		value: d2Value,
		exp:   d.exp,
	}
}

// Add returns d + d2.
func (d Decimal) Add(d2 Decimal) Decimal {
	// rescale returns a new value, so the sum can be stored in it
	if d.exp < d2.exp {
		r := d2.rescale(d.exp)
		r.value.Add(d.getValue(), r.value)
		return r
	} else if d.exp > d2.exp {
		r := d.rescale(d2.exp)
		r.value.Add(r.value, d2.getValue())
		return r
	}

	d3Value := new(big.Int).Add(d.getValue(), d2.getValue())
	return Decimal{
		value: d3Value,
		exp:   d.exp,
	}
}

// Sub returns d - d2.
func (d Decimal) Sub(d2 Decimal) Decimal {
	// rescale returns a new value, so the difference can be stored in it
	if d.exp < d2.exp {
		r := d2.rescale(d.exp)
		r.value.Sub(d.getValue(), r.value)
		return r
	} else if d.exp > d2.exp {
		r := d.rescale(d2.exp)
		r.value.Sub(r.value, d2.getValue())
		return r
	}

	d3Value := new(big.Int).Sub(d.getValue(), d2.getValue())
	return Decimal{
		value: d3Value,
		exp:   d.exp,
	}
}

// Neg returns -d.
func (d Decimal) Neg() Decimal {
	val := new(big.Int).Neg(d.getValue())
	return Decimal{
		value: val,
		exp:   d.exp,
	}
}

// Mul returns d * d2.
func (d Decimal) Mul(d2 Decimal) Decimal {
	expInt64 := int64(d.exp) + int64(d2.exp)
	if expInt64 > math.MaxInt32 || expInt64 < math.MinInt32 {
		// NOTE(vadim): better to panic than give incorrect results, as
		// Decimals are usually used for money
		panic(fmt.Sprintf("exponent %v overflows an int32!", expInt64))
	}

	d3Value := new(big.Int).Mul(d.getValue(), d2.getValue())
	return Decimal{
		value: d3Value,
		exp:   int32(expInt64),
	}
}

// Shift shifts the decimal in base 10.
// It shifts left when shift is positive and right if shift is negative.
// In simpler terms, the given value for shift is added to the exponent
// of the decimal.
// Shift panics if the resulting exponent does not fit in an int32.
func (d Decimal) Shift(shift int32) Decimal {
	exp := int64(d.exp) + int64(shift)
	if exp > math.MaxInt32 || exp < math.MinInt32 {
		panic(fmt.Sprintf("exponent %v overflows an int32!", exp))
	}
	value := d.getValue()
	if value.Sign() == 0 {
		// a copy of zero has a nil word slice, which reflect.DeepEqual tells from an empty one
		value = zeroInt
	}
	return Decimal{
		value: value,
		exp:   int32(exp),
	}
}

// Normalize returns d with the trailing zeros removed from its coefficient, so equal decimals get the
// same coefficient and exponent: 1.50, 1.5 and New(15, -1) all become New(15, -1), and every zero
// becomes Decimal{}, the same as Zero. It's like Java's BigDecimal.stripTrailingZeros.
//
// reflect.DeepEqual, and test helpers built on it such as testify's assert.Equal and gomock.Eq, compare
// the coefficient and exponent rather than the value, so normalize decimals before comparing them that
// way. Equal and Cmp compare values and don't need it.
//
// The value doesn't change, but its string form can: with TrimTrailingZeros = false, 1.50 prints as
// 1.5, and with UseScientificNotation = true, 1500 prints as 1.5E3.
func (d Decimal) Normalize() Decimal {
	v := d.getValue()
	if v.Sign() == 0 {
		return Decimal{}
	}

	// Count the trailing zeros with integer math: in v if it fits an int64, else in v mod 10^19,
	// and in all the digits of v only when those 19 are all zeros.
	var zeros int64
	if v.IsInt64() {
		for x := v.Int64(); x%10 == 0; x /= 10 {
			zeros++
		}
	} else {
		r := new(big.Int).Rem(v, pow10(19))
		for n := r.Abs(r).Uint64(); n%10 == 0 && zeros < 19; n /= 10 {
			zeros++
		}
		if zeros == 19 {
			s := v.String()
			zeros = int64(len(s) - len(strings.TrimRight(s, "0")))
		}
	}
	if room := math.MaxInt32 - int64(d.exp); zeros > room {
		zeros = room
	}
	if zeros == 0 {
		return d
	}
	return Decimal{value: new(big.Int).Quo(v, pow10(zeros)), exp: d.exp + int32(zeros)}
}

// Div returns d / d2. If it doesn't divide exactly, the result will have
// DivisionPrecision digits after the decimal point. Use DivRound to choose the precision per call.
func (d Decimal) Div(d2 Decimal) Decimal {
	return d.DivRound(d2, int32(DivisionPrecision))
}

// QuoRem does division with remainder
// d.QuoRem(d2,precision) returns quotient q and remainder r such that
//
//	d = d2 * q + r, q an integer multiple of 10^(-precision)
//	0 <= r < abs(d2) * 10 ^(-precision) if d>=0
//	0 >= r > -abs(d2) * 10 ^(-precision) if d<0
//
// Note that precision<0 is allowed as input.
func (d Decimal) QuoRem(d2 Decimal, precision int32) (Decimal, Decimal) {
	return d.quoRem(d2, precision, nil)
}

// quoRem is QuoRem that also sets bb, if not nil, to the divisor in the units of the remainder.
func (d Decimal) quoRem(d2 Decimal, precision int32, bb *big.Int) (Decimal, Decimal) {
	if d2.getValue().Sign() == 0 {
		panic("decimal division by 0")
	}
	scale := -precision
	e := int64(d.exp) - int64(d2.exp) - int64(scale)
	if precision == math.MinInt32 || e > math.MaxInt32 || e < math.MinInt32 {
		panic("overflow in decimal QuoRem")
	}
	if bb == nil {
		bb = new(big.Int)
	}
	var aa big.Int
	var scalerest int32
	// d = a 10^ea
	// d2 = b 10^eb
	if e < 0 {
		aa = *d.getValue()
		bb.Mul(d2.getValue(), pow10(-e))
		scalerest = d.exp
		// now aa = a
		//     bb = b 10^(scale + eb - ea)
	} else {
		aa.Mul(d.getValue(), pow10(e))
		*bb = *d2.getValue()
		scalerest = scale + d2.exp
		// now aa = a ^ (ea - eb - scale)
		//     bb = b
	}
	var q, r big.Int
	q.QuoRem(&aa, bb, &r)
	dq := Decimal{value: &q, exp: scale}
	dr := Decimal{value: &r, exp: scalerest}
	return dq, dr
}

// DivRound divides and rounds to a given precision
// i.e. to an integer multiple of 10^(-precision)
//
//	for a positive quotient digit 5 is rounded up, away from 0
//	if the quotient is negative then digit 5 is rounded down, away from 0
//
// Note that precision<0 is allowed as input.
func (d Decimal) DivRound(d2 Decimal, precision int32) Decimal {
	// quoRem already checks initialization
	var bb big.Int
	q, r := d.quoRem(d2, precision, &bb)
	// the actual rounding decision is based on comparing r*10^precision and d2/2,
	// which is comparing 2*|r.value| and |bb|
	var rv2 big.Int
	rv2.Abs(r.getValue())
	rv2.Lsh(&rv2, 1)
	if rv2.CmpAbs(&bb) < 0 {
		return q
	}

	if d.getValue().Sign()*d2.getValue().Sign() < 0 {
		q.value.Sub(q.value, oneInt)
	} else {
		q.value.Add(q.value, oneInt)
	}
	return q
}

// Mod returns d % d2.
func (d Decimal) Mod(d2 Decimal) Decimal {
	_, r := d.QuoRem(d2, 0)
	return r
}

// NumDigits returns the number of digits of the decimal coefficient (d.Value)
func (d Decimal) NumDigits() int {
	v := d.getValue()
	if v.IsInt64() {
		u := uint64(v.Int64())
		if u == 0 {
			return 1
		}
		if v.Sign() < 0 {
			u = -u
		}
		// Not math.Log10, which rounds an exact 1eN like 1e15 down (see #420).
		// bits.Len64(u)*1233>>12 is floor(log10(2^bits)), the digit count or one less.
		n := bits.Len64(u) * 1233 >> 12
		if u >= pow10Uint64[n] {
			n++
		}
		return n
	}

	estimatedNumDigits := int(float64(v.BitLen()) / math.Log2(10))

	// estimatedNumDigits (lg10) may be off by 1, need to verify
	if v.CmpAbs(pow10(int64(estimatedNumDigits))) >= 0 {
		return estimatedNumDigits + 1
	}

	return estimatedNumDigits
}

// IsInteger returns true when decimal can be represented as an integer value, otherwise, it returns false.
func (d Decimal) IsInteger() bool {
	// The most typical case, all decimal with exponent higher or equal 0 can be represented as integer
	if d.exp >= 0 {
		return true
	}
	v := d.getValue()
	if v.Sign() == 0 {
		return true
	}
	if d.exp >= -18 && v.IsInt64() {
		return v.Int64()%int64(pow10Uint64[-d.exp]) == 0
	}
	// When the exponent is negative we have to check every number after the decimal place
	// If all of them are zeroes, we are sure that given decimal can be represented as an integer
	var r big.Int
	q := new(big.Int).Set(v)
	for z := abs(d.exp); z > 0; z-- {
		q.QuoRem(q, tenInt, &r)
		if r.Sign() != 0 {
			return false
		}
	}
	return true
}

// Abs calculates absolute value of any int32. Used for calculating absolute value of decimal's exponent.
func abs(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

// Cmp compares the numbers represented by d and d2 and returns:
//
//	-1 if d <  d2
//	 0 if d == d2
//	+1 if d >  d2
func (d Decimal) Cmp(d2 Decimal) int {
	if d.exp == d2.exp {
		return d.getValue().Cmp(d2.getValue())
	}
	if s, s2 := d.Sign(), d2.Sign(); s != s2 {
		if s > s2 {
			return 1
		}
		return -1
	} else if s == 0 {
		return 0
	}

	var scaled big.Int
	if d.exp < d2.exp {
		return d.getValue().Cmp(scaled.Mul(d2.getValue(), pow10(int64(d2.exp)-int64(d.exp))))
	}
	return scaled.Mul(d.getValue(), pow10(int64(d.exp)-int64(d2.exp))).Cmp(d2.getValue())
}

// Compare compares the numbers represented by d and d2 and returns:
//
//	-1 if d <  d2
//	 0 if d == d2
//	+1 if d >  d2
func (d Decimal) Compare(d2 Decimal) int {
	return d.Cmp(d2)
}

// Equal returns whether the numbers represented by d and d2 are equal.
func (d Decimal) Equal(d2 Decimal) bool {
	return d.Cmp(d2) == 0
}

// Equals returns whether the numbers represented by d and d2 are equal.
//
// Deprecated: Equals is deprecated, please use Equal method instead.
func (d Decimal) Equals(d2 Decimal) bool {
	return d.Equal(d2)
}

// GreaterThan (GT) returns true when d is greater than d2.
func (d Decimal) GreaterThan(d2 Decimal) bool {
	return d.Cmp(d2) == 1
}

// GreaterThanOrEqual (GTE) returns true when d is greater than or equal to d2.
func (d Decimal) GreaterThanOrEqual(d2 Decimal) bool {
	cmp := d.Cmp(d2)
	return cmp == 1 || cmp == 0
}

// LessThan (LT) returns true when d is less than d2.
func (d Decimal) LessThan(d2 Decimal) bool {
	return d.Cmp(d2) == -1
}

// LessThanOrEqual (LTE) returns true when d is less than or equal to d2.
func (d Decimal) LessThanOrEqual(d2 Decimal) bool {
	cmp := d.Cmp(d2)
	return cmp == -1 || cmp == 0
}

// Sign returns:
//
//	-1 if d <  0
//	 0 if d == 0
//	+1 if d >  0
func (d Decimal) Sign() int {
	return d.getValue().Sign()
}

// IsPositive return
//
//	true if d > 0
//	false if d == 0
//	false if d < 0
func (d Decimal) IsPositive() bool {
	return d.Sign() == 1
}

// IsNegative return
//
//	true if d < 0
//	false if d == 0
//	false if d > 0
func (d Decimal) IsNegative() bool {
	return d.Sign() == -1
}

// IsZero return
//
//	true if d == 0
//	false if d > 0
//	false if d < 0
func (d Decimal) IsZero() bool {
	return d.Sign() == 0
}

// Exponent returns the exponent, or scale component of the decimal.
func (d Decimal) Exponent() int32 {
	return d.exp
}

// Coefficient returns the coefficient of the decimal. It is scaled by 10^Exponent()
func (d Decimal) Coefficient() *big.Int {
	// we copy the coefficient so that mutating the result does not mutate the Decimal.
	return new(big.Int).Set(d.getValue())
}

// CoefficientInt64 returns the coefficient of the decimal as int64. It is scaled by 10^Exponent()
// If coefficient cannot be represented in an int64, the result will be undefined.
func (d Decimal) CoefficientInt64() int64 {
	return d.getValue().Int64()
}

// IntPart returns the integer component of the decimal.
func (d Decimal) IntPart() int64 {
	if v := d.getValue(); d.exp <= 0 && d.exp >= -18 && v.IsInt64() {
		return v.Int64() / int64(pow10Uint64[-d.exp])
	}
	scaledD := d.rescale(0)
	return scaledD.getValue().Int64()
}

// BigInt returns integer component of the decimal as a BigInt.
func (d Decimal) BigInt() *big.Int {
	scaledD := d.rescale(0)
	return scaledD.getValue()
}

// BigFloat returns decimal as BigFloat.
// Be aware that casting decimal to BigFloat might cause a loss of precision.
func (d Decimal) BigFloat() *big.Float {
	f := &big.Float{}
	f.SetString(d.String())
	return f
}

// Rat returns a rational number representation of the decimal.
func (d Decimal) Rat() *big.Rat {
	if d.exp <= 0 {
		// NOTE(vadim): must negate after casting to prevent int32 overflow
		return new(big.Rat).SetFrac(d.getValue(), pow10(-int64(d.exp)))
	}

	num := new(big.Int).Mul(d.getValue(), pow10(int64(d.exp)))
	return new(big.Rat).SetFrac(num, oneInt)
}

// Float64 returns the nearest float64 value for d and a bool indicating
// whether f represents d exactly.
// For more details, see the documentation for big.Rat.Float64
func (d Decimal) Float64() (f float64, exact bool) {
	// Rat() materializes 10^|exp|, which takes forever for huge exponents.
	// Values that far outside the float64 range don't need it: they always
	// round to zero or overflow to infinity.
	sign := d.Sign()
	if sign == 0 {
		return 0, true
	}
	// Both operands are exact here, so the division is correctly rounded like Rat().Float64(),
	// and the result is exact when the 5^k part of 10^k divides the coefficient.
	if v := d.getValue(); d.exp <= 0 && d.exp >= -22 && v.IsInt64() {
		if i := v.Int64(); i >= -1<<53 && i <= 1<<53 {
			return float64(i) / float64Pow10[-d.exp], i%pow5Int64[-d.exp] == 0
		}
	}
	// |d| lies in [10^(magnitude-1), 10^magnitude). float64 spans roughly
	// 1e-324..1e308, so ±400 is safely outside it with margin to spare.
	magnitude := int64(d.NumDigits()) + int64(d.exp)
	if magnitude < -400 {
		return math.Copysign(0, float64(sign)), false
	}
	if magnitude > 400 {
		return math.Inf(sign), false
	}

	return d.Rat().Float64()
}

// InexactFloat64 returns the nearest float64 value for d.
// It doesn't indicate if the returned value represents d exactly.
func (d Decimal) InexactFloat64() float64 {
	f, _ := d.Float64()
	return f
}

// String returns the string representation of the decimal
// with the fixed point.
//
// Example:
//
//	d := New(-12345, -3)
//	println(d.String())
//
// Output:
//
//	-12.345
func (d Decimal) String() string {
	return d.string(TrimTrailingZeros, UseScientificNotation)
}

// StringFixed returns a rounded fixed-point string with places digits after
// the decimal point.
//
// Example:
//
//	NewFromFloat(0).StringFixed(2) // output: "0.00"
//	NewFromFloat(0).StringFixed(0) // output: "0"
//	NewFromFloat(5.45).StringFixed(0) // output: "5"
//	NewFromFloat(5.45).StringFixed(1) // output: "5.5"
//	NewFromFloat(5.45).StringFixed(2) // output: "5.45"
//	NewFromFloat(5.45).StringFixed(3) // output: "5.450"
//	NewFromFloat(545).StringFixed(-1) // output: "550"
//
// Regardless of the UseScientificNotation option, the returned string will never be in scientific notation.
func (d Decimal) StringFixed(places int32) string {
	rounded := d.Round(places)
	return rounded.string(false, false)
}

// StringFixedBank returns a banker rounded fixed-point string with places digits
// after the decimal point.
//
// Example:
//
//	NewFromFloat(0).StringFixedBank(2) // output: "0.00"
//	NewFromFloat(0).StringFixedBank(0) // output: "0"
//	NewFromFloat(5.45).StringFixedBank(0) // output: "5"
//	NewFromFloat(5.45).StringFixedBank(1) // output: "5.4"
//	NewFromFloat(5.45).StringFixedBank(2) // output: "5.45"
//	NewFromFloat(5.45).StringFixedBank(3) // output: "5.450"
//	NewFromFloat(545).StringFixedBank(-1) // output: "540"
//
// Regardless of the UseScientificNotation option, the returned string will never be in scientific notation.
func (d Decimal) StringFixedBank(places int32) string {
	rounded := d.RoundBank(places)
	return rounded.string(false, false)
}

// StringFixedCash returns a Swedish/Cash rounded fixed-point string. For
// more details see the documentation at function RoundCash.
//
// Regardless of the UseScientificNotation option, the returned string will never be in scientific notation.
func (d Decimal) StringFixedCash(interval uint8) string {
	rounded := d.RoundCash(interval)
	return rounded.string(false, false)
}

// Format implements fmt.Formatter, so decimals work with the fmt verbs:
//
//   - %v and %s print String(), and %q, %x and %X quote it or print it in hex, as for any fmt.Stringer.
//   - %f and %F print fixed-point notation. With a precision, as in %.2f, the decimal is rounded like
//     StringFixed. Without one, it's printed exactly, like String() but never in scientific notation.
//     Width and the '-', '+', ' ' and '0' flags work as they do for floats.
//   - %#v prints Go syntax that recreates the decimal exactly, such as decimal.New(314, -2).
//
// Other verbs print fmt's usual error, such as %!d(decimal.Decimal=3.14).
func (d Decimal) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v', 's', 'q', 'x', 'X':
		if verb == 'v' && f.Flag('#') {
			formatString(f, 's', d.goSyntax())
		} else {
			formatString(f, verb, d.String())
		}
	case 'f', 'F':
		d.formatFixed(f)
	default:
		_, _ = fmt.Fprintf(f, "%%!%c(decimal.Decimal=%s)", verb, d.String())
	}
}

// formatString formats s as fmt would for the directive that called Format, with verb in place of its own.
func formatString(f fmt.State, verb rune, s string) {
	// Println and plain %v are the common case, and rebuilding the directive costs 3 allocations.
	_, hasWidth := f.Width()
	_, hasPrec := f.Precision()
	if (verb == 'v' || verb == 's') && !hasWidth && !hasPrec {
		_, _ = io.WriteString(f, s)
		return
	}
	_, _ = fmt.Fprintf(f, fmtDirective(f, verb), s)
}

// fmtDirective rebuilds the directive that called Format, as fmt.FormatString does from Go 1.20.
func fmtDirective(f fmt.State, verb rune) string {
	b := []byte{'%'}
	for _, flag := range "+-# 0" {
		if f.Flag(int(flag)) {
			b = append(b, byte(flag))
		}
	}
	if width, ok := f.Width(); ok {
		b = strconv.AppendInt(b, int64(width), 10)
	}
	if prec, ok := f.Precision(); ok {
		b = strconv.AppendInt(append(b, '.'), int64(prec), 10)
	}
	return string(append(b, string(verb)...))
}

func (d Decimal) formatFixed(f fmt.State) {
	var s string
	if prec, ok := f.Precision(); ok {
		s = d.StringFixed(int32(prec))
	} else {
		s = d.string(TrimTrailingZeros, false)
	}

	sign := ""
	switch {
	case s[0] == '-':
		sign, s = "-", s[1:]
	case f.Flag('+'):
		sign = "+"
	case f.Flag(' '):
		sign = " "
	}
	// Before Go 1.23, fmt keeps the width of an earlier directive when ok is false.
	pad := 0
	if width, ok := f.Width(); ok {
		pad = width - len(sign) - len(s)
	}
	switch {
	case pad <= 0:
		s = sign + s
	case f.Flag('-'):
		s = sign + s + strings.Repeat(" ", pad)
	case f.Flag('0'):
		s = sign + strings.Repeat("0", pad) + s
	default:
		s = strings.Repeat(" ", pad) + sign + s
	}
	_, _ = io.WriteString(f, s)
}

func (d Decimal) goSyntax() string {
	switch {
	case d.value == nil:
		return "decimal.Decimal{}"
	case d.value.IsInt64():
		return fmt.Sprintf("decimal.New(%d, %d)", d.value.Int64(), d.exp)
	default:
		// Shift instead of an exponent in the string, which RequireFromString caps at MaxDecodeExponent.
		return fmt.Sprintf("decimal.RequireFromString(%q).Shift(%d)", d.value.String(), d.exp)
	}
}

// Round rounds the decimal to places decimal places.
// If places < 0, it will round the integer part to the nearest 10^(-places).
//
// Example:
//
//	NewFromFloat(5.45).Round(1).String() // output: "5.5"
//	NewFromFloat(545).Round(-1).String() // output: "550" (with UseScientificNotation false, "5.5E2" if true)
func (d Decimal) Round(places int32) Decimal {
	if d.exp == -places {
		return d
	}
	// truncate to places + 1
	ret := d.rescale(-places - 1)

	// add sign(d) * 0.5
	if ret.value.Sign() < 0 {
		ret.value.Sub(ret.value, fiveInt)
	} else {
		ret.value.Add(ret.value, fiveInt)
	}

	// truncate towards zero
	ret.value.Quo(ret.value, tenInt)
	ret.exp++

	return ret
}

// RoundCeil rounds the decimal towards +infinity.
//
// Example:
//
//	NewFromFloat(545).RoundCeil(-2).String()   // output: "600"
//	NewFromFloat(500).RoundCeil(-2).String()   // output: "500"
//	NewFromFloat(1.1001).RoundCeil(2).String() // output: "1.11"
//	NewFromFloat(-1.454).RoundCeil(1).String() // output: "-1.4"
func (d Decimal) RoundCeil(places int32) Decimal {
	if d.exp >= -places {
		return d.rescale(-places)
	}

	// q is d truncated to places, r is not 0 when digits were dropped and has the sign of d
	var r big.Int
	q := new(big.Int)
	q.QuoRem(d.getValue(), pow10(-int64(places)-int64(d.exp)), &r)
	if r.Sign() > 0 {
		q.Add(q, oneInt)
	} else if q.Sign() == 0 && r.Sign() != 0 {
		// reflect.DeepEqual tells an empty word slice from nil, keep the empty one that
		// rounding a nonzero value to zero used to leave
		q.SetBits([]big.Word{})
	}

	return Decimal{value: q, exp: -places}
}

// RoundFloor rounds the decimal towards -infinity.
//
// Example:
//
//	NewFromFloat(545).RoundFloor(-2).String()   // output: "500"
//	NewFromFloat(-500).RoundFloor(-2).String()   // output: "-500"
//	NewFromFloat(1.1001).RoundFloor(2).String() // output: "1.1"
//	NewFromFloat(-1.454).RoundFloor(1).String() // output: "-1.5"
func (d Decimal) RoundFloor(places int32) Decimal {
	if d.exp >= -places {
		return d.rescale(-places)
	}

	// q is d truncated to places, r is not 0 when digits were dropped and has the sign of d
	var r big.Int
	q := new(big.Int)
	q.QuoRem(d.getValue(), pow10(-int64(places)-int64(d.exp)), &r)
	if r.Sign() < 0 {
		q.Sub(q, oneInt)
	} else if q.Sign() == 0 && r.Sign() != 0 {
		// reflect.DeepEqual tells an empty word slice from nil, keep the empty one that
		// rounding a nonzero value to zero used to leave
		q.SetBits([]big.Word{})
	}

	return Decimal{value: q, exp: -places}
}

// RoundUp rounds the decimal away from zero.
//
// Example:
//
//	NewFromFloat(545).RoundUp(-2).String()   // output: "600"
//	NewFromFloat(500).RoundUp(-2).String()   // output: "500"
//	NewFromFloat(1.1001).RoundUp(2).String() // output: "1.11"
//	NewFromFloat(-1.454).RoundUp(1).String() // output: "-1.5"
func (d Decimal) RoundUp(places int32) Decimal {
	if d.exp >= -places {
		return d.rescale(-places)
	}

	// q is d truncated to places, r is not 0 when digits were dropped and has the sign of d
	var r big.Int
	q := new(big.Int)
	q.QuoRem(d.getValue(), pow10(-int64(places)-int64(d.exp)), &r)
	if r.Sign() > 0 {
		q.Add(q, oneInt)
	} else if r.Sign() < 0 {
		q.Sub(q, oneInt)
	}

	return Decimal{value: q, exp: -places}
}

// RoundDown rounds the decimal towards zero.
//
// Example:
//
//	NewFromFloat(545).RoundDown(-2).String()   // output: "500"
//	NewFromFloat(-500).RoundDown(-2).String()   // output: "-500"
//	NewFromFloat(1.1001).RoundDown(2).String() // output: "1.1"
//	NewFromFloat(-1.454).RoundDown(1).String() // output: "-1.4"
func (d Decimal) RoundDown(places int32) Decimal {
	return d.rescale(-places)
}

// RoundBank rounds the decimal to places decimal places.
// If the final digit to round is equidistant from the nearest two integers the
// rounded value is taken as the even number
//
// If places < 0, it will round the integer part to the nearest 10^(-places).
//
// Examples:
//
//	NewFromFloat(5.45).RoundBank(1).String() // output: "5.4"
//	NewFromFloat(545).RoundBank(-1).String() // output: "540"
//	NewFromFloat(5.46).RoundBank(1).String() // output: "5.5"
//	NewFromFloat(546).RoundBank(-1).String() // output: "550"
//	NewFromFloat(5.55).RoundBank(1).String() // output: "5.6"
//	NewFromFloat(555).RoundBank(-1).String() // output: "560"
func (d Decimal) RoundBank(places int32) Decimal {

	round := d.Round(places)

	// it is a tie when twice the k dropped digits equal 10^k
	if k := -int64(places) - int64(d.exp); k > 0 && round.getValue().Bit(0) != 0 {
		var dropped big.Int
		dropped.Rem(d.getValue(), pow10(k))
		if dropped.Abs(&dropped).Lsh(&dropped, 1).Cmp(pow10(k)) == 0 {
			if round.getValue().Sign() < 0 {
				round.value = new(big.Int).Add(round.getValue(), oneInt)
			} else {
				round.value = new(big.Int).Sub(round.getValue(), oneInt)
			}
		}
	}

	return round
}

// RoundCash aka Cash/Penny/öre rounding rounds decimal to a specific
// interval. The amount payable for a cash transaction is rounded to the nearest
// multiple of the minimum currency unit available. The following intervals are
// available: 5, 10, 25, 50 and 100; any other number throws a panic.
//
//	  5:   5 cent rounding 3.43 => 3.45
//	 10:  10 cent rounding 3.45 => 3.50 (5 gets rounded up)
//	 25:  25 cent rounding 3.41 => 3.50
//	 50:  50 cent rounding 3.75 => 4.00
//	100: 100 cent rounding 3.50 => 4.00
//
// For more details: https://en.wikipedia.org/wiki/Cash_rounding
func (d Decimal) RoundCash(interval uint8) Decimal {
	var iVal *big.Int
	switch interval {
	case 5:
		iVal = twentyInt
	case 10:
		iVal = tenInt
	case 25:
		iVal = fourInt
	case 50:
		iVal = twoInt
	case 100:
		iVal = oneInt
	default:
		panic(fmt.Sprintf("Decimal does not support this Cash rounding interval `%d`. Supported: 5, 10, 25, 50, 100", interval))
	}
	dVal := Decimal{
		value: iVal,
	}

	return d.Mul(dVal).Round(0).DivRound(dVal, 2)
}

// Floor returns the nearest integer value less than or equal to d.
func (d Decimal) Floor() Decimal {
	if d.exp >= 0 {
		return d
	}

	// NOTE(vadim): must negate after casting to prevent int32 overflow
	z := new(big.Int).Div(d.getValue(), pow10(-int64(d.exp)))
	return Decimal{value: z, exp: 0}
}

// Ceil returns the nearest integer value greater than or equal to d.
func (d Decimal) Ceil() Decimal {
	if d.exp >= 0 {
		return d
	}

	// NOTE(vadim): must negate after casting to prevent int32 overflow
	z, m := new(big.Int).DivMod(d.getValue(), pow10(-int64(d.exp)), new(big.Int))
	if m.Sign() != 0 {
		z.Add(z, oneInt)
	}
	return Decimal{value: z, exp: 0}
}

// Truncate truncates off digits from the number, without rounding.
//
// If precision >= 0, it specifies the number of decimal places to keep.
// If precision < 0, it truncates the integer part to the nearest 10^(-precision)
// towards zero.
//
// Example:
//
//	decimal.RequireFromString("123.456").Truncate(2).String() // output: "123.45"
//	decimal.RequireFromString("5432").Truncate(-2).String()   // output: "5400"
//	decimal.RequireFromString("-5432").Truncate(-2).String()  // output: "-5400"
func (d Decimal) Truncate(precision int32) Decimal {
	if -precision > d.exp {
		return d.rescale(-precision)
	}
	return d
}

// StringScaled first scales the decimal then calls .String() on it.
//
// Deprecated: buggy and unintuitive. Use StringFixed instead.
func (d Decimal) StringScaled(exp int32) string {
	return d.rescale(exp).String()
}

func (d Decimal) string(trimTrailingZeros, useScientificNotation bool) string {
	if d.exp == 0 {
		return d.getValue().String()
	}
	if d.exp >= 0 {
		if useScientificNotation {
			return d.ScientificNotationString()
		}
		return d.rescale(0).value.String()
	}

	str := d.getValue().String()
	sign := ""
	if str[0] == '-' {
		sign, str = "-", str[1:]
	}

	var intPart, leadingZeros, fractionalPart string

	// NOTE(vadim): this cast to int will cause bugs if d.exp == INT_MIN
	// and you are on a 32-bit machine. Won't fix this super-edge case.
	dExpInt := int(d.exp)
	if len(str) > -dExpInt {
		intPart = str[:len(str)+dExpInt]
		fractionalPart = str[len(str)+dExpInt:]
	} else {
		intPart = "0"

		num0s := -dExpInt - len(str)
		if num0s <= len(zeros) {
			leadingZeros = zeros[:num0s]
		} else {
			leadingZeros = strings.Repeat("0", num0s)
		}
		fractionalPart = str
	}

	if trimTrailingZeros {
		i := len(fractionalPart) - 1
		for ; i >= 0; i-- {
			if fractionalPart[i] != '0' {
				break
			}
		}
		fractionalPart = fractionalPart[:i+1]
		if fractionalPart == "" {
			leadingZeros = ""
		}
	}

	if len(leadingZeros)+len(fractionalPart) > 0 {
		return sign + intPart + "." + leadingZeros + fractionalPart
	}
	return sign + intPart
}

// ScientificNotationString serializes the decimal into standard scientific notation.
//
// The notation is normalized to have one non-zero digit followed by a decimal point and
// the remaining significant digits followed by "E" and the base-10 exponent.
//
// A zero, which has no significant digits, is simply serialized to "0".
func (d Decimal) ScientificNotationString() string {
	exp := int(d.exp)
	intStr := new(big.Int).Abs(d.getValue()).String()
	if intStr == "0" {
		return intStr
	}
	first := intStr[0]
	var remaining string
	if len(intStr) > 1 {
		remaining = "." + intStr[1:]
		exp = exp + len(intStr) - 1
	}
	number := string(first) + remaining + "E" + strconv.Itoa(exp)
	if d.value.Sign() < 0 {
		return "-" + number
	}
	return number
}

// Min returns the smallest Decimal that was passed in the arguments.
//
// To call this function with an array, you must do:
//
//	Min(arr[0], arr[1:]...)
//
// This makes it harder to accidentally call Min with 0 arguments.
func Min(first Decimal, rest ...Decimal) Decimal {
	ans := first
	for _, item := range rest {
		if item.Cmp(ans) < 0 {
			ans = item
		}
	}
	return ans
}

// Max returns the largest Decimal that was passed in the arguments.
//
// To call this function with an array, you must do:
//
//	Max(arr[0], arr[1:]...)
//
// This makes it harder to accidentally call Max with 0 arguments.
func Max(first Decimal, rest ...Decimal) Decimal {
	ans := first
	for _, item := range rest {
		if item.Cmp(ans) > 0 {
			ans = item
		}
	}
	return ans
}

// Sum returns the combined total of the provided first and rest Decimals
func Sum(first Decimal, rest ...Decimal) Decimal {
	if len(rest) == 0 {
		return first
	}

	// Add returns a new value, so the following items can be added to it in place
	total := first.Add(rest[0])
	last := len(rest) - 1
	if last == 0 {
		return total
	}
	var scaled big.Int
	for _, item := range rest[1:last] {
		switch {
		case item.exp < total.exp:
			total = total.rescale(item.exp)
			total.value.Add(total.value, item.getValue())
		case item.exp > total.exp:
			total.value.Add(total.value, scaled.Mul(item.getValue(), pow10(int64(item.exp)-int64(total.exp))))
		default:
			total.value.Add(total.value, item.getValue())
		}
	}

	// the last Add builds the result the same way as adding one item at a time does
	return total.Add(rest[last])
}

// Avg returns the average value of the provided first and rest Decimals
func Avg(first Decimal, rest ...Decimal) Decimal {
	count := New(int64(len(rest)+1), 0)
	sum := Sum(first, rest...)
	return sum.Div(count)
}

// RescalePair rescales two decimals to common exponential value (minimal exp of both decimals)
func RescalePair(d1 Decimal, d2 Decimal) (Decimal, Decimal) {
	if d1.exp < d2.exp {
		return d1, d2.rescale(d1.exp)
	} else if d1.exp > d2.exp {
		return d1.rescale(d2.exp), d2
	}

	return d1, d2
}
