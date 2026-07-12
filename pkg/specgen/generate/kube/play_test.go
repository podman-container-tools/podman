//go:build linux && !remote

package kube

import (
	"math"
	"runtime"
	"strconv"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"go.podman.io/common/pkg/secrets"
	"go.podman.io/podman/v6/libpod/define"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/api/resource"
	v12 "go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/apis/meta/v1"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/util/intstr"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/storage/pkg/system"
	"sigs.k8s.io/yaml"
)

func createSecrets(t *testing.T, d string) *secrets.SecretsManager {
	secretsManager, err := secrets.NewManager(d)
	assert.NoError(t, err)

	driver := "file"
	driverOpts := map[string]string{
		"path": d,
	}

	storeOpts := secrets.StoreOptions{
		DriverOpts: driverOpts,
	}

	for _, s := range k8sSecrets {
		data, err := yaml.Marshal(s)
		assert.NoError(t, err)

		_, err = secretsManager.Store(s.ObjectMeta.Name, data, driver, storeOpts)
		assert.NoError(t, err)
	}

	return secretsManager
}

func TestConfigMapVolumes(t *testing.T) {
	yes := true
	tests := []struct {
		name          string
		volume        v1.Volume
		configmaps    []v1.ConfigMap
		errorMessage  string
		expectedItems map[string][]byte
	}{
		{
			"VolumeFromConfigmap",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "bar",
						},
					},
				},
			},
			configMapList,
			"",
			map[string][]byte{"myvar": []byte("bar")},
		},
		{
			"VolumeFromBinaryConfigmap",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "binary-bar",
						},
					},
				},
			},
			configMapList,
			"",
			map[string][]byte{"myvar": []byte("bin-bar")},
		},
		{
			"ConfigmapMissing",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "fizz",
						},
					},
				},
			},
			configMapList,
			`no such ConfigMap "fizz"`,
			map[string][]byte{},
		},
		{
			"ConfigmapMissingOptional",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "fizz",
						},
						Optional: &yes,
					},
				},
			},
			configMapList,
			"",
			map[string][]byte{},
		},
		{
			"MultiValue",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "multi-item",
						},
						Optional: &yes,
					},
				},
			},
			configMapList,
			"",
			map[string][]byte{"foo": []byte("bar"), "fizz": []byte("buzz")},
		},
		{
			"SpecificValue",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "multi-item",
						},
						Optional: &yes,
						Items:    []v1.KeyToPath{{Key: "fizz", Path: "/custom/path"}},
					},
				},
			},
			configMapList,
			"",
			map[string][]byte{"/custom/path": []byte("buzz")},
		},
		{
			"MultiValueBinary",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "multi-binary-item",
						},
						Optional: &yes,
					},
				},
			},
			configMapList,
			"",
			map[string][]byte{"foo": []byte("bin-bar"), "fizz": []byte("bin-buzz")},
		},
		{
			"SpecificValueBinary",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "multi-binary-item",
						},
						Optional: &yes,
						Items:    []v1.KeyToPath{{Key: "fizz", Path: "/custom/path"}},
					},
				},
			},
			configMapList,
			"",
			map[string][]byte{"/custom/path": []byte("bin-buzz")},
		},
		{
			"DuplicateValues",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "dupe",
						},
					},
				},
			},
			configMapList,
			`the ConfigMap "dupe" is invalid: duplicate key "foo" present in data and binaryData`,
			map[string][]byte{},
		},
		{
			"DuplicateValuesSpecific",
			v1.Volume{
				Name: "test-volume",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "dupe",
						},
						Items: []v1.KeyToPath{{Key: "fizz", Path: "/custom/path"}},
					},
				},
			},
			configMapList,
			`the ConfigMap "dupe" is invalid: duplicate key "foo" present in data and binaryData`,
			map[string][]byte{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := VolumeFromConfigMap(test.volume.ConfigMap, test.configmaps)
			if test.errorMessage == "" {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedItems, result.Items)
			} else {
				assert.Error(t, err)
				assert.Equal(t, test.errorMessage, err.Error())
			}
		})
	}
}

