package util

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	ruser "github.com/moby/sys/user"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"go.podman.io/storage/pkg/idtools"
	stypes "go.podman.io/storage/types"
)

func BreakInsert(mapping []idtools.IDMap, extension idtools.IDMap) (result []idtools.IDMap) {
	result = breakInsert(mapping, extension)
	result = sortAndMergeConsecutiveMappings(result)
	return result
}

//#####################

func TestBreakInsert1(t *testing.T) {
	// extension below mapping
	mapping := []idtools.IDMap{
		{
			ContainerID: 1000,
			HostID:      1000,
			Size:        1,
		},
	}
	extension := idtools.IDMap{
		ContainerID: 2,
		HostID:      2,
		Size:        1,
	}
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 2,
			HostID:      2,
			Size:        1,
		},
		{
			ContainerID: 1000,
			HostID:      1000,
			Size:        1,
		},
	}
	result := BreakInsert(mapping, extension)
	assert.Equal(t, result, expectedResult)
}

func TestBreakInsert2(t *testing.T) {
	// extension below mapping
	mapping := []idtools.IDMap{
		{
			ContainerID: 1000,
			HostID:      1001,
			Size:        2,
		},
	}
	extension := idtools.IDMap{
		ContainerID: 2,
		HostID:      3,
		Size:        2,
	}
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 2,
			HostID:      3,
			Size:        2,
		},
		{
			ContainerID: 1000,
			HostID:      1001,
			Size:        2,
		},
	}
	result := BreakInsert(mapping, extension)
	assert.Equal(t, expectedResult, result)
}

func TestBreakInsert3(t *testing.T) {
	// extension above mapping
	mapping := []idtools.IDMap{
		{
			ContainerID: 2,
			HostID:      3,
			Size:        2,
		},
	}
	extension := idtools.IDMap{
		ContainerID: 1000,
		HostID:      1001,
		Size:        2,
	}
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 2,
			HostID:      3,
			Size:        2,
		},
		{
			ContainerID: 1000,
			HostID:      1001,
			Size:        2,
		},
	}
	result := BreakInsert(mapping, extension)
	assert.Equal(t, expectedResult, result)
}

func TestBreakInsert4(t *testing.T) {
	// extension right below mapping
	mapping := []idtools.IDMap{
		{
			ContainerID: 4,
			HostID:      5,
			Size:        4,
		},
	}
	extension := idtools.IDMap{
		ContainerID: 1,
		HostID:      1,
		Size:        3,
	}
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 1,
			HostID:      1,
			Size:        3,
		},
		{
			ContainerID: 4,
			HostID:      5,
			Size:        4,
		},
	}
	result := BreakInsert(mapping, extension)
	assert.Equal(t, expectedResult, result)
}

func TestSortAndMergeConsecutiveMappings(t *testing.T) {
	// Extension and mapping are consecutive
	mapping := []idtools.IDMap{
		{
			ContainerID: 1,
			HostID:      1,
			Size:        3,
		},
		{
			ContainerID: 4,
			HostID:      4,
			Size:        4,
		},
	}
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 1,
			HostID:      1,
			Size:        7,
		},
	}
	result := sortAndMergeConsecutiveMappings(mapping)
	assert.Equal(t, expectedResult, result)
}

//#####################

func TestParseIDMap(t *testing.T) {
	mapSpec := []string{"+100000:@1002:1"}

	parentMapping := []ruser.IDMap{
		{
			ID:       int64(20),
			ParentID: int64(1002),
			Count:    1,
		},
	}
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 100000,
			HostID:      20,
			Size:        1,
		},
	}
	result, err := ParseIDMap(
		mapSpec,
		"UID",
		parentMapping,
	)
	assert.Equal(t, nil, err)
	assert.Equal(t, expectedResult, result)
}

func TestParseIDMapSizeMissing(t *testing.T) {
	// Size is 1 if not provided
	mapSpec := []string{"+100000:@1002"}

	parentMapping := []ruser.IDMap{
		{
			ID:       int64(20),
			ParentID: int64(1002),
			Count:    1,
		},
	}
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 100000,
			HostID:      20,
			Size:        1,
		},
	}
	result, err := ParseIDMap(
		mapSpec,
		"UID",
		parentMapping,
	)
	assert.Equal(t, nil, err)
	assert.Equal(t, expectedResult, result)
}

