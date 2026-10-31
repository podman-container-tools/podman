# decimal

[![ci](https://github.com/shopspring/decimal/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/shopspring/decimal/actions/workflows/ci.yml)
[![GoDoc](https://pkg.go.dev/badge/github.com/shopspring/decimal)](https://pkg.go.dev/github.com/shopspring/decimal) 

Arbitrary-precision fixed-point decimal numbers in go.

_Note:_ Decimal library can "only" represent numbers with a maximum of 2^31 digits after the decimal point.
Results that aren't exact, like `Div` and `Sqrt`, are rounded to 16 decimal places by default, and decoded input is
limited to exponents up to 10,000. See [Global settings](#global-settings).

## Features

 * The zero-value is 0, and is safe to use without initialization
 * Addition, subtraction, multiplication with no loss of precision
 * Division with specified precision
 * Database/sql serialization/deserialization
 * JSON and XML serialization/deserialization
 * MongoDB BSON serialization/deserialization as Decimal128, with the official Go driver v2

## Install

Run `go get github.com/shopspring/decimal`

## Requirements 

Decimal library requires Go version `>=1.10`

## Documentation

https://pkg.go.dev/github.com/shopspring/decimal


## Usage

```go
package main

import (
	"fmt"
	"github.com/shopspring/decimal"
)

func main() {
	price, err := decimal.NewFromString("136.02")
	if err != nil {
		panic(err)
	}

	quantity := decimal.NewFromInt(3)

	fee, _ := decimal.NewFromString(".035")
	taxRate, _ := decimal.NewFromString(".08875")

	subtotal := price.Mul(quantity)

	preTax := subtotal.Mul(fee.Add(decimal.NewFromFloat(1)))

	total := preTax.Mul(taxRate.Add(decimal.NewFromFloat(1)))

	fmt.Println("Subtotal:", subtotal)                      // Subtotal: 408.06
	fmt.Println("Pre-tax:", preTax)                         // Pre-tax: 422.3421
	fmt.Println("Taxes:", total.Sub(preTax))                // Taxes: 37.482861375
	fmt.Println("Total:", total)                            // Total: 459.824961375
	fmt.Println("Tax rate:", total.Sub(preTax).Div(preTax)) // Tax rate: 0.08875
}
```

## Global settings

These package-level variables change how some functions behave:

| Variable | Default | What it changes | Per-call alternative |
|---|---|---|---|
| `DivisionPrecision` | `16` | Decimal places in `Div` and `Sqrt` results, and in `Avg`, `Atan` and `Tan` | `DivRound`, `SqrtRound` |
| `PowPrecisionNegativeExponent` | `16` | Decimal places in `Pow`, `PowInt32` and `PowBigInt` results for negative or fractional exponents | `PowWithPrecision` |
| `TrimTrailingZeros` | `true` | Whether `String`, and so JSON, text and SQL output, drops trailing zeros: `2.50` prints as `2.5` | `StringFixed` |
| `UseScientificNotation` | `false` | Whether `String` prints numbers stored with a positive exponent in scientific notation, like `1.2E3` | `StringFixed` |
| `MarshalJSONWithoutQuotes` | `false` | Whether JSON output is a number instead of a string | |
| `ExpMaxIterations` | `1000` | Iteration limit of `ExpHullAbrham` | |
| `MaxDecodeExponent` | `10000` | Largest exponent size that `NewFromString` and the decoders accept | |

Set them once at program start. Changing one while other goroutines use the library is a data race.

## Alternative libraries

When working with decimal numbers, you might face problems this library is not perfectly suited for. 
Fortunately, thanks to the wonderful community we have a dozen other libraries that you can choose from.  
Explore other alternatives to find the one that best fits your needs :)  

* [cockroachdb/apd](https://github.com/cockroachdb/apd) - arbitrary precision, mutable and rich API similar to `big.Int`, more performant than this library 
* [alpacahq/alpacadecimal](https://github.com/alpacahq/alpacadecimal) - high performance, low precision (12 digits), fully compatible API with this library 
* [govalues/decimal](https://github.com/govalues/decimal) - high performance, zero-allocation, low precision (19 digits)
* [quagmt/udecimal](https://github.com/quagmt/udecimal) - high performance, mostly zero-allocation, up to 19 digits after the decimal point
* [greatcloak/decimal](https://github.com/greatcloak/decimal) - fork focusing on billing and e-commerce web application related use cases, includes out-of-the-box BSON marshaling support

## FAQ

#### Why don't you just use float64?

Because float64 (or any binary floating point type, actually) can't represent
numbers such as `0.1` exactly.

Consider this code: https://go.dev/play/p/TQBd4yJe6B You might expect that
it prints out `10`, but it actually prints `9.999999999999831`. Over time,
these small errors can really add up!

#### Why don't you just use big.Rat?

big.Rat is fine for representing rational numbers, but Decimal is better for
representing money. Why? Here's a (contrived) example:

Let's say you use big.Rat, and you have two numbers, x and y, both
representing 1/3, and you have `z = 1 - x - y = 1/3`. If you print each one
out, the string output has to stop somewhere (let's say it stops at 3 decimal
digits, for simplicity), so you'll get 0.333, 0.333, and 0.333. But where did
the other 0.001 go?

Here's the above example as code: https://go.dev/play/p/lCZZs0w9KE

With Decimal, the strings being printed out represent the number exactly. So,
if you have `x = y = 1/3` (with precision 3), they will actually be equal to
0.333, and when you do `z = 1 - x - y`, `z` will be equal to .334. No money is
unaccounted for!

You still have to be careful. If you want to split a number `N` 3 ways, you
can't just send `N/3` to three different people. You have to pick one to send
`N - (2/3*N)` to. That person will receive the fraction of a penny remainder.

But, it is much easier to be careful with Decimal than with big.Rat.

#### Why isn't the API similar to big.Int's?

big.Int's API is built to reduce the number of memory allocations for maximal
performance. This makes sense for its use-case, but the trade-off is that the
API is awkward and easy to misuse.

For example, to add two big.Ints, you do: `z := new(big.Int).Add(x, y)`. A
developer unfamiliar with this API might try to do `z := a.Add(a, b)`. This
modifies `a` and sets `z` as an alias for `a`, which they might not expect. It
also modifies any other aliases to `a`.

Here's an example of the subtle bugs you can introduce with big.Int's API:
https://go.dev/play/p/x2R_78pa8r

In contrast, it's difficult to make such mistakes with decimal. Decimals
behave like other go numbers types: even though `a = b` will not deep copy
`b` into `a`, it is impossible to modify a Decimal, since all Decimal methods
return new Decimals and do not modify the originals. The downside is that
this causes extra allocations, so Decimal is less performant.  My assumption
is that if you're using Decimals, you probably care more about correctness
than performance.

#### How do I control the JSON format of one field?

Wrap the field in a small type with its own `MarshalJSON`. For example, to always write money with 2 decimal places:

```go
type Money struct{ decimal.Decimal }

func (m Money) MarshalJSON() ([]byte, error) {
	return []byte(`"` + m.StringFixed(2) + `"`), nil
}
```

A `Money` field marshals as `"12.50"`, other decimals keep their usual format, and unmarshaling still works through
the embedded `Decimal`. To write one field as a JSON number instead of a string, return `[]byte(m.String())`.
This is safer than the global settings, which change every decimal in the program.

## License

The MIT License (MIT)

This is a heavily modified fork of [fpd.Decimal](https://github.com/oguzbilgic/fpd), which was also released under the MIT License.
