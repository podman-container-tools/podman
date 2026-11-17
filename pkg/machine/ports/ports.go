package ports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/storage/pkg/ioutils"
	"go.podman.io/storage/pkg/lockfile"
)

const (
	portAllocFileName = "port-alloc.dat"
	portLockFileName  = "port-alloc.lck"
)

// Reserves a unique port for a machine instance in a global (user) scope across
// all machines and backend types. On success the port is guaranteed to not be
// allocated until released with a call to ReleaseMachinePort().
//
// The purpose of this method is to prevent collisions between machine
// instances when ran at the same time. Note, that dynamic port reassignment
// on its own is insufficient to resolve conflicts, since there is a narrow
// window between port detection and actual service binding, allowing for the
// possibility of a second racing machine to fail if its check is unlucky to
// fall within that window. Additionally, there is the potential for a long
// running reassignment dance over start/stop until all machine instances
// eventually arrive at total conflict free state. By reserving ports using
// mechanism these scenarios are prevented.
func AllocateMachinePort() (int, error) {
	const maxRetries = 10000

	handles := []io.Closer{}
	defer func() {
		for _, handle := range handles {
			handle.Close()
		}
	}()

	lock, err := acquirePortLock()
	if err != nil {
		return 0, err
	}
	defer lock.Unlock()

	ports, err := loadPortAllocations()
	if err != nil {
		return 0, err
	}

	var port int
	for i := 0; ; i++ {
		var handle io.Closer

		// Ports must be held temporarily to prevent repeat search results
		handle, port, err = getRandomPortHold()
		if err != nil {
			return 0, err
		}
		handles = append(handles, handle)

		if _, exists := ports[port]; !exists {
			break
		}

		if i > maxRetries {
			return 0, errors.New("maximum number of retries exceeded searching for available port")
		}
	}

	ports[port] = struct{}{}
	if err := storePortAllocations(ports); err != nil {
		return 0, err
	}

	return port, nil
}

// Releases a reserved port for a machine when no longer required. Care should
// be taken to ensure there are no conditions (e.g. failure paths) where the
// port might unintentionally remain in use after releasing
func ReleaseMachinePort(port int) error {
	lock, err := acquirePortLock()
	if err != nil {
		return err
	}
	defer lock.Unlock()
	ports, err := loadPortAllocations()
	if err != nil {
		return err
	}

	delete(ports, port)
	return storePortAllocations(ports)
}

func IsLocalPortAvailable(port int) bool {
	// Used to mark invalid / unassigned port
	if port <= 0 {
		return false
	}

	lc := getPortCheckListenConfig()
	l, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

func getRandomPortHold() (io.Closer, int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("unable to get free machine port: %w", err)
	}
	_, portString, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		l.Close()
		return nil, 0, fmt.Errorf("unable to determine free machine port: %w", err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		l.Close()
		return nil, 0, fmt.Errorf("unable to convert port to int: %w", err)
	}
	return l, port, err
}

func acquirePortLock() (*lockfile.LockFile, error) {
	lockDir, err := env.GetGlobalDataDir()
	if err != nil {
		return nil, err
	}

	lock, err := lockfile.GetLockFile(filepath.Join(lockDir, portLockFileName))
	if err != nil {
		return nil, err
	}

	lock.Lock()
	return lock, nil
}

func loadPortAllocations() (map[int]struct{}, error) {
	portDir, err := env.GetGlobalDataDir()
	if err != nil {
		return nil, err
	}

	var portData []int
	exists := true
	file, err := os.OpenFile(filepath.Join(portDir, portAllocFileName), 0, 0)
	if errors.Is(err, os.ErrNotExist) {
		exists = false
	} else if err != nil {
		return nil, err
	}
	defer file.Close()

	// Non-existence of the file, or a corrupt file are not treated as hard
	// failures, since dynamic reassignment and continued use will eventually
	// rebuild the dataset. This also makes migration cases simpler, since
	// the state doesn't have to exist
	if exists {
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&portData); err != nil {
			logrus.Warnf("corrupt port allocation file, could not use state")
		}
	}

	ports := make(map[int]struct{})
	placeholder := struct{}{}
	for _, port := range portData {
		ports[port] = placeholder
	}

	return ports, nil
}

func storePortAllocations(ports map[int]struct{}) error {
	portDir, err := env.GetGlobalDataDir()
	if err != nil {
		return err
	}

	portData := make([]int, 0, len(ports))
	for port := range ports {
		portData = append(portData, port)
	}

	opts := &ioutils.AtomicFileWriterOptions{ExplicitCommit: true}
	w, err := ioutils.NewAtomicFileWriterWithOpts(filepath.Join(portDir, portAllocFileName), 0o644, opts)
	if err != nil {
		return err
	}
	defer w.Close()

	enc := json.NewEncoder(w)
	if err := enc.Encode(portData); err != nil {
		return err
	}

	// Commit the changes to disk if no errors
	return w.Commit()
}
