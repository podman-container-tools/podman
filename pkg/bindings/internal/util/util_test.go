package util

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSimpleType(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"string", "foo", true},
		{"bool", true, true},
		{"int", int(1), true},
		{"int64", int64(1), true},
		{"uint", uint(1), true},
		{"uint64", uint64(1), true},
		{"stringer", time.Second, true}, // time.Duration implements fmt.Stringer
		{"float is not simple", 1.5, false},
		{"slice is not simple", []string{"a"}, false},
		{"map is not simple", map[string]string{"a": "b"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsSimpleType(reflect.ValueOf(tt.value)))
		})
	}
}

func TestSimpleTypeToParam(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int", int(42), "42"},
		{"int64 negative", int64(-7), "-7"},
		{"uint", uint(7), "7"},
		{"string", "hello", "hello"},
		{"stringer", 2 * time.Second, "2s"}, // uses String(), not the int64 value
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SimpleTypeToParam(reflect.ValueOf(tt.value)))
		})
	}
}

type changedOptions struct {
	Set   *string
	Unset *string
}

func TestChanged(t *testing.T) {
	val := "x"
	o := &changedOptions{Set: &val}
	assert.True(t, Changed(o, "Set"), "field with a value should be changed")
	assert.False(t, Changed(o, "Unset"), "nil field should not be changed")
}

type toParamsOptions struct {
	Name    *string
	Count   *int
	Enabled *bool
	Tags    []string
	Labels  map[string]string
	Renamed *string `schema:"custom_name"`
	Skipped *string `schema:"-"`
}

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }
func boolp(b bool) *bool    { return &b }

func TestToParamsNil(t *testing.T) {
	// Both an untyped nil and a typed nil pointer must yield empty params.
	params, err := ToParams(nil)
	require.NoError(t, err)
	assert.Empty(t, params)

	params, err = ToParams((*toParamsOptions)(nil))
	require.NoError(t, err)
	assert.Empty(t, params)
}

func TestToParamsUnsetFieldsAreSkipped(t *testing.T) {
	params, err := ToParams(&toParamsOptions{})
	require.NoError(t, err)
	assert.Empty(t, params)
}

func TestToParamsSimpleFields(t *testing.T) {
	params, err := ToParams(&toParamsOptions{
		Name:    strp("foo"),
		Count:   intp(5),
		Enabled: boolp(true),
	})
	require.NoError(t, err)
	assert.Equal(t, "foo", params.Get("name"))
	assert.Equal(t, "5", params.Get("count"))
	assert.Equal(t, "true", params.Get("enabled"))
}

func TestToParamsSlice(t *testing.T) {
	params, err := ToParams(&toParamsOptions{Tags: []string{"a", "b"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, params["tags"])
}

func TestToParamsMap(t *testing.T) {
	params, err := ToParams(&toParamsOptions{Labels: map[string]string{"k": "v"}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"k":"v"}`, params.Get("labels"))
}

func TestToParamsSchemaTag(t *testing.T) {
	params, err := ToParams(&toParamsOptions{
		Renamed: strp("here"),
		Skipped: strp("gone"),
	})
	require.NoError(t, err)
	// "schema" tag renames the param...
	assert.Equal(t, "here", params.Get("custom_name"))
	assert.Empty(t, params.Get("renamed"))
	// ...and "-" omits the field entirely.
	assert.Empty(t, params.Get("skipped"))
}

func TestToParamsSliceOfNonSimpleTypeErrors(t *testing.T) {
	type badOptions struct {
		Items [][]string
	}
	_, err := ToParams(&badOptions{Items: [][]string{{"a"}}})
	assert.Error(t, err)
}