func TestParseIDMap2(t *testing.T) {
	mapSpec := []string{"0:2000:100000", "+1:100:1"}
	parentMapping := []ruser.IDMap(nil)
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 0,
			HostID:      2000,
			Size:        1,
		},
		{
			ContainerID: 1,
			HostID:      100,
			Size:        1,
		},
		{
			ContainerID: 2,
			HostID:      2002,
			Size:        99998,
		},
	}
	result, err := ParseIDMap(
		mapSpec,
		"UID",
		parentMapping,
	)
	assert.Equal(t, nil, err)
	assert.Equal(t, expectedResult, result)
}

func TestParseIDMap3(t *testing.T) {
	mapSpec := []string{"0:0:20", "24:24:6", "+7:1000:2", "+12:2000:3", "+18:3000:7"}
	parentMapping := []ruser.IDMap(nil)
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        7,
		},
		{
			ContainerID: 7,
			HostID:      1000,
			Size:        2,
		},
		{
			ContainerID: 9,
			HostID:      9,
			Size:        3,
		},
		{
			ContainerID: 12,
			HostID:      2000,
			Size:        3,
		},
		{
			ContainerID: 15,
			HostID:      15,
			Size:        3,
		},
		{
			ContainerID: 18,
			HostID:      3000,
			Size:        7,
		},
		{
			ContainerID: 25,
			HostID:      25,
			Size:        5,
		},
	}
	result, err := ParseIDMap(
		mapSpec,
		"UID",
		parentMapping,
	)
	assert.Equal(t, nil, err)
	assert.Equal(t, expectedResult, result)
}

func TestParseIDMap4(t *testing.T) {
	mapSpec := []string{"0:0:20", "+10:1:1"}
	parentMapping := []ruser.IDMap(nil)
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        1,
		},
		{
			ContainerID: 2,
			HostID:      2,
			Size:        8,
		},
		{
			ContainerID: 10,
			HostID:      1,
			Size:        1,
		},
		{
			ContainerID: 11,
			HostID:      11,
			Size:        9,
		},
	}
	result, err := ParseIDMap(
		mapSpec,
		"UID",
		parentMapping,
	)
	assert.Equal(t, nil, err)
	assert.Equal(t, expectedResult, result)
}

func TestParseIDMap5(t *testing.T) {
	mapSpec := []string{"0:20:10", "15:35:10", "+8:23:16"}
	parentMapping := []ruser.IDMap(nil)
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 0,
			HostID:      20,
			Size:        3,
		},
		{
			ContainerID: 8,
			HostID:      23,
			Size:        16,
		},
		{
			ContainerID: 24,
			HostID:      44,
			Size:        1,
		},
	}
	result, err := ParseIDMap(
		mapSpec,
		"UID",
		parentMapping,
	)
	assert.Equal(t, nil, err)
	assert.Equal(t, expectedResult, result)
}

func TestParseIDMapUserGroupFlags(t *testing.T) {
	mapSpec := []string{"u1:3:1", "g2:4:2"}
	parentMapping := []ruser.IDMap(nil)
	expectedResultUser := []idtools.IDMap{
		{
			ContainerID: 1,
			HostID:      3,
			Size:        1,
		},
	}
	expectedResultGroup := []idtools.IDMap{
		{
			ContainerID: 2,
			HostID:      4,
			Size:        2,
		},
	}
	result, err := ParseIDMap(mapSpec, "UID", parentMapping)
	assert.Equal(t, nil, err)
	assert.Equal(t, expectedResultUser, result)
	result, err = ParseIDMap(mapSpec, "GID", parentMapping)
	assert.Equal(t, err, nil)
	assert.Equal(t, expectedResultGroup, result)
}

