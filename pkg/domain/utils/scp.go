package utils

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/ssh"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func ExecuteTransfer(src, dst string, opts entities.ScpExecuteTransferOptions) (*entities.ScpExecuteTransferReport, error) {
	source := entities.ScpTransferImageOptions{}
	dest := entities.ScpTransferImageOptions{}
	sshInfo := entities.ImageScpConnections{}
	loadReport := entities.ScpLoadReport{Names: []string{}}

	podman, err := os.Executable()
	if err != nil {
		return nil, err
	}

	f, err := os.CreateTemp("", "podman") // open temp file for load/save output
	if err != nil {
		return nil, err
	}

	locations := []*entities.ScpTransferImageOptions{}
	cliConnections := []string{}
	args := []string{src}
	if len(dst) > 0 {
		args = append(args, dst)
	}
	for _, arg := range args {
		loc, connect, err := ParseImageSCPArg(arg)
		if err != nil {
			return nil, err
		}
		locations = append(locations, loc)
		cliConnections = append(cliConnections, connect...)
	}
	source = *locations[0]
	switch {
	case len(locations) > 1:
		if err = ValidateSCPArgs(locations); err != nil {
			return nil, err
		}
		dest = *locations[1]
	case len(locations) == 1:
		switch {
		case len(locations[0].Image) == 0:
			return nil, fmt.Errorf("no source image specified: %w", define.ErrInvalidArg)
		case len(locations[0].Image) > 0 && !locations[0].Remote && len(locations[0].User) == 0: // if we have podman image scp $IMAGE
			return nil, fmt.Errorf("must specify a destination: %w", define.ErrInvalidArg)
		}
	}

	source.Quiet = opts.Quiet
	source.File = f.Name() // after parsing the arguments, set the file for the save/load
	dest.File = source.File
	defer os.Remove(source.File)

	allLocal := true // if we are all localhost, do not validate connections but if we are using one localhost and one non we need to use sshd
	for _, val := range cliConnections {
		if !strings.Contains(val, "@localhost::") {
			allLocal = false
			break
		}
	}
	if allLocal {
		cliConnections = []string{}
	}

	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	err = GetServiceInformation(&sshInfo, cliConnections, cfg)
	if err != nil {
		return nil, err
	}

	switch {
	case source.Remote: // if we want to load FROM the remote, dest can either be local or remote in this case
		saveToRemoteOpts := entities.ScpSaveToRemoteOptions{}
		saveToRemoteOpts.Image = source.Image
		saveToRemoteOpts.LocalFile = source.File
		saveToRemoteOpts.Tag = ""
		saveToRemoteOpts.URL = sshInfo.URI[0]
		saveToRemoteOpts.Iden = sshInfo.Identities[0]
		saveToRemoteOpts.SSHMode = opts.SSHMode
		saveToRemoteOpts.Format = opts.SaveFormat
		_, err = SaveToRemote(saveToRemoteOpts)
		if err != nil {
			return nil, err
		}
		if dest.Remote { // we want to load remote -> remote, both source and dest are remote
			loadToRemoteOpts := entities.ScpLoadToRemoteOptions{}
			loadToRemoteOpts.Dest = dest
			loadToRemoteOpts.LocalFile = dest.File
			loadToRemoteOpts.Tag = ""
			loadToRemoteOpts.URL = sshInfo.URI[1]
			loadToRemoteOpts.Iden = sshInfo.Identities[1]
			loadToRemoteOpts.SSHMode = opts.SSHMode
			loadToRemoteRep, err := LoadToRemote(loadToRemoteOpts)
			if err != nil {
				return nil, err
			}
			if len(loadToRemoteRep.Response) > 0 {
				fmt.Println(loadToRemoteRep.Response)
			}
			if len(loadToRemoteRep.ID) > 0 {
				loadReport.Names = append(loadReport.Names, loadToRemoteRep.ID)
			}
			break
		}
		loadCmd := []string{podman}
		loadCmd = append(loadCmd, opts.ParentFlags...)
		loadCmd = append(loadCmd, "load")
		if source.Quiet {
			loadCmd = append(loadCmd, "-q")
		}
		loadCmd = append(loadCmd, "--input", dest.File)
		id, err := ExecPodman(dest, podman, loadCmd)
		if err != nil {
			return nil, err
		}
		if len(id) > 0 {
			loadReport.Names = append(loadReport.Names, id)
		}
	case dest.Remote: // remote host load, implies source is local
		saveCmd := []string{podman}
		saveCmd = append(saveCmd, opts.ParentFlags...)
		saveCmd = append(saveCmd, "save")
		if opts.SaveFormat != "" {
			saveCmd = append(saveCmd, "--format", opts.SaveFormat)
		}
		if source.Quiet {
			saveCmd = append(saveCmd, "-q")
		}
		saveCmd = append(saveCmd, "--output", source.File, source.Image)
		_, err = ExecPodman(dest, podman, saveCmd)
		if err != nil {
			return nil, err
		}

		loadToRemoteOpts := entities.ScpLoadToRemoteOptions{}
		loadToRemoteOpts.Dest = dest
		loadToRemoteOpts.LocalFile = source.File
		loadToRemoteOpts.Tag = ""
		loadToRemoteOpts.URL = sshInfo.URI[0]
		loadToRemoteOpts.Iden = sshInfo.Identities[0]
		loadToRemoteOpts.SSHMode = opts.SSHMode
		loadToRemoteRep, err := LoadToRemote(loadToRemoteOpts)
		if err != nil {
			return nil, err
		}
		if len(loadToRemoteRep.Response) > 0 {
			fmt.Println(loadToRemoteRep.Response)
		}
		if len(loadToRemoteRep.ID) > 0 {
			loadReport.Names = append(loadReport.Names, loadToRemoteRep.ID)
		}
		if err = os.Remove(source.File); err != nil {
			return nil, err
		}
	default: // else native load, both source and dest are local and transferring between users
		if source.User == "" { // source user has to be set, destination does not
			source.User = os.Getenv("USER")
			if source.User == "" {
				u, err := user.Current()
				if err != nil {
					return nil, fmt.Errorf("could not obtain user, make sure the environmental variable $USER is set: %w", err)
				}
				source.User = u.Username
			}
		}
		rep := entities.ScpExecuteTransferReport{}
		rep.Source = &source
		rep.Dest = &dest
		rep.ParentFlags = opts.ParentFlags
		rep.SaveFormat = opts.SaveFormat
		return &rep, nil // transfer needs to be done in ABI due to cross issues
	}

	rep := entities.ScpExecuteTransferReport{}
	rep.LoadReport = &loadReport
	return &rep, nil
}

