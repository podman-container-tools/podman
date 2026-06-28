package util_test

import (
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/bindings/internal/util"
)

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }
func boolp(b bool) *bool    { return &b }

type changedOptions struct {
	Set   *string
	Unset *string
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
			assert.Equal(t, tt.want, util.IsSimpleType(reflect.ValueOf(tt.value)))
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
			assert.Equal(t, tt.want, util.SimpleTypeToParam(reflect.ValueOf(tt.value)))
		})
	}
}

func TestSimpleTypeToParamPanicsOnNonSimpleType(t *testing.T) {
	assert.Panics(t, func() {
		util.SimpleTypeToParam(reflect.ValueOf(1.5))
	})
}

func TestChanged(t *testing.T) {
	val := "x"
	o := &changedOptions{Set: &val}
	assert.True(t, util.Changed(o, "Set"), "field with a value should be changed")
	assert.False(t, util.Changed(o, "Unset"), "nil field should not be changed")
}

func TestToParamsNil(t *testing.T) {
	// Both an untyped nil and a typed nil pointer must yield empty params.
	params, err := util.ToParams(nil)
	require.NoError(t, err)
	assert.Empty(t, params)

	params, err = util.ToParams((*toParamsOptions)(nil))
	require.NoError(t, err)
	assert.Empty(t, params)
}

func TestToParamsUnsetFieldsAreSkipped(t *testing.T) {
	params, err := util.ToParams(&toParamsOptions{})
	require.NoError(t, err)
	assert.Empty(t, params)
}

func TestToParamsSimpleFields(t *testing.T) {
	params, err := util.ToParams(&toParamsOptions{
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
	params, err := util.ToParams(&toParamsOptions{Tags: []string{"a", "b"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, params["tags"])
}

func TestToParamsEmptySlice(t *testing.T) {
	// A non-nil but empty slice is "changed" yet contributes no values, so the
	// param must not be added at all.
	params, err := util.ToParams(&toParamsOptions{Tags: []string{}})
	require.NoError(t, err)
	assert.NotContains(t, params, "tags", "empty slice should not add a param")
}

func TestToParamsMap(t *testing.T) {
	params, err := util.ToParams(&toParamsOptions{Labels: map[string]string{"k": "v"}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"k":"v"}`, params.Get("labels"))
}

func TestToParamsEmptyMap(t *testing.T) {
	// A non-nil but empty map is serialized to an empty JSON object.
	params, err := util.ToParams(&toParamsOptions{Labels: map[string]string{}})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, params.Get("labels"))
}

func TestToParamsSchemaTag(t *testing.T) {
	params, err := util.ToParams(&toParamsOptions{
		Renamed: strp("here"),
		Skipped: strp("gone"),
	})
	require.NoError(t, err)
	// "custom_name" (the schema rename) must be the only key: the field name
	// "renamed" is not used, and the schema:"-" field is omitted entirely.
	assert.Equal(t, url.Values{"custom_name": {"here"}}, params)
}

func TestToParamsSliceOfNonSimpleTypeErrors(t *testing.T) {
	type badOptions struct {
		Items [][]string
	}
	_, err := util.ToParams(&badOptions{Items: [][]string{{"a"}}})
	assert.Error(t, err)
}
