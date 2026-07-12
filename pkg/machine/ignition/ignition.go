//go:build amd64 || arm64

package ignition

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"runtime"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/systemd/parser"
)

/*
	If this file gets too nuts, we can perhaps use existing go code
	to create ignition files.  At this point, the file is so simple
	that I chose to use structs and not import any code as I was
	concerned (unsubstantiated) about too much bloat coming in.

	https://github.com/openshift/machine-config-operator/blob/master/pkg/server/server.go
*/

const (
	PodmanDockerTmpConfPath = "/etc/tmpfiles.d/podman-docker.conf"
	DefaultIgnitionUserName = "core"
)

// Convenience function to convert int to ptr
func IntToPtr(i int) *int {
	return &i
}

// Convenience function to convert string to ptr
func StrToPtr(s string) *string {
	return &s
}

// Convenience function to convert bool to ptr
func BoolToPtr(b bool) *bool {
	return &b
}

func GetNodeUsr(usrName string) NodeUser {
	return NodeUser{Name: &usrName}
}

func GetNodeGrp(grpName string) NodeGroup {
	return NodeGroup{Name: &grpName}
}

type DynamicIgnition struct {
	Name       string
	Key        string
	TimeZone   string
	UID        int
	VMName     string
	VMType     define.VMType
	WritePath  string
	Cfg        Config
	Rootful    bool
	NetRecover bool
	Rosetta    bool
	Swap       uint64
}