// CreateSCPCommand takes an existing command, appends the given arguments and returns a configured podman command for image scp
func CreateSCPCommand(cmd *exec.Cmd, command []string) *exec.Cmd {
	cmd.Args = append(cmd.Args, command...)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd
}

// ScpTag is a helper function for native podman to tag an image after a local load from image SCP
func ScpTag(cmd *exec.Cmd, podman string, dest entities.ScpTransferImageOptions) error {
	cmd.Stdout = nil
	out, err := cmd.Output() // this function captures the output temporarily in order to execute the next command
	if err != nil {
		return err
	}
	image := ExtractImage(out)
	if cmd.Args[0] == "sudo" { // transferRootless will need the sudo since we are loading to sudo from a user acct
		cmd = exec.Command("sudo", podman, "tag", image, dest.Tag)
	} else {
		cmd = exec.Command(podman, "tag", image, dest.Tag)
	}
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// ExtractImage pulls out the last line of output from save/load (image id)
func ExtractImage(out []byte) string {
	fmt.Println(strings.TrimSuffix(string(out), "\n"))         // print output
	stringOut := string(out)                                   // get all output
	arrOut := strings.Split(stringOut, " ")                    // split it into an array
	return strings.ReplaceAll(arrOut[len(arrOut)-1], "\n", "") // replace the trailing \n
}

// LoginUser starts the user process on the host so that image scp can use systemd-run
func LoginUser(user string) (*exec.Cmd, error) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		return nil, err
	}
	machinectl, err := exec.LookPath("machinectl")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(machinectl, "shell", "-q", user+"@.host", sleep, "inf")
	err = cmd.Start()
	return cmd, err
}

