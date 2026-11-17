//go:build !remote && (linux || freebsd)

package libpod

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/storage/pkg/fileutils"
)

// BoltState is a state implementation backed by a Bolt DB
type BoltState struct {
	valid   bool
	dbPath  string
	dbLock  sync.Mutex
	runtime *Runtime
}

// A brief description of the format of the BoltDB state:
// At the top level, the following buckets are created:
// - idRegistryBkt: Maps ID to Name for containers and pods.
//   Used to ensure container and pod IDs are globally unique.
// - nameRegistryBkt: Maps Name to ID for containers and pods.
//   Used to ensure container and pod names are globally unique.
// - ctrBkt: Contains a sub-bucket for each container in the state.
//   Each sub-bucket has config and state keys holding the container's JSON
//   encoded configuration and state (respectively), an optional netNS key
//   containing the path to the container's network namespace, a dependencies
//   bucket containing the container's dependencies, and an optional pod key
//   containing the ID of the pod the container is joined to.
//   After updates to include exec sessions, may also include an exec bucket
//   with the IDs of exec sessions currently in use by the container.
// - allCtrsBkt: Map of ID to name containing only containers. Used for
//   container lookup operations.
// - podBkt: Contains a sub-bucket for each pod in the state.
//   Each sub-bucket has config and state keys holding the pod's JSON encoded
//   configuration and state, plus a containers sub bucket holding the IDs of
//   containers in the pod.
// - allPodsBkt: Map of ID to name containing only pods. Used for pod lookup
//   operations.
// - execBkt: Map of exec session ID to container ID - used for resolving
//   exec session IDs to the containers that hold the exec session.
// - networksBkt: Contains all network names as key with their options json
//   encoded as value.
// - aliasesBkt - Deprecated, use the networksBkt. Used to contain a bucket
//   for each CNI network which contain a map of network alias (an extra name
//   for containers in DNS) to the ID of the container holding the alias.
//   Aliases must be unique per-network, and cannot conflict with names
//   registered in nameRegistryBkt.
// - runtimeConfigBkt: Contains configuration of the libpod instance that
//   initially created the database. This must match for any further instances
//   that access the database, to ensure that state mismatches with
//   containers/storage do not occur.
// - exitCodeBucket/exitCodeTimeStampBucket: (#14559) exit codes must be part
//   of the database to resolve a previous race condition when one process waits
//   for the exit file to be written and another process removes it along with
//   the container during auto-removal.  The same race would happen trying to
//   read the exit code from the containers bucket.  Hence, exit codes go into
//   their own bucket.  To avoid the rather expensive JSON (un)marshalling, we
//   have two buckets: one for the exit codes, the other for the timestamps.

// NewBoltState creates a new bolt-backed state database
func NewBoltState(path string, runtime *Runtime) (*BoltState, error) {
	logrus.Info("Using boltdb as database backend")
	state := new(BoltState)
	state.dbPath = path
	state.runtime = runtime

	logrus.Debugf("Opening legacy boltdb state at %s", path)

	if err := fileutils.Exists(path); err != nil && errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("boltdb database %s does not exist", path)
	}

	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", path, err)
	}
	// Everywhere else, we use s.deferredCloseDBCon(db) to ensure the state's DB
	// mutex is also unlocked.
	// However, here, the mutex has not been locked, since we just created
	// the DB connection, and it hasn't left this function yet - no risk of
	// concurrent access.
	// As such, just a db.Close() is fine here.
	defer db.Close()

	state.valid = true

	return state, nil
}

// Close closes the state and prevents further use
func (s *BoltState) Close() error {
	s.valid = false
	return nil
}

// UpdateContainer updates a container's state from the database
func (s *BoltState) UpdateContainer(ctr *Container) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !ctr.valid {
		return define.ErrCtrRemoved
	}

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	return db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}
		return s.getContainerStateDB(ctrID, ctr, ctrBucket)
	})
}

