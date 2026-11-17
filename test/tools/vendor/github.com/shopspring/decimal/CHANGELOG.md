## Decimal v1.6.0

#### BEHAVIOR CHANGES
- `Decimal` implements `fmt.Formatter`. `%#v` prints Go code that recreates the value, like `decimal.New(314, -2)`, instead of the struct fields, and verbs that don't apply, like `%d`, print `%!d(decimal.Decimal=3.14)`. `%v`, `%s`, `%q`, `%x` and `fmt.Println` print the same as before [#463](https://github.com/shopspring/decimal/pull/463)
- `Ln` rounds correctly, so some results change in the last digit: `Ln` of 10 to 5 places is `2.30259`, not `2.30258` [#471](https://github.com/shopspring/decimal/pull/471)
- `ExpHullAbrham` returns `overallPrecision` significant figures for every input. Negative inputs used to lose digits or return 0, like `-50` at precision 10, and large inputs returned extra digits [#429](https://github.com/shopspring/decimal/pull/429) [#470](https://github.com/shopspring/decimal/pull/470)
- `Scan` converts `float32` with `NewFromFloat32`, so `54.33` stays `54.33` instead of becoming `54.33000183105469`. NaN and infinite floats return an error instead of panicking [#469](https://github.com/shopspring/decimal/pull/469)
- On Go 1.10 to 1.12, `NewFromFloat` can pick a different shortest form in rare cases ([golang/go#29491](https://github.com/golang/go/issues/29491)). It still converts back to the same float [#431](https://github.com/shopspring/decimal/pull/431)

#### FEATURES
- Add BSON Decimal128 support for `Decimal` and `NullDecimal` with the official MongoDB Go driver v2, without a new dependency [#460](https://github.com/shopspring/decimal/pull/460)
- Add `%f` formatting with precision, width and flags, like `%.2f` and `%08.2f` [#463](https://github.com/shopspring/decimal/pull/463)
- Add `Normalize`, which strips trailing zeros so equal decimals also match with `reflect.DeepEqual`, testify and gomock [#464](https://github.com/shopspring/decimal/pull/464)
- Add `Sqrt` and `SqrtRound` [#465](https://github.com/shopspring/decimal/pull/465)
- Add `Root` and `RootRound` for n-th roots [#473](https://github.com/shopspring/decimal/pull/473)
- Add `NewFromBigFloat` constructor [#457](https://github.com/shopspring/decimal/pull/457)
- `Scan` accepts `Decimal`, `big.Int`, `*big.Int` and all integer sizes, as passed by ClickHouse and other drivers [#469](https://github.com/shopspring/decimal/pull/469)

#### ENHANCEMENTS
- `NewFromFloat` and `NewFromFloat32` are 3 to 6 times faster. They use `strconv` instead of a vendored copy of it from 2009, which is removed [#431](https://github.com/shopspring/decimal/pull/431)
- `Ln` is 3 to 14 times faster [#471](https://github.com/shopspring/decimal/pull/471)
- Slightly faster `IsInteger` and `Ceil` [#299](https://github.com/shopspring/decimal/pull/299)
- `NewFromString` accepts the Unicode minus sign `−` (U+2212) [#428](https://github.com/shopspring/decimal/pull/428)
- Scanning NULL into a `Decimal` returns an error that points to `NullDecimal` or `*Decimal` [#474](https://github.com/shopspring/decimal/pull/474)
- Document that global settings aren't goroutine-safe, and list them with their per-call alternatives in the README [#466](https://github.com/shopspring/decimal/pull/466)
- README: show how to control the JSON format of one field, add `udecimal` to the alternative libraries, and update the Go doc and playground links [#475](https://github.com/shopspring/decimal/pull/475) [#476](https://github.com/shopspring/decimal/pull/476) [#264](https://github.com/shopspring/decimal/pull/264)
- Fix doc examples that were wrong or didn't compile [#468](https://github.com/shopspring/decimal/pull/468)
- Fix benchmarks that measured their setup or the wrong function [#467](https://github.com/shopspring/decimal/pull/467)

#### BUGFIXES
- `QuoRem` and `DivRound` panic on precision `math.MinInt32` instead of returning a wrong quotient [#472](https://github.com/shopspring/decimal/pull/472)

## Decimal v1.5.0

#### BEHAVIOR CHANGES
- `NewFromString` and `UnmarshalBinary`, and so JSON, text, SQL, gob and Spanner decoding, reject exponents larger than the new `MaxDecodeExponent` setting (default 10,000). Such input could hang the process or use gigabytes of memory. Raise `MaxDecodeExponent` if you need larger exponents [#440](https://github.com/shopspring/decimal/pull/440)
- `Pow`, `PowWithPrecision`, `PowInt32` and `PowBigInt` round correctly. Results for fractional exponents have `PowPrecisionNegativeExponent` (16) decimal places, and negative powers that used to return 0 keep 16 significant digits, so `10^-18` is `0.000000000000000001`. Results too large to represent return an error, or 0 from `Pow`, instead of hanging or panicking [#439](https://github.com/shopspring/decimal/pull/439)
- `RoundUp`, `RoundDown`, `RoundCeil` and `RoundFloor` always return exponent `-places`, like `Round`. Values don't change, but `Exponent()`, `Coefficient()` and `String()` with `TrimTrailingZeros = false` can [#426](https://github.com/shopspring/decimal/pull/426) [#441](https://github.com/shopspring/decimal/pull/441)
- `Truncate` supports negative precision: `5432` truncated to `-2` is `5400`. Before, it returned `5432` unchanged [#426](https://github.com/shopspring/decimal/pull/426)
- `Shift` panics when the exponent overflows an int32, instead of wrapping around [#437](https://github.com/shopspring/decimal/pull/437)
- `NewFromString` rejects a sign right after a leading decimal point, such as `".-5"` [#447](https://github.com/shopspring/decimal/pull/447)

#### FEATURES
- Add `TrimTrailingZeros` and `UseScientificNotation` settings for string output [#389](https://github.com/shopspring/decimal/pull/389) [#416](https://github.com/shopspring/decimal/pull/416)
- Add Spanner support with `EncodeSpanner` and `DecodeSpanner` for `Decimal` and `NullDecimal` [#395](https://github.com/shopspring/decimal/pull/395)

#### ENHANCEMENTS
- Most operations are faster and allocate less, about twice as fast on average across the benchmarks [#444](https://github.com/shopspring/decimal/pull/444) [#445](https://github.com/shopspring/decimal/pull/445) [#446](https://github.com/shopspring/decimal/pull/446) [#447](https://github.com/shopspring/decimal/pull/447) [#448](https://github.com/shopspring/decimal/pull/448) [#449](https://github.com/shopspring/decimal/pull/449) [#451](https://github.com/shopspring/decimal/pull/451)
- Faster `UnmarshalJSON`, `Scan` and `NewFromString` [#403](https://github.com/shopspring/decimal/pull/403)
- `ExpHullAbrham` raises its partial sum to a power by squaring. Large inputs that took minutes now take seconds [#450](https://github.com/shopspring/decimal/pull/450)
- Document that `NewFromFloat` returns the shortest decimal that converts back to the same float, and how to convert large whole numbers exactly [#442](https://github.com/shopspring/decimal/pull/442)
- Split `decimal.go` into `decimal.go`, `math.go` and `encoding.go` [#452](https://github.com/shopspring/decimal/pull/452)
- Lint with golangci-lint in CI, and test on Go 1.26 and 1.27 [#453](https://github.com/shopspring/decimal/pull/453) [#454](https://github.com/shopspring/decimal/pull/454)

#### BUGFIXES
- Fix `Float64` hanging on decimals with huge exponents [#438](https://github.com/shopspring/decimal/pull/438)
- Fix `ExpHullAbrham` returning `10^exp` instead of 1 for tiny inputs [#433](https://github.com/shopspring/decimal/pull/433)
- Fix a data race in `ExpTaylor` [#414](https://github.com/shopspring/decimal/pull/414)
- Fix `NumDigits` for some exact powers of ten [#425](https://github.com/shopspring/decimal/pull/425)
- Fix the `omitzero` struct tag for `Decimal` fields in nested structs [#415](https://github.com/shopspring/decimal/pull/415)
- Fix `RoundCash` giving wrong results with a low `DivisionPrecision` [#443](https://github.com/shopspring/decimal/pull/443)
- `NullDecimal` is no longer `Valid` after a failed `Scan`, `UnmarshalJSON` or `DecodeSpanner` [#443](https://github.com/shopspring/decimal/pull/443)

## Decimal v1.4.0
#### BREAKING
- Drop support for Go version older than 1.10 [#361](https://github.com/shopspring/decimal/pull/361)

#### FEATURES
- Add implementation of natural logarithm [#339](https://github.com/shopspring/decimal/pull/339) [#357](https://github.com/shopspring/decimal/pull/357)
- Add improved implementation of power operation [#358](https://github.com/shopspring/decimal/pull/358)
- Add Compare method which forwards calls to Cmp [#346](https://github.com/shopspring/decimal/pull/346)
- Add NewFromBigRat constructor [#288](https://github.com/shopspring/decimal/pull/288)
- Add NewFromUint64 constructor [#352](https://github.com/shopspring/decimal/pull/352)

#### ENHANCEMENTS
- Migrate to Github Actions [#245](https://github.com/shopspring/decimal/pull/245) [#340](https://github.com/shopspring/decimal/pull/340)
- Fix examples for RoundDown, RoundFloor, RoundUp, and RoundCeil [#285](https://github.com/shopspring/decimal/pull/285) [#328](https://github.com/shopspring/decimal/pull/328) [#341](https://github.com/shopspring/decimal/pull/341)
- Use Godoc standard to mark deprecated Equals and StringScaled methods [#342](https://github.com/shopspring/decimal/pull/342)
- Removed unnecessary min function for RescalePair method [#265](https://github.com/shopspring/decimal/pull/265)
- Avoid reallocation of initial slice in MarshalBinary (GobEncode) [#355](https://github.com/shopspring/decimal/pull/355)
- Optimize NumDigits method [#301](https://github.com/shopspring/decimal/pull/301) [#356](https://github.com/shopspring/decimal/pull/356)
- Optimize BigInt method [#359](https://github.com/shopspring/decimal/pull/359)
- Support scanning uint64 [#131](https://github.com/shopspring/decimal/pull/131) [#364](https://github.com/shopspring/decimal/pull/364)
- Add docs section with alternative libraries [#363](https://github.com/shopspring/decimal/pull/363)

#### BUGFIXES
- Fix incorrect calculation of decimal modulo [#258](https://github.com/shopspring/decimal/pull/258) [#317](https://github.com/shopspring/decimal/pull/317)
- Allocate new(big.Int) in Copy method to deeply clone it [#278](https://github.com/shopspring/decimal/pull/278)
- Fix overflow edge case in QuoRem method [#322](https://github.com/shopspring/decimal/pull/322)

## Decimal v1.3.1

#### ENHANCEMENTS
- Reduce memory allocation in case of initialization from big.Int [#252](https://github.com/shopspring/decimal/pull/252)

#### BUGFIXES
- Fix binary marshalling of decimal zero value  [#253](https://github.com/shopspring/decimal/pull/253)

## Decimal v1.3.0

#### FEATURES
- Add NewFromFormattedString initializer [#184](https://github.com/shopspring/decimal/pull/184)
- Add NewNullDecimal initializer [#234](https://github.com/shopspring/decimal/pull/234)
- Add implementation of natural exponent function (Taylor, Hull-Abraham) [#229](https://github.com/shopspring/decimal/pull/229)
- Add RoundUp, RoundDown, RoundCeil, RoundFloor methods [#196](https://github.com/shopspring/decimal/pull/196) [#202](https://github.com/shopspring/decimal/pull/202) [#220](https://github.com/shopspring/decimal/pull/220)
- Add XML support for NullDecimal [#192](https://github.com/shopspring/decimal/pull/192)
- Add IsInteger method [#179](https://github.com/shopspring/decimal/pull/179)
- Add Copy helper method [#123](https://github.com/shopspring/decimal/pull/123)
- Add InexactFloat64 helper method [#205](https://github.com/shopspring/decimal/pull/205)
- Add CoefficientInt64 helper method [#244](https://github.com/shopspring/decimal/pull/244)

#### ENHANCEMENTS
- Performance optimization of NewFromString init method [#198](https://github.com/shopspring/decimal/pull/198)
- Performance optimization of Abs and Round methods [#240](https://github.com/shopspring/decimal/pull/240)
- Additional tests (CI) for ppc64le architecture [#188](https://github.com/shopspring/decimal/pull/188)

#### BUGFIXES
- Fix rounding in FormatFloat fallback path (roundShortest method, fix taken from Go main repository) [#161](https://github.com/shopspring/decimal/pull/161)
- Add slice range checks to UnmarshalBinary method [#232](https://github.com/shopspring/decimal/pull/232)

## Decimal v1.2.0

#### BREAKING
- Drop support for Go version older than 1.7 [#172](https://github.com/shopspring/decimal/pull/172)

#### FEATURES
- Add NewFromInt and NewFromInt32 initializers [#72](https://github.com/shopspring/decimal/pull/72)
- Add support for Go modules [#157](https://github.com/shopspring/decimal/pull/157)
- Add BigInt, BigFloat helper methods [#171](https://github.com/shopspring/decimal/pull/171)

#### ENHANCEMENTS
- Memory usage optimization [#160](https://github.com/shopspring/decimal/pull/160)
- Updated travis CI golang versions [#156](https://github.com/shopspring/decimal/pull/156)
- Update documentation [#173](https://github.com/shopspring/decimal/pull/173)
- Improve code quality [#174](https://github.com/shopspring/decimal/pull/174)

#### BUGFIXES
- Revert remove insignificant digits [#159](https://github.com/shopspring/decimal/pull/159)
- Remove 15 interval for RoundCash [#166](https://github.com/shopspring/decimal/pull/166)
