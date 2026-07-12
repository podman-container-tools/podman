//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/shlex"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	envLib "go.podman.io/podman/v6/pkg/env"
	"go.podman.io/podman/v6/utils"
	"go.podman.io/storage/pkg/fileutils"
)

func (ic *ContainerEngine) ContainerRunlabel(ctx context.Context, label string, imageRef string, args []string, options entities.ContainerRunlabelOptions) error {
	pullOptions := &libimage.PullOptions{}
	pullOptions.AuthFilePath = options.Authfile
	pullOptions.CertDirPath = options.CertDir
	pullOptions.Credentials = options.Credentials
	pullOptions.SignaturePolicyPath = options.SignaturePolicy
	pullOptions.InsecureSkipTLSVerify = options.SkipTLSVerify

	pullPolicy := config.PullPolicyNever
	if options.Pull {
		pullPolicy = config.PullPolicyMissing
	}
	if !options.Quiet {
		pullOptions.Writer = os.Stderr
	}

	pulledImages, err := ic.Libpod.LibimageRuntime().Pull(ctx, imageRef, pullPolicy, pullOptions)
	if err != nil {
		return err
	}

	if len(pulledImages) != 1 {
		return errors.New("internal error: expected an image to be pulled (or an error)")
	}

	// Extract the runlabel from the image.
	labels, err := pulledImages[0].Labels(ctx)
	if err != nil {
		return err
	}

	var runlabel string
	for k, v := range labels {
		if strings.EqualFold(k, label) {
			runlabel = v
			break
		}
	}
	if runlabel == "" {
		return fmt.Errorf("cannot find the value of label: %s in image: %s", label, imageRef)
	}

	cmd, env, err := generateRunlabelCommand(runlabel, pulledImages[0], imageRef, args, options)
	if err != nil {
		return err
	}

	if options.Display {
		fmt.Printf("command: %s\n", strings.Join(append([]string{os.Args[0]}, cmd[1:]...), " "))
		return nil
	}

	stdErr := os.Stderr
	stdOut := os.Stdout
	stdIn := os.Stdin
	if options.Quiet {
		stdErr = nil
		stdOut = nil
		stdIn = nil
	}

	// If container already exists && --replace given -- Nuke it
	if options.Replace {
		for i, entry := range cmd {
			if entry == "--name" {
				name := cmd[i+1]
				ctr, err := ic.Libpod.LookupContainer(name)
				if err != nil {
					if !errors.Is(err, define.ErrNoSuchCtr) {
						logrus.Debugf("Error occurred searching for container %s: %v", name, err)
						return err
					}
				} else {
					logrus.Debugf("Runlabel --replace option given. Container %s will be deleted. The new container will be named %s", ctr.ID(), name)
					var timeout *uint
					if err := ic.Libpod.RemoveContainer(ctx, ctr, true, false, timeout); err != nil {
						return err
					}
				}
				break
			}
		}
	}

	return utils.ExecCmdWithStdStreams(stdIn, stdOut, stdErr, env, cmd[0], cmd[1:]...)
}

// generateRunlabelCommand generates the to-be-executed command as a string
// slice along with a base environment.
func generateRunlabelCommand(runlabel string, img *libimage.Image, inputName string, args []string, options entities.ContainerRunlabelOptions) ([]string, []string, error) {
	var (
		err             error
		name, imageName string
		cmd             []string
	)

	// Extract the imageName (or ID).
	imgNames := img.NamesHistory()
	if len(imgNames) == 0 {
		imageName = img.ID()
	} else {
		// The newest name is the first entry in the `NamesHistory`
		// slice.
		imageName = imgNames[0]
	}

	// Use the user-specified name or extract one from the image.
	name = options.Name
	if name == "" {
		normalize := imageName
		if !strings.HasPrefix(img.ID(), inputName) {
			normalize = inputName
		}
		splitImageName := strings.Split(normalize, "/")
		name = splitImageName[len(splitImageName)-1]
		// make sure to remove the tag from the image name, otherwise the name cannot
		// be used as container name because a colon is an illegal character
		name, _, _ = strings.Cut(name, ":")
	}

	// Append the user-specified arguments to the runlabel (command).
	if len(args) > 0 {
		runlabel = fmt.Sprintf("%s %s", runlabel, strings.Join(args, " "))
	}

	cmd, err = generateCommand(runlabel, imageName, name)
	if err != nil {
		return nil, nil, err
	}

	env := generateRunEnvironment(options)
	env = append(env, "PODMAN_RUNLABEL_NESTED=1")
	envmap, err := envLib.ParseSlice(env)
	if err != nil {
		return nil, nil, err
	}

	envmapper := func(k string) string {
		switch k {
		case "OPT1":
			return envmap["OPT1"]
		case "OPT2":
			return envmap["OPT2"]
		case "OPT3":
			return envmap["OPT3"]
		case "PWD":
			// I would prefer to use os.getenv but it appears PWD is not in the os env list.
			d, err := os.Getwd()
			if err != nil {
				logrus.Error("Unable to determine current working directory")
				return ""
			}
			return d
		case "HOME":
			h, err := os.UserHomeDir()
			if err != nil {
				logrus.Warnf("Unable to determine user's home directory: %s", err)
				return ""
			}
			return h
		}
		return ""
	}
	newS := os.Expand(strings.Join(cmd, " "), envmapper)
	cmd, err = shlex.Split(newS)
	if err != nil {
		return nil, nil, err
	}
	return cmd, env, nil
}

