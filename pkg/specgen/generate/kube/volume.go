//go:build !remote && (linux || freebsd)

package kube

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"

	"go.podman.io/common/pkg/parse"
	"go.podman.io/common/pkg/secrets"
	"go.podman.io/podman/v6/libpod"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/storage/pkg/fileutils"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

const (
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	kubeDirectoryPermission = 0o755
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	kubeFilePermission = 0o644
)

type KubeVolumeType int

const (
	KubeVolumeTypeBindMount KubeVolumeType = iota
	KubeVolumeTypeNamed
	KubeVolumeTypeConfigMap
	KubeVolumeTypeBlockDevice
	KubeVolumeTypeCharDevice
	KubeVolumeTypeSecret
	KubeVolumeTypeEmptyDir
	KubeVolumeTypeEmptyDirTmpfs
	KubeVolumeTypeImage
)

type KubeVolume struct {
	// Type of volume to create
	Type KubeVolumeType
	// Path for bind mount, volume name for named volume or image name for image volume
	Source string
	// Items to add to a named volume created where the key is the file name and the value is the data
	// This is only used when there are volumes in the yaml that refer to a configmap
	// Example: if configmap has data "SPECIAL_LEVEL: very" then the file name is "SPECIAL_LEVEL" and the
	// data in that file is "very".
	Items map[string][]byte
	// If the volume is optional, we can move on if it is not found
	// Only used when there are volumes in a yaml that refer to a configmap
	Optional bool
	// DefaultMode sets the permissions on files created for the volume
	// This is optional and defaults to 0644
	DefaultMode int32
	// Used for volumes of type Image. Ignored for other volumes types.
	ImagePullPolicy v1.PullPolicy
}

// Create a KubeVolume from an HostPathVolumeSource
func VolumeFromHostPath(hostPath *v1.HostPathVolumeSource, mountLabel string) (*KubeVolume, error) {
	if hostPath.Type != nil {
		switch *hostPath.Type {
		case v1.HostPathDirectoryOrCreate:
			if err := os.MkdirAll(hostPath.Path, kubeDirectoryPermission); err != nil {
				return nil, err
			}
			// Label a newly created volume
			if err := libpod.LabelVolumePath(hostPath.Path, mountLabel); err != nil {
				return nil, fmt.Errorf("giving %s a label: %w", hostPath.Path, err)
			}
		case v1.HostPathFileOrCreate:
			if err := fileutils.Exists(hostPath.Path); errors.Is(err, fs.ErrNotExist) {
				f, err := os.OpenFile(hostPath.Path, os.O_RDONLY|os.O_CREATE, kubeFilePermission)
				if err != nil {
					return nil, fmt.Errorf("creating HostPath: %w", err)
				}
				if err := f.Close(); err != nil {
					logrus.Warnf("Error in closing newly created HostPath file: %v", err)
				}
			}
			// unconditionally label a newly created volume

			if err := libpod.LabelVolumePath(hostPath.Path, mountLabel); err != nil {
				return nil, fmt.Errorf("giving %s a label: %w", hostPath.Path, err)
			}
		case v1.HostPathSocket:
			st, err := os.Stat(hostPath.Path)
			if err != nil {
				return nil, fmt.Errorf("checking HostPathSocket: %w", err)
			}
			if st.Mode()&os.ModeSocket != os.ModeSocket {
				return nil, fmt.Errorf("checking HostPathSocket: path %s is not a socket", hostPath.Path)
			}
		case v1.HostPathBlockDev:
			dev, err := os.Stat(hostPath.Path)
			if err != nil {
				return nil, fmt.Errorf("checking HostPathBlockDevice: %w", err)
			}
			if dev.Mode()&os.ModeCharDevice == os.ModeCharDevice {
				return nil, fmt.Errorf("checking HostPathDevice: path %s is not a block device", hostPath.Path)
			}
			return &KubeVolume{
				Type:   KubeVolumeTypeBlockDevice,
				Source: hostPath.Path,
			}, nil
		case v1.HostPathCharDev:
			dev, err := os.Stat(hostPath.Path)
			if err != nil {
				return nil, fmt.Errorf("checking HostPathCharDevice: %w", err)
			}
			if dev.Mode()&os.ModeCharDevice != os.ModeCharDevice {
				return nil, fmt.Errorf("checking HostPathCharDevice: path %s is not a character device", hostPath.Path)
			}
			return &KubeVolume{
				Type:   KubeVolumeTypeCharDevice,
				Source: hostPath.Path,
			}, nil
		case v1.HostPathDirectory:
		case v1.HostPathFile:
		case v1.HostPathUnset:
			// do nothing here because we will verify the path exists in validateVolumeHostDir
			break
		default:
			return nil, fmt.Errorf("invalid HostPath type %v", hostPath.Type)
		}
	}

	if err := parse.ValidateVolumeHostDir(hostPath.Path); err != nil {
		return nil, fmt.Errorf("in parsing HostPath in YAML: %w", err)
	}

	return &KubeVolume{
		Type:   KubeVolumeTypeBindMount,
		Source: hostPath.Path,
	}, nil
}

