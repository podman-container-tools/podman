package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlice(t *testing.T) {
	goodMap := make(map[string]string, 0)
	goodMap["apple"] = "red"
	goodMap["banana"] = "yellow"
	goodMap["pear"] = ""
	goodResult := []string{"apple=red", "banana=yellow", "pear"}
	type args struct {
		m map[string]string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Good",
			args: args{
				m: goodMap,
			},
			want: goodResult,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatchf(t, Slice(tt.args.m), tt.want, "Slice() = %v, want %v", Slice(tt.args.m), tt.want)
		})
	}
}

func TestJoin(t *testing.T) {
	firstMap := make(map[string]string, 0)
	firstMap["apple"] = "red"
	secondMap := make(map[string]string, 0)
	secondMap["banana"] = "yellow"
	goodResult := make(map[string]string, 0)
	goodResult["apple"] = "red"
	goodResult["banana"] = "yellow"
	overrideResult := make(map[string]string, 0)
	overrideResult["apple"] = "green"
	overrideResult["banana"] = "yellow"
	overrideMap := make(map[string]string, 0)
	overrideMap["banana"] = "yellow"
	overrideMap["apple"] = "green"
	type args struct {
		base     map[string]string
		override map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "GoodJoin",
			args: args{
				base:     firstMap,
				override: secondMap,
			},
			want: goodResult,
		},
		{
			name: "GoodOverride",
			args: args{
				base:     firstMap,
				override: overrideMap,
			},
			want: overrideResult,
		},
		{
			name: "EmptyOverride",
			args: args{
				base:     firstMap,
				override: nil,
			},
			want: firstMap,
		},
		{
			name: "EmptyBase",
			args: args{
				base:     nil,
				override: firstMap,
			},
			want: firstMap,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Join(tt.args.base, tt.args.override)
			assert.EqualValuesf(t, got, tt.want, "Join() = %v, want %v", got, tt.want)
		})
	}
}

func Test_parseEnv(t *testing.T) {
	good := make(map[string]string)

	type args struct {
		env  map[string]string
		line string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Good",
			args: args{
				env:  good,
				line: "apple=red",
			},
			wantErr: false,
		},
		{
			name: "GoodNoValue",
			args: args{
				env:  good,
				line: "apple=",
			},
			wantErr: false,
		},
		{
			name: "GoodNoKeyNoValue",
			args: args{
				env:  good,
				line: "=",
			},
			wantErr: true,
		},
		{
			name: "BadNoKey",
			args: args{
				env:  good,
				line: "=foobar",
			},
			wantErr: true,
		},
		{
			name: "BadOnlyDelim",
			args: args{
				env:  good,
				line: "=",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parseEnv(tt.args.env, tt.args.line); (err != nil) != tt.wantErr {
				t.Errorf("parseEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMap(t *testing.T) {
	type args struct {
		slice []string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "test parse env to map",
			args: args{
				slice: []string{"apple=red", "banana=yellow", "pear"},
			},
			want: map[string]string{"apple": "red", "banana": "yellow", "pear": ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, Map(tt.args.slice), "Map(%v)", tt.args.slice)
		})
	}
}