// LoadToRemote takes image and remote connection information. it connects to the specified client
// and copies the saved image dir over to the remote host and then loads it onto the machine
// returns a report containing ssh response string and the id of the loaded image, or an error
func LoadToRemote(opts entities.ScpLoadToRemoteOptions) (*entities.ScpLoadToRemoteReport, error) {
	port := 0
	urlPort := opts.URL.Port()
	if urlPort != "" {
		var err error
		port, err = strconv.Atoi(opts.URL.Port())
		if err != nil {
			return nil, err
		}
	}

	input, err := os.Open(opts.LocalFile)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	out, err := ssh.ExecWithInput(&ssh.ConnectionExecOptions{Host: opts.URL.String(), Identity: opts.Iden, Port: port, User: opts.URL.User, Args: []string{"podman", "image", "load"}}, opts.SSHMode, input)
	if err != nil {
		return nil, err
	}
	if opts.Tag != "" {
		return nil, fmt.Errorf("renaming of an image is currently not supported: %w", define.ErrInvalidArg)
	}
	rep := strings.TrimSuffix(out, "\n")
	outArr := strings.Split(rep, " ")
	id := outArr[len(outArr)-1]
	if len(opts.Dest.Tag) > 0 { // tag the remote image using the output ID
		_, err := ssh.Exec(&ssh.ConnectionExecOptions{Host: opts.URL.String(), Identity: opts.Iden, Port: port, User: opts.URL.User, Args: []string{"podman", "image", "tag", id, opts.Dest.Tag}}, opts.SSHMode)
		if err != nil {
			return nil, err
		}
	}
	return &entities.ScpLoadToRemoteReport{Response: rep, ID: id}, nil
}

// SaveToRemote takes image information and remote connection information. it connects to the specified client
// and saves the specified image on the remote machine and then copies it to the specified local location
// returns an error if one occurs.
func SaveToRemote(opts entities.ScpSaveToRemoteOptions) (*entities.ScpSaveToRemoteReport, error) {
	if opts.Tag != "" {
		return nil, fmt.Errorf("renaming of an image is currently not supported: %w", define.ErrInvalidArg)
	}

	port := 0
	urlPort := opts.URL.Port()
	if urlPort != "" {
		var err error
		port, err = strconv.Atoi(opts.URL.Port())
		if err != nil {
			return nil, err
		}
	}

	remoteFile, err := ssh.Exec(&ssh.ConnectionExecOptions{Host: opts.URL.String(), Identity: opts.Iden, Port: port, User: opts.URL.User, Args: []string{"mktemp"}}, opts.SSHMode)
	if err != nil {
		return nil, err
	}

	saveArgs := []string{"podman", "image", "save", opts.Image}
	if opts.Format != "" {
		saveArgs = append(saveArgs, "--format", opts.Format)
	}

	saveArgs = append(saveArgs, "--output", remoteFile)

	_, err = ssh.Exec(&ssh.ConnectionExecOptions{Host: opts.URL.String(), Identity: opts.Iden, Port: port, User: opts.URL.User, Args: saveArgs}, opts.SSHMode)
	if err != nil {
		return nil, err
	}

	scpConnOpts := ssh.ConnectionScpOptions{User: opts.URL.User, Identity: opts.Iden, Port: port, Source: "ssh://" + opts.URL.User.String() + "@" + opts.URL.Hostname() + ":" + remoteFile, Destination: opts.LocalFile}
	scpRep, err := ssh.Scp(&scpConnOpts, opts.SSHMode)
	if err != nil {
		return nil, err
	}
	_, err = ssh.Exec(&ssh.ConnectionExecOptions{Host: opts.URL.String(), Identity: opts.Iden, Port: port, User: opts.URL.User, Args: []string{"rm", scpRep}}, opts.SSHMode)
	if err != nil {
		logrus.Errorf("Removing file on endpoint: %v", err)
	}

	return &entities.ScpSaveToRemoteReport{}, nil
}

// execPodman executes the podman save/load command given the podman binary
func ExecPodman(dest entities.ScpTransferImageOptions, podman string, command []string) (string, error) {
	cmd := exec.Command(podman)
	CreateSCPCommand(cmd, command[1:])
	logrus.Debugf("Executing podman command: %q", cmd)
	if strings.Contains(strings.Join(command, " "), "load") { // need to tag
		if len(dest.Tag) > 0 {
			return "", ScpTag(cmd, podman, dest)
		}
		cmd.Stdout = nil
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		img := ExtractImage(out)
		return img, nil
	}
	return "", cmd.Run()
}

