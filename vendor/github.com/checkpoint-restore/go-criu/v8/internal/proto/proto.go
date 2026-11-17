// Package proto provides a small helper for constructing pointers to
// scalar values, as used by proto2-generated optional/required fields.
//
// protobuf-go-lite does not ship the proto.String/Int32/Bool constructors
// that google.golang.org/protobuf/proto offers, so Ptr replaces them.
package proto

// Ptr returns a pointer to v. It is handy for setting proto2 optional and
// required scalar fields, e.g. Ptr("foo"), Ptr(true), Ptr[int32](4).
//
// TODO: drop this in favor of the built-in new(v) once the minimum
// supported Go version is 1.26 or later.
func Ptr[T any](v T) *T { return &v }