func TestEnvVarsFrom(t *testing.T) {
	d := t.TempDir()
	secretsManager := createSecrets(t, d)

	tests := []struct {
		name     string
		envFrom  v1.EnvFromSource
		options  CtrSpecGenOptions
		succeed  bool
		expected map[string]string
	}{
		{
			"ConfigMapExists",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "foo",
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
			map[string]string{
				"myvar": "foo",
			},
		},
		{
			"ConfigMapDoesNotExist",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "doesnotexist",
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			false,
			nil,
		},
		{
			"OptionalConfigMapDoesNotExist",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "doesnotexist",
					},
					Optional: &optional,
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
			map[string]string{},
		},
		{
			"EmptyConfigMapList",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "foo",
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: []v1.ConfigMap{},
			},
			false,
			nil,
		},
		{
			"OptionalEmptyConfigMapList",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "foo",
					},
					Optional: &optional,
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: []v1.ConfigMap{},
			},
			true,
			map[string]string{},
		},
		{
			"SecretExists",
			v1.EnvFromSource{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "foo",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			map[string]string{
				"myvar": "foo",
			},
		},
		{
			"SecretDoesNotExist",
			v1.EnvFromSource{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "doesnotexist",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			false,
			nil,
		},
		{
			"SecretExistsMultipleDataEntries",
			v1.EnvFromSource{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "multi-data",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			map[string]string{
				"myvar":  "foo",
				"myvar1": "foo1",
			},
		},
		{
			"SecretExistsMultipleStringDataEntries",
			v1.EnvFromSource{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "multi-stringdata",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			map[string]string{
				"myvar":  "foo",
				"myvar1": "foo1",
			},
		},
		{
			"SecretExistsMultipleDataStringDataEntries",
			v1.EnvFromSource{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "multi-data-stringdata",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			map[string]string{
				"myvardata":   "foodata",
				"myvar1":      "foo1string", // stringData overwrites data
				"myvarstring": "foostring",
			},
		},
		{
			"OptionalSecretDoesNotExist",
			v1.EnvFromSource{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "doesnotexist",
					},
					Optional: &optional,
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			map[string]string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := envVarsFrom(test.envFrom, &test.options)
			assert.Equal(t, err == nil, test.succeed)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestEnvVarValue(t *testing.T) {
	d := t.TempDir()
	secretsManager := createSecrets(t, d)
	stringNumCPUs := strconv.Itoa(runtime.NumCPU())

	mi, err := system.ReadMemInfo()
	assert.NoError(t, err)
	stringMemTotal := strconv.FormatInt(mi.MemTotal, 10)

	tests := []struct {
		name     string
		envVar   v1.EnvVar
		options  CtrSpecGenOptions
		succeed  bool
		expected string
	}{
		{
			"ConfigMapExists",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "myvar",
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
			"foo",
		},
		{
			"ContainerKeyDoesNotExistInConfigMap",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "doesnotexist",
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			false,
			nilString,
		},
		{
			"OptionalContainerKeyDoesNotExistInConfigMap",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key:      "doesnotexist",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
			nilString,
		},
		{
			"ConfigMapDoesNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "doesnotexist",
						},
						Key: "myvar",
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			false,
			nilString,
		},
		{
			"OptionalConfigMapDoesNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "doesnotexist",
						},
						Key:      "myvar",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
			nilString,
		},
		{
			"EmptyConfigMapList",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "myvar",
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: []v1.ConfigMap{},
			},
			false,
			nilString,
		},
		{
			"OptionalEmptyConfigMapList",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key:      "myvar",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: []v1.ConfigMap{},
			},
			true,
			nilString,
		},
		{
			"SecretExists",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "myvar",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			"foo",
		},
		{
			"ContainerKeyDoesNotExistInSecret",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "doesnotexist",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			false,
			nilString,
		},
		{
			"OptionalContainerKeyDoesNotExistInSecret",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key:      "doesnotexist",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			nilString,
		},
		{
			"SecretDoesNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "doesnotexist",
						},
						Key: "myvar",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			false,
			nilString,
		},
		{
			"OptionalSecretDoesNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "doesnotexist",
						},
						Key:      "myvar",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			nilString,
		},
		{
			"FieldRefMetadataName",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			CtrSpecGenOptions{
				PodName: "test",
			},
			true,
			"test",
		},
		{
			"FieldRefMetadataUID",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.uid",
					},
				},
			},
			CtrSpecGenOptions{
				PodID: "ec71ff37c67b688598c0008187ab0960dc34e1dfdcbf3a74e3d778bafcfe0977",
			},
			true,
			"ec71ff37c67b688598c0008187ab0960dc34e1dfdcbf3a74e3d778bafcfe0977",
		},
		{
			"FieldRefMetadataLabelsExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels['label']",
					},
				},
			},
			CtrSpecGenOptions{
				Labels: map[string]string{"label": "label"},
			},
			true,
			"label",
		},
		{
			"FieldRefMetadataLabelsEmpty",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels['label']",
					},
				},
			},
			CtrSpecGenOptions{
				Labels: map[string]string{"label": ""},
			},
			true,
			"",
		},
		{
			"FieldRefMetadataLabelsNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels['label']",
					},
				},
			},
			CtrSpecGenOptions{},
			true,
			"",
		},
		{
			"FieldRefMetadataAnnotationsExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['annotation']",
					},
				},
			},
			CtrSpecGenOptions{
				Annotations: map[string]string{"annotation": "annotation"},
			},
			true,
			"annotation",
		},
		{
			"FieldRefMetadataAnnotationsEmpty",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['annotation']",
					},
				},
			},
			CtrSpecGenOptions{
				Annotations: map[string]string{"annotation": ""},
			},
			true,
			"",
		},
		{
			"FieldRefMetadataAnnotationsNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['annotation']",
					},
				},
			},
			CtrSpecGenOptions{},
			true,
			"",
		},
		{
			"FieldRefInvalid1",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['annotation]",
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"FieldRefInvalid2",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.dummy['annotation']",
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"FieldRefNotSupported",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"ResourceFieldRefNotSupported",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.dummy",
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"ResourceFieldRefMemoryDivisorNotValid",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.memory",
						Divisor:  resource.MustParse("2M"),
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"ResourceFieldRefCpuDivisorNotValid",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.cpu",
						Divisor:  resource.MustParse("2m"),
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"ResourceFieldRefNoDivisor",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.memory",
					},
				},
			},
			CtrSpecGenOptions{
				Container: container,
			},
			true,
			memoryString,
		},
		{
			"ResourceFieldRefMemoryDivisor",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.memory",
						Divisor:  resource.MustParse("1Mi"),
					},
				},
			},
			CtrSpecGenOptions{
				Container: container,
			},
			true,
			strconv.Itoa(int(math.Ceil(float64(memoryInt) / 1024 / 1024))),
		},
		{
			"ResourceFieldRefCpuDivisor",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "requests.cpu",
						Divisor:  resource.MustParse("1m"),
					},
				},
			},
			CtrSpecGenOptions{
				Container: container,
			},
			true,
			strconv.Itoa(int(float64(cpuInt) / 0.001)),
		},
		{
			"ResourceFieldRefNoLimitMemory",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.memory",
					},
				},
			},
			CtrSpecGenOptions{
				Container: v1.Container{
					Name: "test",
				},
			},
			true,
			stringMemTotal,
		},
		{
			"ResourceFieldRefNoRequestMemory",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "requests.memory",
					},
				},
			},
			CtrSpecGenOptions{
				Container: v1.Container{
					Name: "test",
				},
			},
			true,
			stringMemTotal,
		},
		{
			"ResourceFieldRefNoLimitCPU",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.cpu",
					},
				},
			},
			CtrSpecGenOptions{
				Container: v1.Container{
					Name: "test",
				},
			},
			true,
			stringNumCPUs,
		},
		{
			"ResourceFieldRefNoRequestCPU",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "requests.cpu",
					},
				},
			},
			CtrSpecGenOptions{
				Container: v1.Container{
					Name: "test",
				},
			},
			true,
			stringNumCPUs,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := envVarValue(test.envVar, &test.options)
			assert.Equal(t, err == nil, test.succeed)
			if test.expected == nilString {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, test.expected, *result)
			}
		})
	}
}

