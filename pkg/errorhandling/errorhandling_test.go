package errorhandling

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCause(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		err         func() error
		expectedErr error
	}{
		{
			name:        "nil error",
			err:         func() error { return nil },
			expectedErr: nil,
		},
		{
			name:        "equal errors",
			err:         func() error { return errors.New("foo") },
			expectedErr: errors.New("foo"),
		},
		{
			name:        "wrapped error",
			err:         func() error { return fmt.Errorf("baz: %w", fmt.Errorf("bar: %w", errors.New("foo"))) },
			expectedErr: errors.New("foo"),
		},
		{
			name: "max depth reached",
			err: func() error {
				err := errors.New("error")
				for i := 0; i <= 101; i++ {
					err = fmt.Errorf("%d: %w", i, err)
				}
				return err
			},
			expectedErr: fmt.Errorf("0: %w", errors.New("error")),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := Cause(tc.err())
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}