// VolumeFromSecret creates a new kube volume from a kube secret.
func VolumeFromSecret(secretSource *v1.SecretVolumeSource, secretsManager *secrets.SecretsManager) (*KubeVolume, error) {
	kv := &KubeVolume{
		Type:        KubeVolumeTypeSecret,
		Source:      secretSource.SecretName,
		Items:       map[string][]byte{},
		DefaultMode: v1.SecretVolumeSourceDefaultMode,
	}
	// Set the defaultMode if set in the kube yaml
	validMode, err := isValidDefaultMode(secretSource.DefaultMode)
	if err != nil {
		return nil, fmt.Errorf("invalid DefaultMode for secret %q: %w", secretSource.SecretName, err)
	}
	if validMode {
		kv.DefaultMode = *secretSource.DefaultMode
	}

	// returns a byte array of a kube secret data, meaning this needs to go into a string map
	_, secretByte, err := secretsManager.LookupSecretData(secretSource.SecretName)
	if err != nil {
		if errors.Is(err, secrets.ErrNoSuchSecret) && secretSource.Optional != nil && *secretSource.Optional {
			kv.Optional = true
			return kv, nil
		}
		return nil, err
	}

	secret := &v1.Secret{}

	if err := yaml.Unmarshal(secretByte, secret); err != nil {
		return nil, fmt.Errorf("only secrets created via the kube yaml file are supported: %w", err)
	}

	// If there are Items specified in the volumeSource, that overwrites the Data from the Secret
	if len(secretSource.Items) > 0 {
		for _, item := range secretSource.Items {
			if val, ok := secret.Data[item.Key]; ok {
				kv.Items[item.Path] = val
			} else if val, ok := secret.StringData[item.Key]; ok {
				kv.Items[item.Path] = []byte(val)
			}
		}
	} else {
		// add key: value pairs to the items array
		maps.Copy(kv.Items, secret.Data)

		for key, entry := range secret.StringData {
			kv.Items[key] = []byte(entry)
		}
	}

	return kv, nil
}

// Create a KubeVolume from a PersistentVolumeClaimVolumeSource
func VolumeFromPersistentVolumeClaim(claim *v1.PersistentVolumeClaimVolumeSource) (*KubeVolume, error) {
	return &KubeVolume{
		Type:   KubeVolumeTypeNamed,
		Source: claim.ClaimName,
	}, nil
}