var (
	nilString     = "<nil>"
	configMapList = []v1.ConfigMap{
		{
			TypeMeta: v12.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "bar",
			},
			Data: map[string]string{
				"myvar": "bar",
			},
		},
		{
			TypeMeta: v12.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "foo",
			},
			Data: map[string]string{
				"myvar": "foo",
			},
		},
		{
			TypeMeta: v12.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "binary-bar",
			},
			BinaryData: map[string][]byte{
				"myvar": []byte("bin-bar"),
			},
		},
		{
			TypeMeta: v12.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "multi-item",
			},
			Data: map[string]string{
				"foo":  "bar",
				"fizz": "buzz",
			},
		},
		{
			TypeMeta: v12.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "multi-binary-item",
			},
			BinaryData: map[string][]byte{
				"foo":  []byte("bin-bar"),
				"fizz": []byte("bin-buzz"),
			},
		},
		{
			TypeMeta: v12.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "dupe",
			},
			BinaryData: map[string][]byte{
				"fiz": []byte("bin-buzz"),
				"foo": []byte("bin-bar"),
			},
			Data: map[string]string{
				"foo": "bar",
			},
		},
	}

	optional = true

	k8sSecrets = []v1.Secret{
		{
			TypeMeta: v12.TypeMeta{
				Kind: "Secret",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "bar",
			},
			Data: map[string][]byte{
				"myvar": []byte("bar"),
			},
		},
		{
			TypeMeta: v12.TypeMeta{
				Kind: "Secret",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "foo",
			},
			Data: map[string][]byte{
				"myvar": []byte("foo"),
			},
		},
		{
			TypeMeta: v12.TypeMeta{
				Kind: "Secret",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "multi-data",
			},
			Data: map[string][]byte{
				"myvar":  []byte("foo"),
				"myvar1": []byte("foo1"),
			},
		},
		{
			TypeMeta: v12.TypeMeta{
				Kind: "Secret",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "multi-stringdata",
			},
			StringData: map[string]string{
				"myvar":  string("foo"),
				"myvar1": string("foo1"),
			},
		},
		{
			TypeMeta: v12.TypeMeta{
				Kind: "Secret",
			},
			ObjectMeta: v12.ObjectMeta{
				Name: "multi-data-stringdata",
			},
			Data: map[string][]byte{
				"myvardata": []byte("foodata"),
				"myvar1":    []byte("foo1data"),
			},
			StringData: map[string]string{
				"myvarstring": string("foostring"),
				"myvar1":      string("foo1string"),
			},
		},
	}

	cpuInt       = 4
	cpuString    = strconv.Itoa(cpuInt)
	memoryInt    = 30000000
	memoryString = strconv.Itoa(memoryInt)
	container    = v1.Container{
		Name: "test",
		Resources: v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(cpuString),
				v1.ResourceMemory: resource.MustParse(memoryString),
			},
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(cpuString),
				v1.ResourceMemory: resource.MustParse(memoryString),
			},
		},
	}
)