// parseImageSCPArg returns the valid connection, and source/destination data based off of the information provided by the user
// arg is a string containing one of the cli arguments returned is a filled out source/destination options structs as well as a connections array and an error if applicable
func ParseImageSCPArg(arg string) (*entities.ScpTransferImageOptions, []string, error) {
	location := entities.ScpTransferImageOptions{}
	var err error
	cliConnections := []string{}

	switch {
	case strings.Contains(arg, "@localhost::"): // image transfer between users
		// Split on "@localhost::" rather than the first "@" so usernames
		// that themselves contain an "@" (e.g. Active Directory "user@domain")
		// are preserved. See https://github.com/containers/podman/issues/27655
		location.User, _, _ = strings.Cut(arg, "@localhost::")
		location, err = ValidateImagePortion(location, arg)
		if err != nil {
			return nil, nil, err
		}
		cliConnections = append(cliConnections, arg)
	case strings.Contains(arg, "::"):
		location, err = ValidateImagePortion(location, arg)
		if err != nil {
			return nil, nil, err
		}
		location.Remote = true
		cliConnections = append(cliConnections, arg)
	default:
		location.Image = arg
	}
	return &location, cliConnections, nil
}

func ValidateImagePortion(location entities.ScpTransferImageOptions, arg string) (entities.ScpTransferImageOptions, error) {
	if RemoteArgLength(arg, 1) > 0 {
		before := strings.Split(arg, "::")[1]
		name := ValidateImageName(before)
		if before != name {
			location.Image = name
		} else {
			location.Image = before
		} // this will get checked/set again once we validate connections
	}
	return location, nil
}

// validateImageName makes sure that the image given is valid and no injections are occurring
// we simply use this for error checking, bot setting the image
func ValidateImageName(input string) string {
	// ParseNormalizedNamed transforms a shortname image into its
	// full name reference so busybox => docker.io/library/busybox
	// we want to keep our shortnames, so only return an error if
	// we cannot parse what the user has given us
	if ref, err := alltransports.ParseImageName(input); err == nil {
		return ref.Transport().Name()
	}
	return input
}

// validateSCPArgs takes the array of source and destination options and checks for common errors
func ValidateSCPArgs(locations []*entities.ScpTransferImageOptions) error {
	if len(locations) > 2 {
		return fmt.Errorf("cannot specify more than two arguments: %w", define.ErrInvalidArg)
	}
	switch {
	case len(locations[0].Image) > 0 && len(locations[1].Image) > 0:
		locations[1].Tag = locations[1].Image
		locations[1].Image = ""
	case len(locations[0].Image) == 0 && len(locations[1].Image) == 0:
		return fmt.Errorf("a source image must be specified: %w", define.ErrInvalidArg)
	}
	return nil
}

// remoteArgLength is a helper function to simplify the extracting of host argument data
// returns an int which contains the length of a specified index in a host::image string
func RemoteArgLength(input string, side int) int {
	if strings.Contains(input, "::") {
		return len((strings.Split(input, "::"))[side])
	}
	return -1
}

// GetServiceInformation takes the parsed list of hosts to connect to and validates the information
func GetServiceInformation(sshInfo *entities.ImageScpConnections, cliConnections []string, cfg *config.Config) error {
	var urlS string
	var iden string
	for i, val := range cliConnections {
		connection, _, _ := strings.Cut(val, "::")
		sshInfo.Connections = append(sshInfo.Connections, connection)
		conn, err := cfg.GetConnection(sshInfo.Connections[i], false)
		if err == nil {
			// connection found
			urlS = conn.URI
			iden = conn.Identity
		} else { // no match, warn user and do a manual connection.
			urlS = "ssh://" + sshInfo.Connections[i]
			iden = ""
			logrus.Warnf("Unknown connection name given. Please use system connection add to specify the default remote socket location")
		}
		urlFinal, err := url.Parse(urlS) // create an actual url to pass to exec command
		if err != nil {
			return err
		}
		if urlFinal.User.Username() == "" {
			if urlFinal.User, err = GetUserInfo(urlFinal); err != nil {
				return err
			}
		}
		sshInfo.URI = append(sshInfo.URI, urlFinal)
		sshInfo.Identities = append(sshInfo.Identities, iden)
	}
	return nil
}

func GetUserInfo(uri *url.URL) (*url.Userinfo, error) {
	var (
		usr *user.User
		err error
	)
	if u, found := os.LookupEnv("_CONTAINERS_ROOTLESS_UID"); found {
		usr, err = user.LookupId(u)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup rootless user: %w", err)
		}
	} else {
		usr, err = user.Current()
		if err != nil {
			return nil, fmt.Errorf("failed to obtain current user: %w", err)
		}
	}

	pw, set := uri.User.Password()
	if set {
		return url.UserPassword(usr.Username, pw), nil
	}
	return url.User(usr.Username), nil
}