func replaceName(arg, name string) string {
	if arg == "NAME" {
		return name
	}

	newarg := strings.ReplaceAll(arg, "$NAME", name)
	newarg = strings.ReplaceAll(newarg, "${NAME}", name)
	if strings.HasSuffix(newarg, "=NAME") {
		newarg = strings.ReplaceAll(newarg, "=NAME", fmt.Sprintf("=%s", name))
	}
	return newarg
}

func replaceImage(arg, image string) string {
	if arg == "IMAGE" {
		return image
	}
	newarg := strings.ReplaceAll(arg, "$IMAGE", image)
	newarg = strings.ReplaceAll(newarg, "${IMAGE}", image)
	if strings.HasSuffix(newarg, "=IMAGE") {
		newarg = strings.ReplaceAll(newarg, "=IMAGE", fmt.Sprintf("=%s", image))
	}
	return newarg
}

// generateCommand takes a label (string) and converts it to an executable command
func generateCommand(command, imageName, name string) ([]string, error) {
	if name == "" {
		name = imageName
	}

	cmd, err := shlex.Split(command)
	if err != nil {
		return nil, err
	}

	prog, err := substituteCommand(cmd[0])
	if err != nil {
		return nil, err
	}
	newCommand := []string{prog}
	for _, arg := range cmd[1:] {
		var newArg string
		switch arg {
		case "IMAGE=IMAGE":
			newArg = fmt.Sprintf("IMAGE=%s", imageName)
		case "NAME=NAME":
			newArg = fmt.Sprintf("NAME=%s", name)
		default:
			newArg = replaceName(arg, name)
			newArg = replaceImage(newArg, imageName)
		}
		newCommand = append(newCommand, newArg)
	}
	return newCommand, nil
}

// GenerateRunEnvironment merges the current environment variables with optional
// environment variables provided by the user
func generateRunEnvironment(options entities.ContainerRunlabelOptions) []string {
	newEnv := os.Environ()
	if options.Optional1 != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT1=%s", options.Optional1))
	}
	if options.Optional2 != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT2=%s", options.Optional2))
	}
	if options.Optional3 != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT3=%s", options.Optional3))
	}
	return newEnv
}

func substituteCommand(cmd string) (string, error) {
	var newCommand string

	// Replace cmd with "/proc/self/exe" if "podman" or "docker" is being
	// used. If "/usr/bin/docker" is provided, we also sub in podman.
	// Otherwise, leave the command unchanged.
	if cmd == "podman" || filepath.Base(cmd) == "docker" {
		newCommand = "/proc/self/exe"
	} else {
		newCommand = cmd
	}

	// If cmd is an absolute or relative path, check if the file exists.
	// Throw an error if it doesn't exist.
	if strings.Contains(newCommand, "/") || strings.HasPrefix(newCommand, ".") {
		res, err := filepath.Abs(newCommand)
		if err != nil {
			return "", err
		}
		if err := fileutils.Exists(res); !errors.Is(err, fs.ErrNotExist) {
			return res, nil
		} else if err != nil {
			return "", err
		}
	}

	return newCommand, nil
}