func (ign *DynamicIgnition) Write() error {
	b, err := json.Marshal(ign.Cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(ign.WritePath, b, 0o644)
}

func (ign *DynamicIgnition) getUsers() []PasswdUser {
	var users []PasswdUser

	isCoreUser := ign.Name == DefaultIgnitionUserName

	// if we are not using the 'core' user, we need to tell ignition to
	// not add it
	if !isCoreUser {
		coreUser := PasswdUser{
			Name:        DefaultIgnitionUserName,
			ShouldExist: BoolToPtr(false),
		}
		users = append(users, coreUser)
	}

	// Adding the user
	user := PasswdUser{
		Name:              ign.Name,
		SSHAuthorizedKeys: []SSHAuthorizedKey{SSHAuthorizedKey(ign.Key)},
		UID:               IntToPtr(ign.UID),
	}

	// If we are not using the core user, we need to make the user part
	// of the following groups
	if !isCoreUser {
		user.Groups = []Group{
			Group("sudo"),
			Group("adm"),
			Group("wheel"),
			Group("systemd-journal"),
		}
	}

	// set root SSH key
	root := PasswdUser{
		Name:              "root",
		SSHAuthorizedKeys: []SSHAuthorizedKey{SSHAuthorizedKey(ign.Key)},
	}
	// add them all in
	users = append(users, user, root)

	return users
}

// GenerateIgnitionConfig
func (ign *DynamicIgnition) GenerateIgnitionConfig() error {
	if len(ign.Name) < 1 {
		ign.Name = DefaultIgnitionUserName
	}
	ignVersion := Ignition{
		Version: "3.2.0",
	}
	ignPassword := Passwd{
		Users: ign.getUsers(),
	}

	ignStorage := Storage{
		Directories: getDirs(ign.Name),
		Files:       getFiles(ign.Name, ign.UID, ign.Rootful, ign.VMType, ign.NetRecover, ign.Swap),
		Links:       getLinks(),
	}

	// Add or set the time zone for the machine
	if len(ign.TimeZone) > 0 {
		var err error
		tz := ign.TimeZone
		// local means the same as the host
		// look up where it is pointing to on the host
		if ign.TimeZone == "local" {
			if env, ok := os.LookupEnv("TZ"); ok {
				tz = env
			} else {
				tz, err = getLocalTimeZone()
				if err != nil {
					return fmt.Errorf("error getting local timezone: %q", err)
				}
			}
		}
		// getLocalTimeZone() can return empty string, do not add broken symlink in that case
		// coreos will default to UTC
		if tz == "" {
			logrus.Info("Unable to determine local timezone, machine will default to UTC")
		} else {
			tzLink := Link{
				Node: Node{
					Group:     GetNodeGrp("root"),
					Path:      "/etc/localtime",
					Overwrite: BoolToPtr(false),
					User:      GetNodeUsr("root"),
				},
				LinkEmbedded1: LinkEmbedded1{
					Hard: BoolToPtr(false),
					// We always want this value in unix form (../usr/share/zoneinfo) because this is being
					// set in the machine OS (always Linux) and systemd needs the relative symlink.  However,
					// filepath.join on windows will use a "\\" separator so use path.Join() which always
					// uses the slash.
					Target: path.Join("../usr/share/zoneinfo", tz),
				},
			}
			ignStorage.Links = append(ignStorage.Links, tzLink)
		}
	}

	ignSystemd := Systemd{
		Units: []Unit{
			{
				Enabled: BoolToPtr(true),
				Name:    "podman.socket",
			},
			{
				// TODO Need to understand if this could play a role in machine
				// updates given a certain configuration
				// Disable auto-updating of fcos images
				// https://github.com/containers/podman/issues/20122
				Enabled: BoolToPtr(false),
				Name:    "zincati.service",
			},
		},
	}

	// Only AppleHv with Apple Silicon can use Rosetta
	if ign.VMType == define.AppleHvVirt && runtime.GOARCH == "arm64" {
		rosettaUnit := Systemd{
			Units: []Unit{
				{
					Enabled: BoolToPtr(true),
					Name:    "rosetta-activation.service",
				},
			},
		}
		ignSystemd.Units = append(ignSystemd.Units, rosettaUnit.Units...)
	}

	// Only after all checks are done
	// it's ready create the ingConfig
	ign.Cfg = Config{
		Ignition: ignVersion,
		Passwd:   ignPassword,
		Storage:  ignStorage,
		Systemd:  ignSystemd,
	}
	return nil
}

func getDirs(usrName string) []Directory {
	// Ignition has a bug/feature? where if you make a series of dirs
	// in one swoop, then the leading dirs are creates as root.
	newDirs := []string{
		"/home/" + usrName + "/.config",
		"/home/" + usrName + "/.config/containers",
		"/home/" + usrName + "/.config/systemd",
		"/home/" + usrName + "/.config/systemd/user",
	}
	dirs := make([]Directory, len(newDirs))
	for i, d := range newDirs {
		newDir := Directory{
			Node: Node{
				Group: GetNodeGrp(usrName),
				Path:  d,
				User:  GetNodeUsr(usrName),
			},
			DirectoryEmbedded1: DirectoryEmbedded1{Mode: IntToPtr(0o755)},
		}
		dirs[i] = newDir
	}

	return dirs
}

func getFiles(usrName string, uid int, rootful bool, vmtype define.VMType, _ bool, swap uint64) []File {
	files := make([]File, 0)

	// enable linger mode for the user
	files = append(files, File{
		Node: Node{
			Group: GetNodeGrp("root"),
			Path:  "/var/lib/systemd/linger/" + usrName,
			User:  GetNodeUsr("root"),
			// the coreos image might already have this defined
			Overwrite: BoolToPtr(true),
		},
		FileEmbedded1: FileEmbedded1{
			Contents: Resource{
				Source: EncodeDataURLPtr(""),
			},
			Mode: IntToPtr(0o644),
		},
	})

	containers := `[containers]
netns="bridge"
pids_limit=0
`
	// TODO I think this can be removed but leaving breadcrumb until certain.
	// Set deprecated machine_enabled until podman package on fcos is
	// current enough to no longer require it
	// 	rootContainers := `[engine]
	// machine_enabled=true
	// `
	// Prevent subUID from clashing with actual UID
	subUID := 100000
	subUIDs := 1000000
	if uid >= subUID && uid < (subUID+subUIDs) {
		subUID = uid + 1
	}
	etcSubUID := fmt.Sprintf("%s:%d:%d\n", usrName, subUID, subUIDs)

	// Set containers.conf up for core user to use networks
	// by default
	files = append(files, File{
		Node: Node{
			Group: GetNodeGrp(usrName),
			Path:  "/home/" + usrName + "/.config/containers/containers.conf",
			User:  GetNodeUsr(usrName),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: EncodeDataURLPtr(containers),
			},
			Mode: IntToPtr(0o744),
		},
	})

	// Set up /etc/subuid and /etc/subgid
	for _, sub := range []string{"/etc/subuid", "/etc/subgid"} {
		files = append(files, File{
			Node: Node{
				Group:     GetNodeGrp("root"),
				Path:      sub,
				User:      GetNodeUsr("root"),
				Overwrite: BoolToPtr(true),
			},
			FileEmbedded1: FileEmbedded1{
				Append: nil,
				Contents: Resource{
					Source: EncodeDataURLPtr(etcSubUID),
				},
				Mode: IntToPtr(0o744),
			},
		})
	}

	// Set machine marker file to indicate podman what vmtype we are
	// operating under
	files = append(files, File{
		Node: Node{
			Group: GetNodeGrp("root"),
			// Files used to /etc/containers/podman-machine but had to be moved so we do not overmount it.
			Path: "/etc/podman-machine",
			User: GetNodeUsr("root"),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: EncodeDataURLPtr(fmt.Sprintf("%s\n", vmtype.String())),
			},
			Mode: IntToPtr(0o644),
		},
	})

	files = append(files, File{
		Node: Node{
			Path: PodmanDockerTmpConfPath,
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			// Create a symlink from the docker socket to the podman socket.
			Contents: Resource{
				Source: EncodeDataURLPtr(GetPodmanDockerTmpConfig(uid, rootful, true)),
			},
			Mode: IntToPtr(0o644),
		},
	})

	if swap > 0 {
		files = append(files, File{
			Node: Node{
				Path: "/etc/systemd/zram-generator.conf",
			},
			FileEmbedded1: FileEmbedded1{
				Append: nil,
				Contents: Resource{
					Source: EncodeDataURLPtr(fmt.Sprintf("[zram0]\nzram-size=%d\n", swap)),
				},
				Mode: IntToPtr(0o644),
			},
		})
	}

	return files
}