// AllContainers retrieves all the containers in the database
// If `loadState` is set, the containers' state will be loaded as well.
func (s *BoltState) AllContainers(loadState bool) ([]*Container, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	ctrs := []*Container{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		allCtrsBucket, err := getAllCtrsBucket(tx)
		if err != nil {
			return err
		}

		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		return allCtrsBucket.ForEach(func(id, _ []byte) error {
			// If performance becomes an issue, this check can be
			// removed. But the error messages that come back will
			// be much less helpful.
			ctrExists := ctrBucket.Bucket(id)
			if ctrExists == nil {
				return fmt.Errorf("state is inconsistent - container ID %s in all containers, but container not found: %w", string(id), define.ErrInternal)
			}

			ctr := new(Container)
			ctr.config = new(ContainerConfig)
			ctr.state = new(ContainerState)

			if err := s.getContainerFromDB(id, ctr, ctrBucket, loadState); err != nil {
				logrus.Errorf("Error retrieving container from database: %v", err)
			} else {
				ctrs = append(ctrs, ctr)
			}

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return ctrs, nil
}

// GetNetworks returns the networks this container is a part of.
func (s *BoltState) GetNetworks(ctr *Container) (map[string]types.PerNetworkOptions, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	if !ctr.valid {
		return nil, define.ErrCtrRemoved
	}

	// if the network mode is not bridge return no networks
	if !ctr.config.NetMode.IsBridge() {
		return nil, nil
	}

	ctrID := []byte(ctr.ID())

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	networks := make(map[string]types.PerNetworkOptions)

	var convertDB bool

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		dbCtr := ctrBucket.Bucket(ctrID)
		if dbCtr == nil {
			ctr.valid = false
			return fmt.Errorf("container %s does not exist in database: %w", ctr.ID(), define.ErrNoSuchCtr)
		}

		ctrNetworkBkt := dbCtr.Bucket(networksBkt)
		if ctrNetworkBkt == nil {
			// convert if needed
			convertDB = true
			return nil
		}

		return ctrNetworkBkt.ForEach(func(network, v []byte) error {
			opts := types.PerNetworkOptions{}
			if err := json.Unmarshal(v, &opts); err != nil {
				// special case for backwards compat
				// earlier version used the container id as value so we set a
				// special error to indicate the we have to migrate the db
				if !bytes.Equal(v, ctrID) {
					return err
				}
				convertDB = true
			}
			networks[string(network)] = opts
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if convertDB {
		err = db.Update(func(tx *bolt.Tx) error {
			ctrBucket, err := getCtrBucket(tx)
			if err != nil {
				return err
			}

			dbCtr := ctrBucket.Bucket(ctrID)
			if dbCtr == nil {
				ctr.valid = false
				return fmt.Errorf("container %s does not exist in database: %w", ctr.ID(), define.ErrNoSuchCtr)
			}

			var networkList []string

			ctrNetworkBkt := dbCtr.Bucket(networksBkt)
			if ctrNetworkBkt == nil {
				ctrNetworkBkt, err = dbCtr.CreateBucket(networksBkt)
				if err != nil {
					return fmt.Errorf("creating networks bucket for container %s: %w", ctr.ID(), err)
				}
				// the container has no networks in the db lookup config and write to the db
				networkList = ctr.config.NetworksDeprecated
				// if there are no networks we have to add the default
				if len(networkList) == 0 {
					networkList = []string{ctr.runtime.config.Network.DefaultNetwork}
				}
			} else {
				err = ctrNetworkBkt.ForEach(func(network, _ []byte) error {
					networkList = append(networkList, string(network))
					return nil
				})
				if err != nil {
					return err
				}
			}

			// the container has no networks in the db lookup config and write to the db
			for i, network := range networkList {
				var intName string
				if ctr.state.NetInterfaceDescriptions != nil {
					eth, exists := ctr.state.NetInterfaceDescriptions[network]
					if !exists {
						return fmt.Errorf("no network interface name for container %s on network %s", ctr.config.ID, network)
					}
					intName = fmt.Sprintf("eth%d", eth)
				} else {
					intName = fmt.Sprintf("eth%d", i)
				}
				getAliases := func(network string) []string {
					var aliases []string
					ctrAliasesBkt := dbCtr.Bucket(aliasesBkt)
					if ctrAliasesBkt == nil {
						return nil
					}
					netAliasesBkt := ctrAliasesBkt.Bucket([]byte(network))
					if netAliasesBkt == nil {
						// No aliases for this specific network.
						return nil
					}

					// let's ignore the error here there is nothing we can do
					_ = netAliasesBkt.ForEach(func(alias, _ []byte) error {
						aliases = append(aliases, string(alias))
						return nil
					})
					// also add the short container id as alias
					return aliases
				}

				netOpts := &types.PerNetworkOptions{
					InterfaceName: intName,
					// we have to add the short id as alias for docker compat
					Aliases: append(getAliases(network), ctr.config.ID[:12]),
				}

				optsBytes, err := json.Marshal(netOpts)
				if err != nil {
					return err
				}
				// insert into network map because we need to return this
				networks[network] = *netOpts

				err = ctrNetworkBkt.Put([]byte(network), optsBytes)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return networks, nil
}

// GetContainerConfig returns a container config from the database by full ID
func (s *BoltState) GetContainerConfig(id string) (*ContainerConfig, error) {
	if len(id) == 0 {
		return nil, define.ErrEmptyID
	}

	if !s.valid {
		return nil, define.ErrDBClosed
	}

	config := new(ContainerConfig)

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		ctrBucket, err := getCtrBucket(tx)
		if err != nil {
			return err
		}

		return s.getContainerConfigFromDB([]byte(id), config, ctrBucket)
	})
	if err != nil {
		return nil, err
	}

	return config, nil
}

// UpdateVolume updates the volume's state from the database.
func (s *BoltState) UpdateVolume(volume *Volume) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !volume.valid {
		return define.ErrVolumeRemoved
	}

	newState := new(VolumeState)
	volumeName := []byte(volume.Name())

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		volBucket, err := getVolBucket(tx)
		if err != nil {
			return err
		}

		volToUpdate := volBucket.Bucket(volumeName)
		if volToUpdate == nil {
			volume.valid = false
			return fmt.Errorf("no volume with name %s found in database: %w", volume.Name(), define.ErrNoSuchVolume)
		}

		stateBytes := volToUpdate.Get(stateKey)
		if stateBytes == nil {
			// Having no state is valid.
			// Return nil, use the empty state.
			return nil
		}

		if err := json.Unmarshal(stateBytes, newState); err != nil {
			return fmt.Errorf("unmarshalling volume %s state: %w", volume.Name(), err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	volume.state = newState

	return nil
}

// AllVolumes returns all volumes present in the state
func (s *BoltState) AllVolumes() ([]*Volume, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	volumes := []*Volume{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		allVolsBucket, err := getAllVolsBucket(tx)
		if err != nil {
			return err
		}

		volBucket, err := getVolBucket(tx)
		if err != nil {
			return err
		}
		err = allVolsBucket.ForEach(func(id, _ []byte) error {
			volExists := volBucket.Bucket(id)
			// This check can be removed if performance becomes an
			// issue, but much less helpful errors will be produced
			if volExists == nil {
				return fmt.Errorf("inconsistency in state - volume %s is in all volumes bucket but volume not found: %w", string(id), define.ErrInternal)
			}

			volume := new(Volume)
			volume.config = new(VolumeConfig)
			volume.state = new(VolumeState)

			if err := s.getVolumeFromDB(id, volume, volBucket); err != nil {
				if !errors.Is(err, define.ErrNSMismatch) {
					logrus.Errorf("Retrieving volume %s from the database: %v", string(id), err)
				}
			} else {
				volumes = append(volumes, volume)
			}

			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return volumes, nil
}

// UpdatePod updates a pod's state from the database
func (s *BoltState) UpdatePod(pod *Pod) error {
	if !s.valid {
		return define.ErrDBClosed
	}

	if !pod.valid {
		return define.ErrPodRemoved
	}

	newState := new(podState)

	db, err := s.getDBCon()
	if err != nil {
		return err
	}
	defer s.deferredCloseDBCon(db)

	podID := []byte(pod.ID())

	err = db.View(func(tx *bolt.Tx) error {
		podBkt, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		podDB := podBkt.Bucket(podID)
		if podDB == nil {
			pod.valid = false
			return fmt.Errorf("no pod with ID %s found in database: %w", pod.ID(), define.ErrNoSuchPod)
		}

		// Get the pod state JSON
		podStateBytes := podDB.Get(stateKey)
		if podStateBytes == nil {
			return fmt.Errorf("pod %s is missing state key in DB: %w", pod.ID(), define.ErrInternal)
		}

		if err := json.Unmarshal(podStateBytes, newState); err != nil {
			return fmt.Errorf("unmarshalling pod %s state JSON: %w", pod.ID(), err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	pod.state = newState

	return nil
}

// AllPods returns all pods present in the state
func (s *BoltState) AllPods() ([]*Pod, error) {
	if !s.valid {
		return nil, define.ErrDBClosed
	}

	pods := []*Pod{}

	db, err := s.getDBCon()
	if err != nil {
		return nil, err
	}
	defer s.deferredCloseDBCon(db)

	err = db.View(func(tx *bolt.Tx) error {
		allPodsBucket, err := getAllPodsBucket(tx)
		if err != nil {
			return err
		}

		podBucket, err := getPodBucket(tx)
		if err != nil {
			return err
		}

		err = allPodsBucket.ForEach(func(id, _ []byte) error {
			podExists := podBucket.Bucket(id)
			// This check can be removed if performance becomes an
			// issue, but much less helpful errors will be produced
			if podExists == nil {
				return fmt.Errorf("inconsistency in state - pod %s is in all pods bucket but pod not found: %w", string(id), define.ErrInternal)
			}

			pod := new(Pod)
			pod.config = new(PodConfig)
			pod.state = new(podState)

			if err := s.getPodFromDB(id, pod, podBucket); err != nil {
				if !errors.Is(err, define.ErrNSMismatch) {
					logrus.Errorf("Retrieving pod %s from the database: %v", string(id), err)
				}
			} else {
				pods = append(pods, pod)
			}

			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return pods, nil
}