func TestHttpLivenessProbe(t *testing.T) {
	tests := []struct {
		name          string
		specGenerator specgen.SpecGenerator
		container     v1.Container
		restartPolicy string
		succeed       bool
		expectedURL   string
	}{
		{
			"HttpLivenessProbeUrlSetCorrectly",
			specgen.SpecGenerator{},
			v1.Container{
				LivenessProbe: &v1.Probe{
					Handler: v1.Handler{
						HTTPGet: &v1.HTTPGetAction{
							Scheme: "http",
							Host:   "127.0.0.1",
							Port:   intstr.FromInt(8080),
							Path:   "/health",
						},
					},
				},
			},
			"always",
			true,
			"http://127.0.0.1:8080/health",
		},
		{
			"HttpLivenessProbeUrlUsesDefaults",
			specgen.SpecGenerator{},
			v1.Container{
				LivenessProbe: &v1.Probe{
					Handler: v1.Handler{
						HTTPGet: &v1.HTTPGetAction{
							Port: intstr.FromInt(80),
						},
					},
				},
			},
			"always",
			true,
			"http://localhost:80/",
		},
		{
			"HttpLivenessProbeNamedPort",
			specgen.SpecGenerator{},
			v1.Container{
				LivenessProbe: &v1.Probe{
					Handler: v1.Handler{
						HTTPGet: &v1.HTTPGetAction{
							Port: intstr.FromString("httpPort"),
						},
					},
				},
				Ports: []v1.ContainerPort{
					{Name: "servicePort", ContainerPort: 7000},
					{Name: "httpPort", ContainerPort: 8000},
				},
			},
			"always",
			true,
			"http://localhost:8000/",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := setupLivenessProbe(&test.specGenerator, test.container, test.restartPolicy)
			if err == nil {
				assert.Equal(t, err == nil, test.succeed)
				assert.Contains(t, test.specGenerator.ContainerHealthCheckConfig.HealthConfig.Test, test.expectedURL)
			}
		})
	}
}