func getLinks() []Link {
	return []Link{{
		Node: Node{
			Group:     GetNodeGrp("root"),
			Path:      "/etc/systemd/user/sockets.target.wants/podman.socket",
			User:      GetNodeUsr("root"),
			Overwrite: BoolToPtr(true),
		},
		LinkEmbedded1: LinkEmbedded1{
			Hard:   BoolToPtr(false),
			Target: "/usr/lib/systemd/user/podman.socket",
		},
	}, {
		Node: Node{
			Group:     GetNodeGrp("root"),
			Path:      "/usr/local/bin/docker",
			Overwrite: BoolToPtr(true),
			User:      GetNodeUsr("root"),
		},
		LinkEmbedded1: LinkEmbedded1{
			Hard:   BoolToPtr(false),
			Target: "/usr/bin/podman",
		},
	}}
}

func EncodeDataURLPtr(contents string) *string {
	return StrToPtr(fmt.Sprintf("data:,%s", url.PathEscape(contents)))
}

func GetPodmanDockerTmpConfig(uid int, rootful bool, newline bool) string {
	// Derived from https://github.com/containers/podman/blob/main/contrib/systemd/system/podman-docker.conf
	podmanSock := "/run/podman/podman.sock"
	if !rootful {
		podmanSock = fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)
	}
	suffix := ""
	if newline {
		suffix = "\n"
	}

	return fmt.Sprintf("L+  /run/docker.sock   -    -    -     -   %s%s", podmanSock, suffix)
}

type IgnitionBuilder struct {
	dynamicIgnition DynamicIgnition
	units           []Unit
}

// NewIgnitionBuilder generates a new IgnitionBuilder type using the
// base `DynamicIgnition` object
func NewIgnitionBuilder(dynamicIgnition DynamicIgnition) IgnitionBuilder {
	return IgnitionBuilder{
		dynamicIgnition,
		[]Unit{},
	}
}

// GenerateIgnitionConfig generates the ignition config
func (i *IgnitionBuilder) GenerateIgnitionConfig() error {
	return i.dynamicIgnition.GenerateIgnitionConfig()
}

// WithUnit adds systemd units to the internal `DynamicIgnition` config
func (i *IgnitionBuilder) WithUnit(units ...Unit) {
	i.dynamicIgnition.Cfg.Systemd.Units = append(i.dynamicIgnition.Cfg.Systemd.Units, units...)
}

// WithFile adds storage files to the internal `DynamicIgnition` config
func (i *IgnitionBuilder) WithFile(files ...File) {
	i.dynamicIgnition.Cfg.Storage.Files = append(i.dynamicIgnition.Cfg.Storage.Files, files...)
}

// BuildWithIgnitionFile copies the provided ignition file into the internal
// `DynamicIgnition` write path
func (i *IgnitionBuilder) BuildWithIgnitionFile(ignPath string) error {
	inputIgnition, err := os.ReadFile(ignPath)
	if err != nil {
		return err
	}

	return os.WriteFile(i.dynamicIgnition.WritePath, inputIgnition, 0o644)
}

// Build writes the internal `DynamicIgnition` config to its write path
func (i *IgnitionBuilder) Build() error {
	logrus.Debugf("writing ignition file to %q", i.dynamicIgnition.WritePath)
	return i.dynamicIgnition.Write()
}

func (i *IgnitionBuilder) AddPlaybook(contents string, destPath string, username string) error {
	// create the ignition file object
	f := File{
		Node: Node{
			Group: GetNodeGrp(username),
			Path:  destPath,
			User:  GetNodeUsr(username),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: EncodeDataURLPtr(contents),
			},
			Mode: IntToPtr(0o744),
		},
	}

	// call ignitionBuilder.WithFile
	// add the config file to the ignition object
	i.WithFile(f)

	unit := parser.NewUnitFile()
	unit.Add("Unit", "After", "ready.service")
	unit.Add("Unit", "ConditionFirstBoot", "yes")
	unit.Add("Service", "Type", "oneshot")
	unit.Add("Service", "User", username)
	unit.Add("Service", "Group", username)
	unit.Add("Service", "ExecStart", fmt.Sprintf("ansible-playbook %s", destPath))
	unit.Add("Install", "WantedBy", "default.target")
	unitContents, err := unit.ToString()
	if err != nil {
		return err
	}

	// create a systemd service
	playbookUnit := Unit{
		Enabled:  BoolToPtr(true),
		Name:     "playbook.service",
		Contents: &unitContents,
	}
	i.WithUnit(playbookUnit)

	return nil
}

func DefaultReadyUnitFile() parser.UnitFile {
	u := parser.NewUnitFile()
	u.Add("Unit", "After", "sshd.socket sshd.service")
	u.Add("Unit", "OnFailure", "emergency.target")
	u.Add("Unit", "OnFailureJobMode", "isolate")
	u.Add("Service", "Type", "oneshot")
	u.Add("Service", "RemainAfterExit", "yes")
	u.Add("Install", "RequiredBy", "default.target")
	return *u
}
