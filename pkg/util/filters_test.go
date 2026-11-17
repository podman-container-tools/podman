package util

import (
	"net/url"
	"testing"

	"go.podman.io/common/pkg/filters"
)

func TestMatchLabelFilters(t *testing.T) {
	testLabels := map[string]string{
		"label1": "",
		"label2": "test",
		"label3": "",
	}
	type args struct {
		filterValues []string
		labels       map[string]string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Match when all filters the same as labels",
			args: args{
				filterValues: []string{"label1", "label3", "label2=test"},
				labels:       testLabels,
			},
			want: true,
		},
		{
			name: "Match when filter value not provided in args",
			args: args{
				filterValues: []string{"label2"},
				labels:       testLabels,
			},
			want: true,
		},
		{
			name: "Match when no filter value is given",
			args: args{
				filterValues: []string{"label2="},
				labels:       testLabels,
			},
			want: true,
		},
		{
			name: "Do not match when filter value differs",
			args: args{
				filterValues: []string{"label2=differs"},
				labels:       testLabels,
			},
			want: false,
		},
		{
			name: "Do not match when filter value not listed in labels",
			args: args{
				filterValues: []string{"label1=xyz"},
				labels:       testLabels,
			},
			want: false,
		},
		{
			name: "Do not match when one from many not ok",
			args: args{
				filterValues: []string{"label1=xyz", "invalid=valid"},
				labels:       testLabels,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filters.MatchLabelFilters(tt.args.filterValues, tt.args.labels); got != tt.want {
				t.Errorf("MatchLabelFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeVolumePruneFilters(t *testing.T) {
	t.Parallel()
	t.Run("empty means anonymous true", func(t *testing.T) {
		t.Parallel()
		got := NormalizeVolumePruneFilters(url.Values{})
		if got.Get("anonymous") != "true" {
			t.Fatalf("expected anonymous=true, got %q", got.Get("anonymous"))
		}
	})
	t.Run("all true drops all and no anonymous", func(t *testing.T) {
		t.Parallel()
		got := NormalizeVolumePruneFilters(url.Values{"all": {"true"}})
		if got.Has("all") || got.Has("anonymous") {
			t.Fatalf("got %#v, want no all/anonymous keys", got)
		}
	})
	t.Run("all true keeps label filters", func(t *testing.T) {
		t.Parallel()
		got := NormalizeVolumePruneFilters(url.Values{"all": {"true"}, "label": {"k=v"}, "label!": {"x"}})
		if got.Has("all") || got.Has("anonymous") {
			t.Fatalf("got %#v, want no all/anonymous keys", got)
		}
		if got.Get("label") != "k=v" {
			t.Fatalf("label dropped alongside all: got %#v", got)
		}
		if got.Get("label!") != "x" {
			t.Fatalf("label! dropped alongside all: got %#v", got)
		}
	})
	t.Run("label without anonymous key does not inject anonymous", func(t *testing.T) {
		t.Parallel()
		in := url.Values{"label": {"k=v"}}
		got := NormalizeVolumePruneFilters(in)
		if got.Has("anonymous") {
			t.Fatal("must not inject anonymous alongside label (OR would ignore label for anonymous volumes)")
		}
		if got.Get("label") != "k=v" {
			t.Fatalf("label got %q", got.Get("label"))
		}
	})
	t.Run("explicit anonymous skips injection branch", func(t *testing.T) {
		t.Parallel()
		got := NormalizeVolumePruneFilters(url.Values{"anonymous": {"false"}})
		if got.Get("anonymous") != "false" {
			t.Fatalf("got %q", got.Get("anonymous"))
		}
	})
	t.Run("until does not inject anonymous", func(t *testing.T) {
		t.Parallel()
		got := NormalizeVolumePruneFilters(url.Values{"until": {"1h"}})
		if got.Has("anonymous") {
			t.Fatal("must not inject anonymous alongside until")
		}
	})
}
