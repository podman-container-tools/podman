package common_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
