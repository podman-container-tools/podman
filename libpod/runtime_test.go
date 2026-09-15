//go:build !remote && (linux || freebsd)

package libpod

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/storage"
)

func Test_generateName(t *testing.T) {
	state, _ := getEmptySqliteState(t)

	r := &Runtime{
		state: state,
	}

	// Test that (*Runtime).generateName returns different names
	// if called twice.
	n1, _ := r.generateName()
	n2, _ := r.generateName()
	assert.NotEqual(t, n1, n2)
}

// shutdownRecorderStore records the Shutdown calls made against it.  Only
// Shutdown is implemented, any other call panics on the embedded nil
// storage.Store, which keeps accidental use of this type visible.
type shutdownRecorderStore struct {
	storage.Store
	forced []bool
}

func (s *shutdownRecorderStore) Shutdown(force bool) ([]string, error) {
	s.forced = append(s.forced, force)
	return nil, nil
}

func TestRuntimeShutdownStoreOnError(t *testing.T) {
	for _, tc := range []struct {
		name         string
		retErr       error
		withStore    bool
		wantShutdown []bool
	}{
		{
			// The regression: makeRuntime used to look at a local
			// variable that configureStore() never assigned, so the
			// store of a partially-created runtime was leaked.
			name:         "failed runtime shuts the store down non-forcibly",
			retErr:       errors.New("some failure after the store was configured"),
			withStore:    true,
			wantShutdown: []bool{false},
		},
		{
			name:      "successful runtime keeps the store open",
			retErr:    nil,
			withStore: true,
		},
		{
			name:      "failed runtime without a store does nothing",
			retErr:    errors.New("some failure before the store was configured"),
			withStore: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := new(Runtime)
			store := new(shutdownRecorderStore)
			if tc.withStore {
				r.store = store
			}

			r.shutdownStoreOnError(tc.retErr)

			assert.Equal(t, tc.wantShutdown, store.forced)
		})
	}
}
