package common_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func TestPodOptions(t *testing.T) {
	entry := "/test1"
	exampleOptions := entities.ContainerCreateOptions{CPUS: 5.5, CPUSetCPUs: "0-4", Entrypoint: &entry, Hostname: "foo", Name: "testing123", Volume: []string{"/fakeVol1", "/fakeVol2"}, Net: &entities.NetOptions{DNSSearch: []string{"search"}}, PID: "ns:/proc/self/ns"}

	podOptions := entities.PodCreateOptions{}
	err := common.ContainerToPodOptions(&exampleOptions, &podOptions)
	assert.NoError(t, err)

	cc := reflect.ValueOf(&exampleOptions).Elem()
	pc := reflect.ValueOf(&podOptions).Elem()

	pcType := reflect.TypeFor[entities.PodCreateOptions]()
	for i := 0; i < pc.NumField(); i++ {
		podField := pc.FieldByIndex([]int{i})
		podType := pcType.Field(i)
		for j := 0; j < cc.NumField(); j++ {
			containerField := cc.FieldByIndex([]int{j})
			containerType := reflect.TypeFor[entities.ContainerCreateOptions]().Field(j)
			tagPod, _, _ := strings.Cut(podType.Tag.Get("json"), ",")
			tagContainer, _, _ := strings.Cut(containerType.Tag.Get("json"), ",")
			if tagPod == tagContainer && (tagPod != "" && tagContainer != "") {
				areEqual := true
				if containerField.Kind() == podField.Kind() {
					switch containerField.Kind() {
					case reflect.Slice:
						for i, w := range containerField.Interface().([]string) {
							areEqual = podField.Interface().([]string)[i] == w
						}
					case reflect.String:
						areEqual = podField.String() == containerField.String()
					case reflect.Bool:
						areEqual = podField.Bool() == containerField.Bool()
					case reflect.Pointer:
						areEqual = reflect.DeepEqual(podField.Elem().Interface(), containerField.Elem().Interface())
					}
				}
				assert.True(t, areEqual)
			}
		}
	}
}

func TestSubuidSubgidNameMappings(t *testing.T) {
	// Regression test for issue #29885
	// Ensure that --subuidname populates SubUIDName and --subgidname populates SubGIDName
	cf := entities.ContainerCreateOptions{}
	cmd := &cobra.Command{}
	common.DefineCreateFlags(cmd, &cf, entities.CreateMode)

	// Parse --subuidname
	err := cmd.ParseFlags([]string{"--subuidname", "testsubuid"})
	require.NoError(t, err)
	assert.Equal(t, "testsubuid", cf.SubUIDName, "subuidname should populate SubUIDName")
	assert.Empty(t, cf.SubGIDName, "subuidname should not populate SubGIDName")

	// Parse --subgidname
	cf = entities.ContainerCreateOptions{}
	cmd = &cobra.Command{}
	common.DefineCreateFlags(cmd, &cf, entities.CreateMode)
	err = cmd.ParseFlags([]string{"--subgidname", "testsubgid"})
	require.NoError(t, err)
	assert.Equal(t, "testsubgid", cf.SubGIDName, "subgidname should populate SubGIDName")
	assert.Empty(t, cf.SubUIDName, "subgidname should not populate SubUIDName")
}
