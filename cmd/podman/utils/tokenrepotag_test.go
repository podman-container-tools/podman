package utils

import "testing"

func TestTokenRepoTag(t *testing.T) {
	for _, tc := range []struct{ in, repo, tag string }{
		{"quay.io/foo/bar:latest", "quay.io/foo/bar", "latest"},
		{"localhost:5000/foo:v1", "localhost:5000/foo", "v1"},
		{"localhost:5000/foo", "localhost:5000/foo", "<none>"},
		{"localhost/foo@sha256:" + "0123456789012345678901234567890123456789012345678901234567890123", "localhost/foo", "<none>"},
		{"<none>:<none>", "<none>", "<none>"},
	} {
		repo, tag, err := TokenRepoTag(tc.in)
		if err != nil || repo != tc.repo || tag != tc.tag {
			t.Errorf("%q: got (%q, %q, %v), want (%q, %q)", tc.in, repo, tag, err, tc.repo, tc.tag)
		}
	}
}
