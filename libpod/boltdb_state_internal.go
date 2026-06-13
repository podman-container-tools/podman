//go:build !remote && (linux || freebsd)

package libpod

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/plugin"
)

const (
	ctrName        = "ctr"
	allCtrsName    = "all-ctrs"
	podName        = "pod"
	allPodsName    = "allPods"
	volName        = "vol"
	allVolsName    = "allVolumes"
	execName       = "exec"
	aliasesName    = "aliases"
	volumeCtrsName = "volume-ctrs"

	configName   = "config"
	stateName    = "state"
	netNSName    = "netns"
	networksName = "networks"
)

var (
	ctrBkt      = []byte(ctrName)
	allCtrsBkt  = []byte(allCtrsName)
	podBkt      = []byte(podName)
	allPodsBkt  = []byte(allPodsName)
	volBkt      = []byte(volName)
	allVolsBkt  = []byte(allVolsName)
	aliasesBkt  = []byte(aliasesName)
	networksBkt = []byte(networksName)

	configKey = []byte(configName)
	stateKey  = []byte(stateName)
	netNSKey  = []byte(netNSName)
)

// Open a connection to the database.
// Must be paired with a `defer closeDBCon()` on the returned database, to
// ensure the state is properly unlocked
func (s *BoltState) getDBCon() (*bolt.DB, error) {
	// We need an in-memory lock to avoid issues around POSIX file advisory
	// locks as described in the link below:
	// https://www.sqlite.org/src/artifact/c230a7a24?ln=994-1081
	s.dbLock.Lock()

	db, err := bolt.Open(s.dbPath, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", s.dbPath, err)
	}

	return db, nil
}

// deferredCloseDBCon closes the bolt db but instead of returning an
// error it logs the error. it is meant to be used within the confines
// of a defer statement only
func (s *BoltState) deferredCloseDBCon(db *bolt.DB) {
	if err := s.closeDBCon(db); err != nil {
		logrus.Errorf("Failed to close libpod db: %q", err)
	}
}

// Close a connection to the database.
// MUST be used in place of `db.Close()` to ensure proper unlocking of the
// state.
func (s *BoltState) closeDBCon(db *bolt.DB) error {
	err := db.Close()

	s.dbLock.Unlock()

	return err
}

func getCtrBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(ctrBkt)
	if bkt == nil {
		return nil, fmt.Errorf("containers bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getAllCtrsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(allCtrsBkt)
	if bkt == nil {
		return nil, fmt.Errorf("all containers bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getPodBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(podBkt)
	if bkt == nil {
		return nil, fmt.Errorf("pods bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getAllPodsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(allPodsBkt)
	if bkt == nil {
		return nil, fmt.Errorf("all pods bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getVolBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(volBkt)
	if bkt == nil {
		return nil, fmt.Errorf("volumes bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func getAllVolsBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	bkt := tx.Bucket(allVolsBkt)
	if bkt == nil {
		return nil, fmt.Errorf("all volumes bucket not found in DB: %w", define.ErrDBBadConfig)
	}
	return bkt, nil
}

func (s *BoltState) getContainerConfigFromDB(id []byte, config *ContainerConfig, ctrsBkt *bolt.Bucket) error {
	ctrBkt := ctrsBkt.Bucket(id)
	if ctrBkt == nil {
		return fmt.Errorf("container %s not found in DB: %w", string(id), define.ErrNoSuchCtr)
	}

	configBytes := ctrBkt.Get(configKey)
	if configBytes == nil {
		return fmt.Errorf("container %s missing config key in DB: %w", string(id), define.ErrInternal)
	}

	if err := json.Unmarshal(configBytes, config); err != nil {
		return fmt.Errorf("unmarshalling container %s config: %w", string(id), err)
	}

	// convert ports to the new format if needed
	if len(config.ContainerNetworkConfig.OldPortMappings) > 0 && len(config.ContainerNetworkConfig.PortMappings) == 0 {
		config.ContainerNetworkConfig.PortMappings = ocicniPortsToNetTypesPorts(config.ContainerNetworkConfig.OldPortMappings)
		// keep the OldPortMappings in case an user has to downgrade podman

		// indicate that the config was modified and should be written back to the db when possible
		config.rewrite = true
	}

	return nil
}

func (s *BoltState) getContainerStateDB(id []byte, ctr *Container, ctrsBkt *bolt.Bucket) error {
	newState := new(ContainerState)
	ctrToUpdate := ctrsBkt.Bucket(id)
	if ctrToUpdate == nil {
		ctr.valid = false
		return fmt.Errorf("container %s does not exist in database: %w", ctr.ID(), define.ErrNoSuchCtr)
	}

	newStateBytes := ctrToUpdate.Get(stateKey)
	if newStateBytes == nil {
		return fmt.Errorf("container %s does not have a state key in DB: %w", ctr.ID(), define.ErrInternal)
	}

	if err := json.Unmarshal(newStateBytes, newState); err != nil {
		return fmt.Errorf("unmarshalling container %s state: %w", ctr.ID(), err)
	}

	// backwards compat, previously we used an extra bucket for the netns so try to get it from there
	netNSBytes := ctrToUpdate.Get(netNSKey)
	if netNSBytes != nil && newState.NetNS == "" {
		newState.NetNS = string(netNSBytes)
	}

	// New state compiled successfully, swap it into the current state
	ctr.state = newState
	return nil
}

func (s *BoltState) getContainerFromDB(id []byte, ctr *Container, ctrsBkt *bolt.Bucket, loadState bool) error {
	if err := s.getContainerConfigFromDB(id, ctr.config, ctrsBkt); err != nil {
		return err
	}

	if loadState {
		if err := s.getContainerStateDB(id, ctr, ctrsBkt); err != nil {
			return err
		}
	}

	// Get the lock
	lock, err := s.runtime.lockManager.RetrieveLock(ctr.config.LockID)
	if err != nil {
		return fmt.Errorf("retrieving lock for container %s: %w", string(id), err)
	}
	ctr.lock = lock

	if ctr.config.OCIRuntime == "" {
		ctr.ociRuntime = s.runtime.defaultOCIRuntime
	} else {
		// Handle legacy containers which might use a literal path for
		// their OCI runtime name.
		runtimeName := ctr.config.OCIRuntime
		ociRuntime, ok := s.runtime.ociRuntimes[runtimeName]
		if !ok {
			runtimeSet := false

			// If the path starts with a / and exists, make a new
			// OCI runtime for it using the full path.
			if strings.HasPrefix(runtimeName, "/") {
				if stat, err := os.Stat(runtimeName); err == nil && !stat.IsDir() {
					newOCIRuntime, err := newConmonOCIRuntime(runtimeName, []string{runtimeName}, s.runtime.conmonPath, s.runtime.runtimeFlags, s.runtime.config)
					if err == nil {
						// The runtime lock should
						// protect against concurrent
						// modification of the map.
						ociRuntime = newOCIRuntime
						s.runtime.ociRuntimes[runtimeName] = ociRuntime
						runtimeSet = true
					}
				}
			}

			if !runtimeSet {
				// Use a MissingRuntime implementation
				ociRuntime = getMissingRuntime(runtimeName, s.runtime)
			}
		}
		ctr.ociRuntime = ociRuntime
	}

	ctr.runtime = s.runtime
	ctr.valid = true

	return nil
}

func (s *BoltState) getPodFromDB(id []byte, pod *Pod, podBkt *bolt.Bucket) error {
	podDB := podBkt.Bucket(id)
	if podDB == nil {
		return fmt.Errorf("pod with ID %s not found: %w", string(id), define.ErrNoSuchPod)
	}

	podConfigBytes := podDB.Get(configKey)
	if podConfigBytes == nil {
		return fmt.Errorf("pod %s is missing configuration key in DB: %w", string(id), define.ErrInternal)
	}

	if err := json.Unmarshal(podConfigBytes, pod.config); err != nil {
		return fmt.Errorf("unmarshalling pod %s config from DB: %w", string(id), err)
	}

	// Get the lock
	lock, err := s.runtime.lockManager.RetrieveLock(pod.config.LockID)
	if err != nil {
		return fmt.Errorf("retrieving lock for pod %s: %w", string(id), err)
	}
	pod.lock = lock

	pod.runtime = s.runtime
	pod.valid = true

	return nil
}

func (s *BoltState) getVolumeFromDB(name []byte, volume *Volume, volBkt *bolt.Bucket) error {
	volDB := volBkt.Bucket(name)
	if volDB == nil {
		return fmt.Errorf("volume with name %s not found: %w", string(name), define.ErrNoSuchVolume)
	}

	volConfigBytes := volDB.Get(configKey)
	if volConfigBytes == nil {
		return fmt.Errorf("volume %s is missing configuration key in DB: %w", string(name), define.ErrInternal)
	}

	if err := json.Unmarshal(volConfigBytes, volume.config); err != nil {
		return fmt.Errorf("unmarshalling volume %s config from DB: %w", string(name), err)
	}

	// Volume state is allowed to be nil for legacy compatibility
	volStateBytes := volDB.Get(stateKey)
	if volStateBytes != nil {
		if err := json.Unmarshal(volStateBytes, volume.state); err != nil {
			return fmt.Errorf("unmarshalling volume %s state from DB: %w", string(name), err)
		}
	}

	// Need this for UsesVolumeDriver() so set it now.
	volume.runtime = s.runtime
	volume.volumePlugin = sync.OnceValues(func() (*plugin.VolumePlugin, error) {
		if !volume.UsesVolumeDriver() {
			return nil, nil
		}
		return volume.runtime.getVolumePlugin(volume.config)
	})

	// Get the lock
	lock, err := s.runtime.lockManager.RetrieveLock(volume.config.LockID)
	if err != nil {
		return fmt.Errorf("retrieving lock for volume %q: %w", string(name), err)
	}
	volume.lock = lock

	volume.valid = true

	return nil
}

// ocicniPortsToNetTypesPorts convert the old port format to the new one
// while deduplicating ports into ranges
//
//nolint:staticcheck
func ocicniPortsToNetTypesPorts(ports []types.OCICNIPortMapping) []types.PortMapping {
	if len(ports) == 0 {
		return nil
	}

	newPorts := make([]types.PortMapping, 0, len(ports))

	// first sort the ports
	sort.Slice(ports, func(i, j int) bool {
		return compareOCICNIPorts(ports[i], ports[j])
	})

	// we already check if the slice is empty so we can use the first element
	currentPort := types.PortMapping{
		HostIP:        ports[0].HostIP,
		HostPort:      uint16(ports[0].HostPort),
		ContainerPort: uint16(ports[0].ContainerPort),
		Protocol:      ports[0].Protocol,
		Range:         1,
	}

	for i := 1; i < len(ports); i++ {
		if ports[i].HostIP == currentPort.HostIP &&
			ports[i].Protocol == currentPort.Protocol &&
			ports[i].HostPort-int32(currentPort.Range) == int32(currentPort.HostPort) &&
			ports[i].ContainerPort-int32(currentPort.Range) == int32(currentPort.ContainerPort) {
			currentPort.Range++
		} else {
			newPorts = append(newPorts, currentPort)
			currentPort = types.PortMapping{
				HostIP:        ports[i].HostIP,
				HostPort:      uint16(ports[i].HostPort),
				ContainerPort: uint16(ports[i].ContainerPort),
				Protocol:      ports[i].Protocol,
				Range:         1,
			}
		}
	}
	newPorts = append(newPorts, currentPort)
	return newPorts
}

// compareOCICNIPorts will sort the ocicni ports by
// 1) host ip
// 2) protocol
// 3) hostPort
// 4) container port
//
//nolint:staticcheck
func compareOCICNIPorts(i, j types.OCICNIPortMapping) bool {
	if i.HostIP != j.HostIP {
		return i.HostIP < j.HostIP
	}

	if i.Protocol != j.Protocol {
		return i.Protocol < j.Protocol
	}

	if i.HostPort != j.HostPort {
		return i.HostPort < j.HostPort
	}

	return i.ContainerPort < j.ContainerPort
}