func TestParseAutoIDMap(t *testing.T) {
	result, err := parseAutoIDMap("3:4:5", "UID", []ruser.IDMap{})
	assert.Equal(t, err, nil)
	assert.Equal(t, result, []idtools.IDMap{
		{
			ContainerID: 3,
			HostID:      4,
			Size:        5,
		},
	})
}

func TestParseAutoIDMapRelative(t *testing.T) {
	parentMapping := []ruser.IDMap{
		{
			ID:       0,
			ParentID: 1000,
			Count:    1,
		},
		{
			ID:       1,
			ParentID: 100000,
			Count:    65536,
		},
	}
	result, err := parseAutoIDMap("100:@100000:1", "UID", parentMapping)
	assert.Equal(t, err, nil)
	assert.Equal(t, result, []idtools.IDMap{
		{
			ContainerID: 100,
			HostID:      1,
			Size:        1,
		},
	})
}

func TestFillIDMap(t *testing.T) {
	availableRanges := [][2]int{{0, 10}, {10000, 20000}}
	idmap := []idtools.IDMap{
		{
			ContainerID: 1,
			HostID:      1000,
			Size:        10,
		},
		{
			ContainerID: 30,
			HostID:      2000,
			Size:        20,
		},
	}
	expectedResult := []idtools.IDMap{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        1,
		},
		{
			ContainerID: 1,
			HostID:      1000,
			Size:        10,
		},
		{
			ContainerID: 11,
			HostID:      1,
			Size:        9,
		},
		{
			ContainerID: 20,
			HostID:      10000,
			Size:        10,
		},
		{
			ContainerID: 30,
			HostID:      2000,
			Size:        20,
		},
		{
			ContainerID: 50,
			HostID:      10010,
			Size:        9990,
		},
	}
	result := fillIDMap(idmap, availableRanges)
	assert.Equal(t, expectedResult, result)
}

func TestGetAvailableIDRanges(t *testing.T) {
	all := [][2]int{{0, 30}, {50, 70}}
	used := [][2]int{{2, 4}, {25, 55}}
	expectedResult := [][2]int{{0, 2}, {4, 25}, {55, 70}}
	result := getAvailableIDRanges(all, used)
	assert.Equal(t, expectedResult, result)
}

func TestValidateSysctls(t *testing.T) {
	strSlice := []string{"net.core.test1=4", "kernel.msgmax=2"}
	result, _ := ValidateSysctls(strSlice)
	assert.Equal(t, result["net.core.test1"], "4")
}

func TestValidateSysctlBadSysctl(t *testing.T) {
	strSlice := []string{"BLAU=BLUE", "GELB^YELLOW"}
	_, err := ValidateSysctls(strSlice)
	assert.Error(t, err)
}

func TestValidateSysctlBadSysctlWithExtraSpaces(t *testing.T) {
	expectedError := "'%s' is invalid, extra spaces found"

	// should fail fast on first sysctl
	strSlice1 := []string{
		"net.ipv4.ping_group_range = 0 0",
		"net.ipv4.ping_group_range=0 0 ",
	}
	_, err := ValidateSysctls(strSlice1)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), fmt.Sprintf(expectedError, strSlice1[0]))

	// should fail on second sysctl
	strSlice2 := []string{
		"net.ipv4.ping_group_range=0 0",
		"net.ipv4.ping_group_range=0 0 ",
	}
	_, err = ValidateSysctls(strSlice2)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), fmt.Sprintf(expectedError, strSlice2[1]))
}

func TestCoresToPeriodAndQuota(t *testing.T) {
	cores := 1.0
	expectedPeriod := DefaultCPUPeriod
	expectedQuota := int64(DefaultCPUPeriod)

	actualPeriod, actualQuota := CoresToPeriodAndQuota(cores)
	assert.Equal(t, actualPeriod, expectedPeriod, "Period does not match")
	assert.Equal(t, actualQuota, expectedQuota, "Quota does not match")
}

func TestPeriodAndQuotaToCores(t *testing.T) {
	var (
		period        uint64 = 100000
		quota         int64  = 50000
		expectedCores        = 0.5
	)

	assert.Equal(t, PeriodAndQuotaToCores(period, quota), expectedCores)
}

