package namespaces

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type usernsModeTest struct {
	name        string
	mode        UsernsMode
	isHost      bool
	isKeepID    bool
	isNoMap     bool
	isAuto      bool
	isDefault   bool
	isPrivate   bool
	isNS        bool
	nsPath      string
	isContainer bool
	container   string
	valid       bool
}

func TestUsernsMode(t *testing.T) {
	tests := []usernsModeTest{
		{name: "empty", mode: "", isDefault: true, isPrivate: true, valid: true},
		{name: "host", mode: "host", isHost: true, valid: true},
		{name: "private", mode: "private", isPrivate: true, valid: true},
		{name: "keep-id", mode: "keep-id", isKeepID: true, isPrivate: true, valid: true},
		{name: "keep-id with options", mode: "keep-id:uid=1000", isKeepID: true, isPrivate: true, valid: true},
		{name: "nomap", mode: "nomap", isNoMap: true, isPrivate: true, valid: true},
		{name: "auto", mode: "auto", isAuto: true, isPrivate: true, valid: true},
		{name: "auto with options", mode: "auto:size=1000", isAuto: true, isPrivate: true, valid: true},
		{name: "ns path", mode: "ns:/run/userns/x", isNS: true, nsPath: "/run/userns/x", isPrivate: true, valid: true},
		{name: "container", mode: "container:ctr1", isContainer: true, container: "ctr1", valid: true},
		{name: "container without name", mode: "container", isPrivate: true, valid: false},
		{name: "container with empty name", mode: "container:", isContainer: true, valid: false},
		{name: "invalid", mode: "bogus", isPrivate: true, valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isHost, tt.mode.IsHost(), "IsHost")
			assert.Equal(t, tt.isKeepID, tt.mode.IsKeepID(), "IsKeepID")
			assert.Equal(t, tt.isNoMap, tt.mode.IsNoMap(), "IsNoMap")
			assert.Equal(t, tt.isAuto, tt.mode.IsAuto(), "IsAuto")
			assert.Equal(t, tt.isDefault, tt.mode.IsDefaultValue(), "IsDefaultValue")
			assert.Equal(t, tt.isPrivate, tt.mode.IsPrivate(), "IsPrivate")
			assert.Equal(t, tt.isNS, tt.mode.IsNS(), "IsNS")
			if tt.isNS {
				assert.Equal(t, tt.nsPath, tt.mode.NS(), "NS")
			}
			assert.Equal(t, tt.isContainer, tt.mode.IsContainer(), "IsContainer")
			assert.Equal(t, tt.container, tt.mode.Container(), "Container")
			assert.Equal(t, tt.valid, tt.mode.Valid(), "Valid")
		})
	}
}

func TestUsernsModeGetKeepIDOptions(t *testing.T) {
	t.Run("wrong mode errors", func(t *testing.T) {
		_, err := UsernsMode("host").GetKeepIDOptions()
		assert.Error(t, err)
	})

	t.Run("keep-id without options", func(t *testing.T) {
		opts, err := UsernsMode("keep-id").GetKeepIDOptions()
		require.NoError(t, err)
		assert.Nil(t, opts.UID)
		assert.Nil(t, opts.GID)
		assert.Nil(t, opts.MaxSize)
	})

	t.Run("keep-id with uid, gid and size", func(t *testing.T) {
		opts, err := UsernsMode("keep-id:uid=1000,gid=2000,size=65536").GetKeepIDOptions()
		require.NoError(t, err)
		require.NotNil(t, opts.UID)
		require.NotNil(t, opts.GID)
		require.NotNil(t, opts.MaxSize)
		assert.Equal(t, uint32(1000), *opts.UID)
		assert.Equal(t, uint32(2000), *opts.GID)
		assert.Equal(t, uint32(65536), *opts.MaxSize)
	})

	t.Run("non-numeric value errors", func(t *testing.T) {
		_, err := UsernsMode("keep-id:uid=abc").GetKeepIDOptions()
		assert.Error(t, err)
	})

	t.Run("option without a value errors", func(t *testing.T) {
		_, err := UsernsMode("keep-id:uid").GetKeepIDOptions()
		assert.Error(t, err)
	})

	t.Run("unknown option errors", func(t *testing.T) {
		_, err := UsernsMode("keep-id:bogus=1").GetKeepIDOptions()
		assert.Error(t, err)
	})
}

type networkModeTest struct {
	name          string
	mode          NetworkMode
	isNone        bool
	isHost        bool
	isDefault     bool
	isBridge      bool
	isPasta       bool
	isPod         bool
	isPrivate     bool
	isContainer   bool
	container     string
	isNS          bool
	nsPath        string
	isUserDefined bool
	userDefined   string
}

func TestNetworkMode(t *testing.T) {
	tests := []networkModeTest{
		{name: "none", mode: "none", isNone: true, isPrivate: true},
		{name: "host", mode: "host", isHost: true},
		{name: "default", mode: "default", isDefault: true, isPrivate: true},
		{name: "bridge", mode: "bridge", isBridge: true, isPrivate: true},
		{name: "pasta", mode: "pasta", isPasta: true, isPrivate: true},
		{name: "pasta with options", mode: "pasta:-T,5", isPasta: true, isPrivate: true},
		{name: "pod", mode: "pod", isPod: true, isPrivate: true},
		{name: "container", mode: "container:web", isContainer: true, container: "web"},
		{name: "user defined", mode: "mynet", isPrivate: true, isUserDefined: true, userDefined: "mynet"},
		{name: "ns path", mode: "ns:/run/netns/x", isPrivate: true, isNS: true, nsPath: "/run/netns/x"},
		{name: "name starting with ns", mode: "nsproxy", isPrivate: true, isUserDefined: true, userDefined: "nsproxy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isNone, tt.mode.IsNone(), "IsNone")
			assert.Equal(t, tt.isHost, tt.mode.IsHost(), "IsHost")
			assert.Equal(t, tt.isDefault, tt.mode.IsDefault(), "IsDefault")
			assert.Equal(t, tt.isBridge, tt.mode.IsBridge(), "IsBridge")
			assert.Equal(t, tt.isPasta, tt.mode.IsPasta(), "IsPasta")
			assert.Equal(t, tt.isPod, tt.mode.IsPod(), "IsPod")
			assert.Equal(t, tt.isPrivate, tt.mode.IsPrivate(), "IsPrivate")
			assert.Equal(t, tt.isContainer, tt.mode.IsContainer(), "IsContainer")
			assert.Equal(t, tt.container, tt.mode.Container(), "Container")
			assert.Equal(t, tt.isNS, tt.mode.IsNS(), "IsNS")
			if tt.isNS {
				assert.Equal(t, tt.nsPath, tt.mode.NS(), "NS")
			}
			assert.Equal(t, tt.isUserDefined, tt.mode.IsUserDefined(), "IsUserDefined")
			assert.Equal(t, tt.userDefined, tt.mode.UserDefined(), "UserDefined")
		})
	}
}