func VolumeFromConfigMap(configMapVolumeSource *v1.ConfigMapVolumeSource, configMaps []v1.ConfigMap) (*KubeVolume, error) {
	var configMap *v1.ConfigMap
	kv := &KubeVolume{
		Type:        KubeVolumeTypeConfigMap,
		Items:       map[string][]byte{},
		DefaultMode: v1.ConfigMapVolumeSourceDefaultMode,
	}
	for _, cm := range configMaps {
		if cm.Name == configMapVolumeSource.Name {
			matchedCM := cm
			// Set the source to the config map name
			kv.Source = cm.Name
			configMap = &matchedCM
			break
		}
	}
	// Set the defaultMode if set in the kube yaml
	validMode, err := isValidDefaultMode(configMapVolumeSource.DefaultMode)
	if err != nil {
		return nil, fmt.Errorf("invalid DefaultMode for configMap %q: %w", configMapVolumeSource.Name, err)
	}
	if validMode {
		kv.DefaultMode = *configMapVolumeSource.DefaultMode
	}

	if configMap == nil {
		// If the volumeSource was optional, move on even if a matching configmap wasn't found
		if configMapVolumeSource.Optional != nil && *configMapVolumeSource.Optional {
			kv.Source = configMapVolumeSource.Name
			kv.Optional = *configMapVolumeSource.Optional
			return kv, nil
		}
		return nil, fmt.Errorf("no such ConfigMap %q", configMapVolumeSource.Name)
	}

	// don't allow keys from "data" and "binaryData" to overlap
	for k := range configMap.Data {
		if _, ok := configMap.BinaryData[k]; ok {
			return nil, fmt.Errorf("the ConfigMap %q is invalid: duplicate key %q present in data and binaryData", configMap.Name, k)
		}
	}

	// If there are Items specified in the volumeSource, that overwrites the Data from the configmap
	if len(configMapVolumeSource.Items) > 0 {
		for _, item := range configMapVolumeSource.Items {
			if val, ok := configMap.Data[item.Key]; ok {
				kv.Items[item.Path] = []byte(val)
			} else if val, ok := configMap.BinaryData[item.Key]; ok {
				kv.Items[item.Path] = val
			}
		}
	} else {
		for k, v := range configMap.Data {
			kv.Items[k] = []byte(v)
		}
		maps.Copy(kv.Items, configMap.BinaryData)
	}
	return kv, nil
}

// Create a kubeVolume for an emptyDir volume
func VolumeFromEmptyDir(emptyDirVolumeSource *v1.EmptyDirVolumeSource, name string) (*KubeVolume, error) {
	if emptyDirVolumeSource.Medium == v1.StorageMediumMemory {
		return &KubeVolume{
			Type:   KubeVolumeTypeEmptyDirTmpfs,
			Source: name,
		}, nil
	} else {
		return &KubeVolume{
			Type:   KubeVolumeTypeEmptyDir,
			Source: name,
		}, nil
	}
}

func VolumeFromImage(imageVolumeSource *v1.ImageVolumeSource, _ string) (*KubeVolume, error) {
	return &KubeVolume{
		Type:            KubeVolumeTypeImage,
		Source:          imageVolumeSource.Reference,
		ImagePullPolicy: imageVolumeSource.PullPolicy,
	}, nil
}

// Create a KubeVolume from one of the supported VolumeSource
func VolumeFromSource(volumeSource v1.VolumeSource, configMaps []v1.ConfigMap, secretsManager *secrets.SecretsManager, volName, mountLabel string) (*KubeVolume, error) {
	switch {
	case volumeSource.HostPath != nil:
		return VolumeFromHostPath(volumeSource.HostPath, mountLabel)
	case volumeSource.PersistentVolumeClaim != nil:
		return VolumeFromPersistentVolumeClaim(volumeSource.PersistentVolumeClaim)
	case volumeSource.ConfigMap != nil:
		return VolumeFromConfigMap(volumeSource.ConfigMap, configMaps)
	case volumeSource.Secret != nil:
		return VolumeFromSecret(volumeSource.Secret, secretsManager)
	case volumeSource.EmptyDir != nil:
		return VolumeFromEmptyDir(volumeSource.EmptyDir, volName)
	case volumeSource.Image != nil:
		return VolumeFromImage(volumeSource.Image, volName)
	default:
		return nil, errors.New("HostPath, ConfigMap, EmptyDir, Secret, and PersistentVolumeClaim are currently the only supported VolumeSource")
	}
}

// Create a map of volume name to KubeVolume
func InitializeVolumes(specVolumes []v1.Volume, configMaps []v1.ConfigMap, secretsManager *secrets.SecretsManager, mountLabel string) (map[string]*KubeVolume, error) {
	volumes := make(map[string]*KubeVolume)

	for _, specVolume := range specVolumes {
		volume, err := VolumeFromSource(specVolume.VolumeSource, configMaps, secretsManager, specVolume.Name, mountLabel)
		if err != nil {
			return nil, fmt.Errorf("failed to create volume %q: %w", specVolume.Name, err)
		}

		volumes[specVolume.Name] = volume
	}

	return volumes, nil
}

// isValidDefaultMode returns true if mode is between 0 and 0777
func isValidDefaultMode(mode *int32) (bool, error) {
	if mode == nil {
		return false, nil
	}
	if *mode >= 0 && *mode <= int32(os.ModePerm) {
		return true, nil
	}
	return false, errors.New("must be between 0000 and 0777")
}