func TestParseInputTime(t *testing.T) {
	tm, err := ParseInputTime("1.5", true)
	if err != nil {
		t.Errorf("expected error to be nil but was: %v", err)
	}

	expected, err := time.ParseInLocation(time.RFC3339Nano, "1970-01-01T00:00:01.500000000Z", time.UTC)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, expected, tm)
}

func TestConvertMappings(t *testing.T) {
	start := []specs.LinuxIDMapping{
		{
			ContainerID: 1,
			HostID:      2,
			Size:        3,
		},
		{
			ContainerID: 4,
			HostID:      5,
			Size:        6,
		},
	}

	converted := RuntimeSpecToIDtools(start)

	convertedBack := IDtoolsToRuntimeSpec(converted)

	assert.Equal(t, len(start), len(convertedBack))

	for i := range start {
		assert.Equal(t, start[i].ContainerID, convertedBack[i].ContainerID)
		assert.Equal(t, start[i].HostID, convertedBack[i].HostID)
		assert.Equal(t, start[i].Size, convertedBack[i].Size)
	}
}

func TestConvertTimeout(t *testing.T) {
	timeout := ConvertTimeout(0)
	assert.Equal(t, uint(0), timeout)

	timeout = ConvertTimeout(100)
	assert.Equal(t, uint(100), timeout)

	timeout = ConvertTimeout(-1)
	assert.Equal(t, uint(math.MaxUint32), timeout)

	timeout = ConvertTimeout(-100)
	assert.Equal(t, uint(math.MaxUint32), timeout)
}

func TestGetRootlessKeepIDMapping(t *testing.T) {
	tests := []struct {
		uid, gid                 int
		uids, gids               []idtools.IDMap
		size                     int
		expectedOptions          *stypes.IDMappingOptions
		expectedUID, expectedGID int
		expectedError            error
	}{
		{
			uid:  1000,
			gid:  1000,
			uids: []idtools.IDMap{},
			gids: []idtools.IDMap{},
			expectedOptions: &stypes.IDMappingOptions{
				HostUIDMapping: false,
				HostGIDMapping: false,
				UIDMap:         []idtools.IDMap{{ContainerID: 1000, HostID: 0, Size: 1}},
				GIDMap:         []idtools.IDMap{{ContainerID: 1000, HostID: 0, Size: 1}},
			},
			expectedUID: 1000,
			expectedGID: 1000,
		},
		{
			uid:  0,
			gid:  0,
			uids: []idtools.IDMap{{ContainerID: 0, HostID: 100000, Size: 65536}},
			gids: []idtools.IDMap{{ContainerID: 0, HostID: 100000, Size: 65536}},
			expectedOptions: &stypes.IDMappingOptions{
				HostUIDMapping: false,
				HostGIDMapping: false,
				UIDMap:         []idtools.IDMap{{ContainerID: 0, HostID: 0, Size: 1}, {ContainerID: 1, HostID: 1, Size: 65536}},
				GIDMap:         []idtools.IDMap{{ContainerID: 0, HostID: 0, Size: 1}, {ContainerID: 1, HostID: 1, Size: 65536}},
			},
			expectedUID: 0,
			expectedGID: 0,
		},
		{
			uid:  0,
			gid:  0,
			uids: []idtools.IDMap{{ContainerID: 0, HostID: 100000, Size: 65536}},
			gids: []idtools.IDMap{{ContainerID: 0, HostID: 100000, Size: 65536}},
			expectedOptions: &stypes.IDMappingOptions{
				HostUIDMapping: false,
				HostGIDMapping: false,
				UIDMap:         []idtools.IDMap{{ContainerID: 0, HostID: 0, Size: 1}, {ContainerID: 1, HostID: 1, Size: 1023}},
				GIDMap:         []idtools.IDMap{{ContainerID: 0, HostID: 0, Size: 1}, {ContainerID: 1, HostID: 1, Size: 1023}},
			},
			expectedUID: 0,
			expectedGID: 0,
			size:        1024,
		},
		{
			uid:  0,
			gid:  0,
			uids: []idtools.IDMap{{ContainerID: 0, HostID: 100000, Size: 65536}},
			gids: []idtools.IDMap{{ContainerID: 0, HostID: 100000, Size: 65536}},
			expectedOptions: &stypes.IDMappingOptions{
				HostUIDMapping: false,
				HostGIDMapping: false,
				UIDMap:         []idtools.IDMap{{ContainerID: 0, HostID: 0, Size: 1}},
				GIDMap:         []idtools.IDMap{{ContainerID: 0, HostID: 0, Size: 1}},
			},
			expectedUID: 0,
			expectedGID: 0,
			size:        1,
		},
		{
			uid:  0,
			gid:  0,
			uids: []idtools.IDMap{{ContainerID: 0, HostID: 100000, Size: 65536}},
			gids: []idtools.IDMap{{ContainerID: 0, HostID: 100000, Size: 65536}},
			expectedOptions: &stypes.IDMappingOptions{
				HostUIDMapping: false,
				HostGIDMapping: false,
				UIDMap:         []idtools.IDMap{{ContainerID: 0, HostID: 0, Size: 1}, {ContainerID: 1, HostID: 1, Size: 1}},
				GIDMap:         []idtools.IDMap{{ContainerID: 0, HostID: 0, Size: 1}, {ContainerID: 1, HostID: 1, Size: 1}},
			},
			expectedUID: 0,
			expectedGID: 0,
			size:        2,
		},
		{
			uid:  1000,
			gid:  1000,
			uids: []idtools.IDMap{},
			gids: []idtools.IDMap{},
			expectedOptions: &stypes.IDMappingOptions{
				HostUIDMapping: false,
				HostGIDMapping: false,
				UIDMap:         []idtools.IDMap{{ContainerID: 1000, HostID: 0, Size: 1}},
				GIDMap:         []idtools.IDMap{{ContainerID: 1000, HostID: 0, Size: 1}},
			},
			expectedUID: 1000,
			expectedGID: 1000,
			size:        1000000,
		},
	}

	for _, test := range tests {
		options, uid, gid, err := getRootlessKeepIDMapping(test.uid, test.gid, test.uids, test.gids, test.size)
		assert.NoError(t, err)
		assert.Equal(t, test.expectedOptions, options)
		assert.Equal(t, test.expectedUID, uid)
		assert.Equal(t, test.expectedGID, gid)
	}
}