func TestTCPLivenessProbe(t *testing.T) {
	tests := []struct {
		name          string
		specGenerator specgen.SpecGenerator
		container     v1.Container
		restartPolicy string
		succeed       bool
		expectedHost  string
		expectedPort  string
	}{
		{
			"TCPLivenessProbeNormal",
			specgen.SpecGenerator{},
			v1.Container{
				LivenessProbe: &v1.Probe{
					Handler: v1.Handler{
						TCPSocket: &v1.TCPSocketAction{
							Host: "127.0.0.1",
							Port: intstr.FromInt(8080),
						},
					},
				},
			},
			"always",
			true,
			"127.0.0.1",
			"8080",
		},
		{
			"TCPLivenessProbeHostUsesDefault",
			specgen.SpecGenerator{},
			v1.Container{
				LivenessProbe: &v1.Probe{
					Handler: v1.Handler{
						TCPSocket: &v1.TCPSocketAction{
							Port: intstr.FromInt(200),
						},
					},
				},
			},
			"always",
			true,
			"localhost",
			"200",
		},
		{
			"TCPLivenessProbeUseNamedPort",
			specgen.SpecGenerator{},
			v1.Container{
				LivenessProbe: &v1.Probe{
					Handler: v1.Handler{
						TCPSocket: &v1.TCPSocketAction{
							Port: intstr.FromString("servicePort"),
							Host: "myservice.domain.com",
						},
					},
				},
				Ports: []v1.ContainerPort{
					{ContainerPort: 6000},
					{Name: "servicePort", ContainerPort: 4000},
					{Name: "2ndServicePort", ContainerPort: 3000},
				},
			},
			"always",
			true,
			"myservice.domain.com",
			"4000",
		},
		{
			"TCPLivenessProbeInvalidPortName",
			specgen.SpecGenerator{},
			v1.Container{
				LivenessProbe: &v1.Probe{
					Handler: v1.Handler{
						TCPSocket: &v1.TCPSocketAction{
							Port: intstr.FromString("3rdservicePort"),
							Host: "myservice.domain.com",
						},
					},
				},
				Ports: []v1.ContainerPort{
					{ContainerPort: 6000},
					{Name: "servicePort", ContainerPort: 4000},
					{Name: "2ndServicePort", ContainerPort: 3000},
				},
			},
			"always",
			false,
			"myservice.domain.com",
			"4000",
		},
		{
			"TCPLivenessProbeNormalWithOnFailureRestartPolicy",
			specgen.SpecGenerator{},
			v1.Container{
				LivenessProbe: &v1.Probe{
					Handler: v1.Handler{
						TCPSocket: &v1.TCPSocketAction{
							Host: "127.0.0.1",
							Port: intstr.FromInt(8080),
						},
					},
				},
			},
			"on-failure",
			true,
			"127.0.0.1",
			"8080",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := setupLivenessProbe(&test.specGenerator, test.container, test.restartPolicy)
			assert.Equal(t, err == nil, test.succeed)
			if err == nil {
				assert.Equal(t, int(test.specGenerator.ContainerHealthCheckConfig.HealthCheckOnFailureAction), define.HealthCheckOnFailureActionRestart)
				assert.Contains(t, test.specGenerator.ContainerHealthCheckConfig.HealthConfig.Test, test.expectedHost)
				assert.Contains(t, test.specGenerator.ContainerHealthCheckConfig.HealthConfig.Test, test.expectedPort)
			}
		})
	}
}

func TestDeviceResource(t *testing.T) {
	tests := []struct {
		name          string
		specGenerator specgen.SpecGenerator
		container     v1.Container
		succeed       bool
		devices       []spec.LinuxDevice
	}{
		{
			"ParseQualifiedCDI",
			specgen.SpecGenerator{},
			v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						"nvidia.com/gpu=0": resource.MustParse("1"),
					},
				},
			},
			true,
			[]spec.LinuxDevice{
				{Path: "nvidia.com/gpu=0"},
			},
		},
		{
			"ParsePodmanDeviceResource",
			specgen.SpecGenerator{},
			v1.Container{
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"podman.io/device=/dev/kmsg": resource.MustParse("1"),
					},
				},
			},
			true,
			[]spec.LinuxDevice{
				{Path: "/dev/kmsg"},
			},
		},
		{
			"InvalidCDI",
			specgen.SpecGenerator{},
			v1.Container{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						"foobar.net/class=///": resource.MustParse("1"),
					},
				},
			},
			false,
			[]spec.LinuxDevice{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := setupContainerDevices(&test.specGenerator, test.container)
			assert.Equal(t, err == nil, test.succeed)
			if err == nil {
				assert.Equal(t, test.specGenerator.Devices, test.devices)
			}
		})
	}
}