func getDefaultMountOptionsNoStat(_ string) (defaultMountOptions, error) {
	return defaultMountOptions{false, true, true}, nil
}

func TestProcessOptions(t *testing.T) {
	tests := []struct {
		name             string
		options          []string
		isTmpfs          bool
		sourcePath       string
		expected         []string
		expectedNoCreate bool
		expectErr        bool
	}{
		{
			name:       "tmpfs",
			options:    []string{"rw", "size=512m", "noatime"},
			isTmpfs:    true,
			sourcePath: "",
			expected:   []string{"nodev", "nosuid", "rprivate", "rw", "size=512m", "tmpcopyup", "noatime"},
		},
		{
			name:       "duplicate idmap option",
			sourcePath: "/path/to/source",
			options:    []string{"idmap", "idmap"},
			expectErr:  true,
		},
		{
			name:       "mode allowed only with tmpfs",
			sourcePath: "/path/to/source",
			options:    []string{"rw", "rbind", "mode=0123"},
			expectErr:  true,
		},
		{
			name:       "noswap allowed only with tmpfs",
			sourcePath: "/path/to/source",
			options:    []string{"noswap"},
			expectErr:  true,
		},
		{
			name:       "tmpcopyup allowed only with tmpfs",
			sourcePath: "/path/to/source",
			options:    []string{"tmpcopyup"},
			expectErr:  true,
		},
		{
			name:       "notmpcopyup allowed only with tmpfs",
			sourcePath: "/path/to/source",
			options:    []string{"notmpcopyup"},
			expectErr:  true,
		},
		{
			name:       "z not allowed with tmpfs",
			isTmpfs:    true,
			sourcePath: "/path/to/source",
			options:    []string{"z"},
			expectErr:  true,
		},
		{
			name:       "size allowed only with tmpfs",
			sourcePath: "/path/to/source",
			options:    []string{"size=123456"},
			expectErr:  true,
		},
		{
			name:       "conflicting option dev/nodev",
			sourcePath: "/path/to/source",
			options:    []string{"dev", "nodev"},
			expectErr:  true,
		},
		{
			name:       "conflicting option suid/nosuid",
			sourcePath: "/path/to/source",
			options:    []string{"suid", "nosuid"},
			expectErr:  true,
		},
		{
			name:       "conflicting option exec/noexec",
			sourcePath: "/path/to/source",
			options:    []string{"noexec", "exec"},
			expectErr:  true,
		},
		{
			name:       "conflicting option ro/rw",
			sourcePath: "/path/to/source",
			options:    []string{"ro", "rw"},
			expectErr:  true,
		},
		{
			name:       "conflicting option bind/rbind",
			sourcePath: "/path/to/source",
			options:    []string{"bind", "rbind"},
			expectErr:  true,
		},
		{
			name:       "conflicting option bind/rbind",
			sourcePath: "/path/to/source",
			options:    []string{"bind", "rbind"},
			expectErr:  true,
		},
		{
			name:       "default bind mount",
			sourcePath: "/path/to/source",
			expected:   []string{"nodev", "nosuid", "rbind", "rprivate"},
		},
		{
			name:       "default bind mount with bind",
			sourcePath: "/path/to/source",
			options:    []string{"bind"},
			expected:   []string{"nodev", "nosuid", "bind", "private"},
		},
		{
			name:       "noatime allowed only with tmpfs",
			sourcePath: "/path/to/source",
			options:    []string{"noatime"},
			expectErr:  true,
		},
		{
			name:             "nocreate option is parsed and filtered",
			sourcePath:       "/path/to/source",
			options:          []string{"nocreate", "ro"},
			expected:         []string{"nodev", "nosuid", "rbind", "ro", "rprivate"},
			expectedNoCreate: true,
		},
		{
			name:             "nocreate with other options",
			sourcePath:       "/path/to/source",
			options:          []string{"rw", "nocreate", "z"},
			expected:         []string{"nodev", "nosuid", "rbind", "rprivate", "rw", "z"},
			expectedNoCreate: true,
		},
		{
			name:             "no nocreate option",
			sourcePath:       "/path/to/source",
			options:          []string{"ro"},
			expected:         []string{"nodev", "nosuid", "rbind", "ro", "rprivate"},
			expectedNoCreate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, noCreate, err := processOptionsInternal(tt.options, tt.isTmpfs, tt.sourcePath, getDefaultMountOptionsNoStat)
			if tt.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
				sort.Strings(opts)
				sort.Strings(tt.expected)
				assert.Equal(t, opts, tt.expected)
				assert.Equal(t, noCreate, tt.expectedNoCreate)
			}
		})
	}
}

func TestGetRootlessStateDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Not implemented on Windows")
	}
	dir, err := GetRootlessStateDir()
	assert.NoError(t, err)
	assert.NotEqual(t, dir, "libpod/tmp")
}

// https://github.com/containers/podman/issues/25458
func TestParseDockerignoreLeadingTrailingSlashes(t *testing.T) {
	contextDir := t.TempDir()

	for _, tt := range []struct {
		name     string
		ignore   string
		expected []string
	}{
		{"leading slash", "/.git/\n", []string{".git"}},
		{"trailing slash", "target/\n", []string{"target"}},
		{"both slashes", "/build/\n", []string{"build"}},
		{"no slashes", "vendor\n", []string{"vendor"}},
		{"slash only line", "/\n", []string{}},
		{"multiple patterns", "/.git/\n/target/\nvendor\n", []string{".git", "target", "vendor"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ignorePath := filepath.Join(contextDir, ".containerignore")
			err := os.WriteFile(ignorePath, []byte(tt.ignore), 0o644)
			assert.NoError(t, err)

			excludes, _, err := ParseDockerignore(nil, contextDir)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, excludes)
		})
	}
}
