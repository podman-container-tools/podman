package common

import (
	"bufio"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/spf13/cobra"
	libimageDefine "go.podman.io/common/libimage/define"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/ssh"
	"go.podman.io/image/v5/pkg/sysregistriesv2"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/inspect"
	"go.podman.io/podman/v6/pkg/signal"
	systemdDefine "go.podman.io/podman/v6/pkg/systemd/define"
	"go.podman.io/podman/v6/pkg/util"
)

var (
	// ChangeCmds is the list of valid Change commands to passed to the Commit call
	ChangeCmds = []string{"CMD", "ENTRYPOINT", "ENV", "EXPOSE", "LABEL", "ONBUILD", "STOPSIGNAL", "USER", "VOLUME", "WORKDIR"}
	// LogLevels supported by podman
	LogLevels = []string{"trace", "debug", "info", "warn", "warning", "error", "fatal", "panic"}
	// ValidSaveFormats is the list of support podman save formats
	ValidSaveFormats = []string{define.OCIManifestDir, define.OCIArchive, define.V2s2ManifestDir, define.V2s2Archive}
	// ValidScpFormats is the list of formats for podman image scp (archive types only)
	ValidScpFormats = []string{define.OCIArchive, define.V2s2Archive}
)

type completeType int

const (
	// complete names and IDs after two chars
	completeDefault completeType = iota
	// only complete IDs
	completeIDs
	// only complete Names
	completeNames
)

type keyValueCompletion map[string]func(s string) ([]string, cobra.ShellCompDirective)

func setupContainerEngine(cmd *cobra.Command) (entities.ContainerEngine, error) {
	containerEngine, err := registry.NewContainerEngine(cmd, []string{})
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, err
	}
	if !registry.IsRemote() {
		_, noMoveProcess := cmd.Annotations[registry.NoMoveProcess]
		cgroupMode := ""

		if flag := cmd.LocalFlags().Lookup("cgroups"); flag != nil {
			cgroupMode = flag.Value.String()
		}

		err := containerEngine.SetupRootless(registry.Context(), noMoveProcess, cgroupMode)
		if err != nil {
			return nil, err
		}
	}
	return containerEngine, nil
}

func setupImageEngine(cmd *cobra.Command) (entities.ImageEngine, error) {
	imageEngine, err := registry.NewImageEngine(cmd, []string{})
	if err != nil {
		return nil, err
	}
	// we also need to set up the container engine since this
	// is required to set up the rootless namespace
	if _, err = setupContainerEngine(cmd); err != nil {
		return nil, err
	}
	return imageEngine, nil
}

func getContainers(cmd *cobra.Command, toComplete string, cType completeType, statuses ...string) ([]string, cobra.ShellCompDirective) {
	var listContainers []entities.ListContainer
	suggestions := []string{}
	listOpts := entities.ContainerListOptions{
		Filters: make(map[string][]string),
	}
	listOpts.All = true
	listOpts.Pod = true
	if len(statuses) > 0 {
		listOpts.Filters["status"] = statuses
	}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, err := engine.ContainerList(registry.Context(), listOpts)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	listContainers = append(listContainers, containers...)

	// Add containers from the external storage into complete list
	if ok, _ := cmd.Flags().GetBool("external"); ok {
		externalContainers, err := engine.ContainerListExternal(registry.Context())
		if err != nil {
			cobra.CompErrorln(err.Error())
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		listContainers = append(listContainers, externalContainers...)
	}

	for _, c := range listContainers {
		// include ids in suggestions if cType == completeIDs or
		// more then 2 chars are typed and cType == completeDefault
		if ((len(toComplete) > 1 && cType == completeDefault) ||
			cType == completeIDs) && strings.HasPrefix(c.ID, toComplete) {
			suggestions = append(suggestions, c.ID[0:12]+"\t"+c.PodName)
		}
		// include name in suggestions
		if cType != completeIDs && strings.HasPrefix(c.Names[0], toComplete) {
			suggestions = append(suggestions, c.Names[0]+"\t"+c.PodName)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getPods(cmd *cobra.Command, toComplete string, cType completeType, statuses ...string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	listOpts := entities.PodPSOptions{
		Filters: make(map[string][]string),
	}
	if len(statuses) > 0 {
		listOpts.Filters["status"] = statuses
	}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	pods, err := engine.PodPs(registry.Context(), listOpts)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, pod := range pods {
		// include ids in suggestions if cType == completeIDs or
		// more then 2 chars are typed and cType == completeDefault
		if ((len(toComplete) > 1 && cType == completeDefault) ||
			cType == completeIDs) && strings.HasPrefix(pod.Id, toComplete) {
			suggestions = append(suggestions, pod.Id[0:12])
		}
		// include name in suggestions
		if cType != completeIDs && strings.HasPrefix(pod.Name, toComplete) {
			suggestions = append(suggestions, pod.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getQuadlets(cmd *cobra.Command, toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	lsOpts := entities.QuadletListOptions{}
	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	quadlets, err := engine.QuadletList(registry.Context(), lsOpts)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, q := range quadlets {
		if strings.HasPrefix(q.Name, toComplete) {
			suggestions = append(suggestions, q.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getVolumes(cmd *cobra.Command, toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	lsOpts := entities.VolumeListOptions{}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	volumes, err := engine.VolumeList(registry.Context(), lsOpts)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, v := range volumes {
		if strings.HasPrefix(v.Name, toComplete) {
			suggestions = append(suggestions, v.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getImages(cmd *cobra.Command, toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	listOptions := entities.ImageListOptions{}

	engine, err := setupImageEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	images, err := engine.List(registry.Context(), listOptions)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, image := range images {
		// include ids in suggestions if more then 2 chars are typed
		if len(toComplete) > 1 && strings.HasPrefix(image.ID, toComplete) {
			suggestions = append(suggestions, image.ID[0:12])
		}
		for _, repo := range image.RepoTags {
			if toComplete == "" {
				// suggest only full repo path if no input is given
				if strings.HasPrefix(repo, toComplete) {
					suggestions = append(suggestions, repo)
				}
			} else {
				// suggested "registry.fedoraproject.org/f29/httpd:latest" as
				// - "registry.fedoraproject.org/f29/httpd:latest"
				// - "f29/httpd:latest"
				// - "httpd:latest"
				paths := strings.Split(repo, "/")
				for i := range paths {
					suggestionWithTag := strings.Join(paths[i:], "/")
					if strings.HasPrefix(suggestionWithTag, toComplete) {
						suggestions = append(suggestions, suggestionWithTag)
					}
				}
			}
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getManifestListMembers(cmd *cobra.Command, list, toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	inspectOptions := entities.ManifestInspectOptions{}

	engine, err := setupImageEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	listData, err := engine.ManifestInspect(registry.Context(), list, inspectOptions)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, item := range listData.Manifests {
		if strings.HasPrefix(item.Digest.String(), toComplete) {
			suggestions = append(suggestions, item.Digest.String())
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getSecrets(cmd *cobra.Command, toComplete string, cType completeType) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	secrets, err := engine.SecretList(registry.Context(), entities.SecretListRequest{})
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, s := range secrets {
		// works the same as in getNetworks
		if ((len(toComplete) > 1 && cType == completeDefault) ||
			cType == completeIDs) && strings.HasPrefix(s.ID, toComplete) {
			suggestions = append(suggestions, s.ID[0:12])
		}
		// include name in suggestions
		if cType != completeIDs && strings.HasPrefix(s.Spec.Name, toComplete) {
			suggestions = append(suggestions, s.Spec.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getRegistries() ([]string, cobra.ShellCompDirective) {
	regs, err := sysregistriesv2.UnqualifiedSearchRegistries(nil)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return regs, cobra.ShellCompDirectiveNoFileComp
}

func getNetworks(cmd *cobra.Command, toComplete string, cType completeType) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	networkListOptions := entities.NetworkListOptions{}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	networks, err := engine.NetworkList(registry.Context(), networkListOptions)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, n := range networks {
		// include ids in suggestions if cType == completeIDs or
		// more then 2 chars are typed and cType == completeDefault
		if ((len(toComplete) > 1 && cType == completeDefault) ||
			cType == completeIDs) && strings.HasPrefix(n.ID, toComplete) {
			suggestions = append(suggestions, n.ID[0:12])
		}
		// include name in suggestions
		if cType != completeIDs && strings.HasPrefix(n.Name, toComplete) {
			suggestions = append(suggestions, n.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getArtifacts(cmd *cobra.Command, toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	listOptions := entities.ArtifactListOptions{}

	engine, err := setupImageEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	artifacts, err := engine.ArtifactList(registry.Context(), listOptions)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	for _, artifact := range artifacts {
		if strings.HasPrefix(artifact.Name, toComplete) {
			suggestions = append(suggestions, artifact.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getCommands(cmd *cobra.Command, toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	lsOpts := entities.ContainerListOptions{}

	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	containers, err := engine.ContainerList(registry.Context(), lsOpts)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	externalContainers, err := engine.ContainerListExternal(registry.Context())
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers = append(containers, externalContainers...)

	for _, container := range containers {
		// taking of the first element of commands list is done intentionally
		// to exclude command arguments from suggestions (e.g. exclude arguments "-g daemon"
		// from "nginx -g daemon" output)
		if len(container.Command) > 0 {
			if strings.HasPrefix(container.Command[0], toComplete) {
				suggestions = append(suggestions, container.Command[0])
			}
		}
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func fdIsNotDir(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		cobra.CompErrorln(err.Error())
		return true
	}
	return !stat.IsDir()
}

func getPathCompletion(root string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if toComplete == "" {
		toComplete = "/"
	}
	// Important: securejoin is required to make sure we never leave the root mount point
	userpath, err := securejoin.SecureJoin(root, toComplete)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveDefault
	}
	var base string
	f, err := os.Open(userpath)
	// when error or file is not dir get the parent path to stat
	if err != nil || fdIsNotDir(f) {
		// Do not use path.Dir() since this cleans the paths which
		// then no longer matches the user input.
		userpath, base = path.Split(userpath)
		toComplete, _ = path.Split(toComplete)
		f, err = os.Open(userpath)
		if err != nil {
			return nil, cobra.ShellCompDirectiveDefault
		}
	}

	if fdIsNotDir(f) {
		// nothing to complete since it is no dir
		return nil, cobra.ShellCompDirectiveDefault
	}

	entries, err := f.ReadDir(-1)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveDefault
	}
	if len(entries) == 0 {
		// path is empty dir, just add the trailing slash and no space
		if !strings.HasSuffix(toComplete, "/") {
			toComplete += "/"
		}
		return []string{toComplete}, cobra.ShellCompDirectiveDefault | cobra.ShellCompDirectiveNoSpace
	}
	completions := make([]string, 0, len(entries))
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base) {
			suf := ""
			// When the entry is a directory we add the "/" as suffix and do not want to add space
			// to match normal shell completion behavior.
			// Just inc counter again to fake more than one entry in this case and thus get no space.
			if e.IsDir() {
				suf = "/"
				count++
			}
			completions = append(completions, simplePathJoinUnix(toComplete, e.Name()+suf))
			count++
		}
	}
	directive := cobra.ShellCompDirectiveDefault
	if count > 1 {
		// when we have more than one match we do not want to add a space after the completion
		directive |= cobra.ShellCompDirectiveNoSpace
	}
	return completions, directive
}

// simplePathJoinUnix joins to path components by adding a slash only if p1 doesn't end with one.
// We cannot use path.Join() for the completions logic because this one always calls Clean() on
// the path which changes it from the input.
func simplePathJoinUnix(p1, p2 string) string {
	if len(p1) == 0 {
		// Special case if p1 is not set just return p2 as is
		// and do not add a slash as the input didn't contain one either.
		return p2
	}
	if p1[len(p1)-1] == '/' {
		return p1 + p2
	}
	return p1 + "/" + p2
}

// ValidCurrentCmdLine validates the current cmd line
// It utilizes the Args function from the cmd struct
// In most cases the Args function validates the args length but it
// is also used to verify that --latest is not given with an argument.
// This function helps to makes sure we only complete valid arguments.
func ValidCurrentCmdLine(cmd *cobra.Command, args []string, toComplete string) bool {
	if cmd.Args == nil {
		// Without an Args function we cannot check so assume it's correct
		return true
	}
	// We have to append toComplete to the args otherwise the
	// argument count would not match the expected behavior
	if err := cmd.Args(cmd, append(args, toComplete)); err != nil {
		// Special case if we use ExactArgs(2) or MinimumNArgs(2),
		// They will error if we try to complete the first arg.
		// Let's try to parse the common error and compare if we have less args than
		// required. In this case we are fine and should provide completion.

		// Clean the err msg so we can parse it with fmt.Sscanf
		// Trim MinimumNArgs prefix
		cleanErr := strings.TrimPrefix(err.Error(), "requires at least ")
		// Trim MinimumNArgs "only" part
		cleanErr = strings.ReplaceAll(cleanErr, "only received", "received")
		// Trim ExactArgs prefix
		cleanErr = strings.TrimPrefix(cleanErr, "accepts ")
		var need, got int
		cobra.CompDebugln(cleanErr, true)
		_, err = fmt.Sscanf(cleanErr, "%d arg(s), received %d", &need, &got)
		if err == nil {
			if need >= got {
				// We still need more arguments so provide more completions
				return true
			}
		}
		return false
	}
	return true
}

func prefixSlice(pre string, slice []string) []string {
	for i := range slice {
		slice[i] = pre + slice[i]
	}
	return slice
}

func suffixCompSlice(suf string, slice []string) []string {
	for i := range slice {
		key, val, hasVal := strings.Cut(slice[i], "\t")
		if hasVal {
			slice[i] = key + suf + "\t" + val
		} else {
			slice[i] += suf
		}
	}
	return slice
}

func completeKeyValues(toComplete string, k keyValueCompletion) ([]string, cobra.ShellCompDirective) {
	suggestions := make([]string, 0, len(k))
	directive := cobra.ShellCompDirectiveNoFileComp
	for key, getComps := range k {
		if strings.HasPrefix(toComplete, key) {
			if getComps != nil {
				suggestions, dir := getComps(toComplete[len(key):])
				return prefixSlice(key, suggestions), dir
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if strings.HasPrefix(key, toComplete) {
			suggestions = append(suggestions, key)
			latKey := key[len(key)-1:]
			if latKey == "=" || latKey == ":" {
				// make sure we don't add a space after ':' or '='
				directive = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
			}
		}
	}
	return suggestions, directive
}

func getBoolCompletion(_ string) ([]string, cobra.ShellCompDirective) {
	return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
}

/* Autocomplete Functions for cobra ValidArgsFunction */

func AutocompleteArtifacts(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getArtifacts(cmd, toComplete)
}

func AutocompleteArtifactAdd(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		// first argument accepts the name reference
		return getArtifacts(cmd, toComplete)
	}
	return nil, cobra.ShellCompDirectiveDefault
}

// AutocompleteContainers - Autocomplete all container names.
func AutocompleteContainers(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault)
}

// AutocompleteContainersCreated - Autocomplete only created container names.
func AutocompleteContainersCreated(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "created")
}

// AutocompleteContainersExited - Autocomplete only exited container names.
func AutocompleteContainersExited(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "exited")
}

// AutocompleteContainersPaused - Autocomplete only paused container names.
func AutocompleteContainersPaused(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "paused")
}

// AutocompleteContainersRunning - Autocomplete only running container names.
func AutocompleteContainersRunning(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "running")
}

// AutocompleteContainersStartable - Autocomplete only created and exited container names.
func AutocompleteContainersStartable(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getContainers(cmd, toComplete, completeDefault, "created", "exited")
}

// AutocompletePods - Autocomplete all pod names.
func AutocompletePods(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getPods(cmd, toComplete, completeDefault)
}

// AutocompletePodsRunning - Autocomplete only running pod names.
// It considers degraded as running.
func AutocompletePodsRunning(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getPods(cmd, toComplete, completeDefault, "running", "degraded")
}

// AutoCompletePodsPause - Autocomplete only paused pod names
// When a pod has a few containers paused, that ends up in degraded state
// So autocomplete degraded pod names as well
func AutoCompletePodsPause(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getPods(cmd, toComplete, completeDefault, "paused", "degraded")
}

// AutocompleteForKube - Autocomplete all Podman objects supported by kube generate.
func AutocompleteForKube(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, _ := getContainers(cmd, toComplete, completeDefault)
	pods, _ := getPods(cmd, toComplete, completeDefault)
	volumes, _ := getVolumes(cmd, toComplete)
	objs := containers
	objs = append(objs, pods...)
	objs = append(objs, volumes...)
	return objs, cobra.ShellCompDirectiveNoFileComp
}

func AutocompleteForGenerate(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return AutocompleteForKube(cmd, args, toComplete)
}

// AutocompleteContainersAndPods - Autocomplete container names and pod names.
func AutocompleteContainersAndPods(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, _ := getContainers(cmd, toComplete, completeDefault)
	pods, _ := getPods(cmd, toComplete, completeDefault)
	return append(containers, pods...), cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteContainersAndImages - Autocomplete container names and pod names.
func AutocompleteContainersAndImages(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, _ := getContainers(cmd, toComplete, completeDefault)
	images, _ := getImages(cmd, toComplete)
	return append(containers, images...), cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteVolumes - Autocomplete volumes.
func AutocompleteVolumes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getVolumes(cmd, toComplete)
}

// AutocompleteSecrets - Autocomplete secrets.
func AutocompleteSecrets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getSecrets(cmd, toComplete, completeDefault)
}

func AutocompleteSecretCreate(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 1 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImages - Autocomplete images.
func AutocompleteImages(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getImages(cmd, toComplete)
}

// AutocompleteQuadlets - Autocomplete quadlets.
func AutocompleteQuadlets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getQuadlets(cmd, toComplete)
}

// AutocompleteManifestListAndMember - Autocomplete names of manifest lists and digests of items in them.
func AutocompleteManifestListAndMember(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		return getImages(cmd, toComplete)
	}
	if len(args) == 1 {
		return getManifestListMembers(cmd, args[0], toComplete)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImageSearchFilters - Autocomplete `search --filter`.
func AutocompleteImageSearchFilters(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return libimageDefine.SearchFilters, cobra.ShellCompDirectiveNoFileComp
}

// AutocompletePodExitPolicy - Autocomplete pod exit policy.
func AutocompletePodExitPolicy(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return config.PodExitPolicies, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCreateRun - Autocomplete only the fist argument as image and then do file completion.
func AutocompleteCreateRun(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) < 1 {
		// check if the rootfs flag is set
		// if it is set to true provide directory completion
		rootfs, err := cmd.Flags().GetBool("rootfs")
		if err == nil && rootfs {
			return nil, cobra.ShellCompDirectiveFilterDirs
		}
		return getImages(cmd, toComplete)
	}
	// Mount the image and provide path completion
	engine, err := setupImageEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveDefault
	}

	resp, err := engine.Mount(registry.Context(), []string{args[0]}, entities.ImageMountOptions{})
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveDefault
	}
	defer func() {
		_, err := engine.Unmount(registry.Context(), []string{args[0]}, entities.ImageUnmountOptions{})
		if err != nil {
			cobra.CompErrorln(err.Error())
		}
	}()
	if len(resp) != 1 {
		return nil, cobra.ShellCompDirectiveDefault
	}

	// So this uses ShellCompDirectiveDefault to also still provide normal shell
	// completion in case no path matches. This is useful if someone tries to get
	// completion for paths that are not available in the image, e.g. /proc/...
	return getPathCompletion(resp[0].Path, toComplete)
}

// AutocompleteRegistries - Autocomplete registries.
func AutocompleteRegistries(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getRegistries()
}

// AutocompleteNetworks - Autocomplete networks.
func AutocompleteNetworks(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return getNetworks(cmd, toComplete, completeDefault)
}

// AutocompleteHostsFile - Autocomplete hosts file options.
// -> "image", "none", paths
func AutocompleteHostsFile(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	hostsFileModes := []string{"image", "none"}
	return hostsFileModes, cobra.ShellCompDirectiveDefault
}

// AutocompleteDefaultOneArg - Autocomplete path only for the first argument.
func AutocompleteDefaultOneArg(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCommitCommand - Autocomplete podman commit command args.
func AutocompleteCommitCommand(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		return getContainers(cmd, toComplete, completeDefault)
	}
	if len(args) == 1 {
		return getImages(cmd, toComplete)
	}
	// don't complete more than 2 args
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCpCommand - Autocomplete podman cp command args.
func AutocompleteCpCommand(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) < 2 {
		if i := strings.IndexByte(toComplete, ':'); i > -1 {
			// Looks like the user already set the container.
			// Let's mount it and provide path completion for files in the container.
			engine, err := setupContainerEngine(cmd)
			if err != nil {
				cobra.CompErrorln(err.Error())
				return nil, cobra.ShellCompDirectiveDefault
			}

			resp, err := engine.ContainerMount(registry.Context(), []string{toComplete[:i]}, entities.ContainerMountOptions{})
			if err != nil {
				cobra.CompErrorln(err.Error())
				return nil, cobra.ShellCompDirectiveDefault
			}
			defer func() {
				_, err := engine.ContainerUnmount(registry.Context(), []string{toComplete[:i]}, entities.ContainerUnmountOptions{})
				if err != nil {
					cobra.CompErrorln(err.Error())
				}
			}()
			if len(resp) != 1 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			comps, directive := getPathCompletion(resp[0].Path, toComplete[i+1:])
			return prefixSlice(toComplete[:i+1], comps), directive
		}
		// Suggest containers when they match the input otherwise normal shell completion is used
		containers, _ := getContainers(cmd, toComplete, completeDefault)
		for _, container := range containers {
			if strings.HasPrefix(container, toComplete) {
				return suffixCompSlice(":", containers), cobra.ShellCompDirectiveNoSpace
			}
		}
		// else complete paths on the host
		return nil, cobra.ShellCompDirectiveDefault
	}
	// don't complete more than 2 args
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteExecCommand - Autocomplete podman exec command args.
func AutocompleteExecCommand(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		return getContainers(cmd, toComplete, completeDefault, "running")
	}
	return nil, cobra.ShellCompDirectiveDefault
}

// AutocompleteRunlabelCommand - Autocomplete podman container runlabel command args.
func AutocompleteRunlabelCommand(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		// This is unfortunate because the argument order is label followed by image.
		// If it would be the other way around we could inspect the first arg and get
		// all labels from it to suggest them.
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 1 {
		return getImages(cmd, toComplete)
	}
	return nil, cobra.ShellCompDirectiveDefault
}

// AutocompleteContainerOneArg - Autocomplete containers as fist arg.
func AutocompleteContainerOneArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) == 0 {
		return getContainers(cmd, toComplete, completeDefault)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteNetworkConnectCmd - Autocomplete podman network connect/disconnect command args.
func AutocompleteNetworkConnectCmd(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return getNetworks(cmd, toComplete, completeDefault)
	}
	if len(args) == 1 {
		return getContainers(cmd, toComplete, completeDefault)
	}
	// don't complete more than 2 args
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteTopCmd - Autocomplete podman top/pod top command args.
func AutocompleteTopCmd(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	latest := cmd.Flags().Lookup("latest")
	// only complete containers/pods as first arg if latest is not set
	if len(args) == 0 && (latest == nil || !latest.Changed) {
		if cmd.Parent().Name() == "pod" {
			// need to complete pods since we are using pod top
			return getPods(cmd, toComplete, completeDefault)
		}
		return getContainers(cmd, toComplete, completeDefault)
	}
	descriptors, err := util.GetContainerPidInformationDescriptors()
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return descriptors, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteInspect - Autocomplete podman inspect.
func AutocompleteInspect(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, _ := getContainers(cmd, toComplete, completeDefault)
	images, _ := getImages(cmd, toComplete)
	pods, _ := getPods(cmd, toComplete, completeDefault)
	networks, _ := getNetworks(cmd, toComplete, completeDefault)
	volumes, _ := getVolumes(cmd, toComplete)
	artifact, _ := getArtifacts(cmd, toComplete)

	suggestions := make([]string, 0, len(containers)+len(images)+len(pods)+len(networks)+len(volumes)+len(artifact))
	suggestions = append(suggestions, containers...)
	suggestions = append(suggestions, images...)
	suggestions = append(suggestions, pods...)
	suggestions = append(suggestions, networks...)
	suggestions = append(suggestions, volumes...)
	suggestions = append(suggestions, artifact...)

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func AutoCompleteFarms(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	farms, err := podmanConfig.ContainersConfDefaultsRO.GetAllFarms()
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	suggestions := make([]string, 0, len(farms))
	for _, farm := range farms {
		suggestions = append(suggestions, farm.Name)
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSystemConnections - Autocomplete system connections.
func AutocompleteSystemConnections(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cons, err := podmanConfig.ContainersConfDefaultsRO.GetAllConnections()
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	suggestions := make([]string, 0, len(cons))
	for _, con := range cons {
		// the URI will be show as description in shells like zsh
		suggestions = append(suggestions, con.Name+"\t"+con.URI)
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteScp returns a list of connections, images, or both, depending on the amount of arguments
func AutocompleteScp(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	switch len(args) {
	case 0:
		prefix, imagesToComplete, isImages := strings.Cut(toComplete, "::")
		if isImages {
			imageSuggestions, _ := getImages(cmd, imagesToComplete)
			return prefixSlice(prefix+"::", imageSuggestions), cobra.ShellCompDirectiveNoFileComp
		}
		connectionSuggestions, _ := AutocompleteSystemConnections(cmd, args, toComplete)
		imageSuggestions, _ := getImages(cmd, toComplete)
		totalSuggestions := append(suffixCompSlice("::", connectionSuggestions), imageSuggestions...)
		directive := cobra.ShellCompDirectiveNoFileComp
		// if we have connections do not add a space after the completion
		if len(connectionSuggestions) > 0 {
			directive = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
		}
		return totalSuggestions, directive
	case 1:
		_, imagesToComplete, isImages := strings.Cut(args[0], "::")
		if isImages {
			if len(imagesToComplete) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			imageSuggestions, _ := getImages(cmd, toComplete)
			return imageSuggestions, cobra.ShellCompDirectiveNoFileComp
		}
		connectionSuggestions, _ := AutocompleteSystemConnections(cmd, args, toComplete)
		return suffixCompSlice("::", connectionSuggestions), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

/* -------------- Flags ----------------- */

// AutocompleteDetachKeys - Autocomplete detach-keys options.
// -> "ctrl-"
func AutocompleteDetachKeys(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if strings.HasSuffix(toComplete, ",") {
		return []string{toComplete + "ctrl-"}, cobra.ShellCompDirectiveNoSpace
	}
	return []string{"ctrl-"}, cobra.ShellCompDirectiveNoSpace
}

// AutocompleteChangeInstructions - Autocomplete change instructions options for commit and import.
// -> "CMD", "ENTRYPOINT", "ENV", "EXPOSE", "LABEL", "ONBUILD", "STOPSIGNAL", "USER", "VOLUME", "WORKDIR"
func AutocompleteChangeInstructions(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return ChangeCmds, cobra.ShellCompDirectiveNoSpace
}

// AutocompleteImageFormat - Autocomplete image format options.
// -> "oci", "docker"
func AutocompleteImageFormat(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	ImageFormat := []string{"oci", "docker"}
	return ImageFormat, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteInitCtr - Autocomplete init container type
// -> "once", "always"
func AutocompleteInitCtr(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	InitCtrType := []string{define.AlwaysInitContainer, define.OneShotInitContainer}
	return InitCtrType, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCreateAttach - Autocomplete create --attach options.
// -> "stdin", "stdout", "stderr"
func AutocompleteCreateAttach(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return []string{"stdin", "stdout", "stderr"}, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteNamespace - Autocomplete namespace options.
// -> host,container:[name],ns:[path],private
func AutocompleteNamespace(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"container:": func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"ns:":        func(_ string) ([]string, cobra.ShellCompDirective) { return nil, cobra.ShellCompDirectiveDefault },
		"host":       nil,
		"private":    nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteUserNamespace - Autocomplete namespace options.
// -> same as AutocompleteNamespace with "auto", "keep-id" added
func AutocompleteUserNamespace(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	results, directive := AutocompleteNamespace(cmd, args, toComplete)
	results = append(results, "auto", "keep-id", "nomap")
	return results, directive
}

// AutocompleteCgroupMode - Autocomplete cgroup mode options.
// -> "enabled", "disabled", "no-conmon", "split"
func AutocompleteCgroupMode(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cgroupModes := []string{"enabled", "disabled", "no-conmon", "split"}
	return cgroupModes, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImageVolume - Autocomplete image volume options.
// -> "bind", "tmpfs", "ignore"
func AutocompleteImageVolume(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	imageVolumes := []string{"bind", "tmpfs", "ignore"}
	return imageVolumes, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteLogDriver - Autocomplete log-driver options.
// -> "journald", "none", "k8s-file", "passthrough", "passthrough-tty"
func AutocompleteLogDriver(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	// don't show json-file
	logDrivers := []string{define.JournaldLogging, define.NoLogging, define.KubernetesLogging}
	if !registry.IsRemote() {
		logDrivers = append(logDrivers, define.PassthroughLogging, define.PassthroughTTYLogging)
	}
	return logDrivers, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteLogOpt - Autocomplete log-opt options.
// -> "path=", "tag="
func AutocompleteLogOpt(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	logOptions := []string{"path=", "tag=", "max-size=", "label="}
	if strings.HasPrefix(toComplete, "path=") {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return logOptions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

// AutocompletePullOption - Autocomplete pull options for create and run command.
// -> "always", "missing", "never"
func AutocompletePullOption(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	pullOptions := []string{"always", "missing", "never", "newer"}
	return pullOptions, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteRestartOption - Autocomplete restart options for create and run command.
// -> "always", "no", "on-failure", "unless-stopped"
func AutocompleteRestartOption(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	restartOptions := []string{
		define.RestartPolicyAlways, define.RestartPolicyNo,
		define.RestartPolicyOnFailure, define.RestartPolicyUnlessStopped,
	}
	return restartOptions, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSecurityOption - Autocomplete security options options.
func AutocompleteSecurityOption(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"apparmor=":         nil,
		"no-new-privileges": nil,
		"seccomp=":          func(_ string) ([]string, cobra.ShellCompDirective) { return nil, cobra.ShellCompDirectiveDefault },
		"label=": func(s string) ([]string, cobra.ShellCompDirective) {
			if strings.HasPrefix(s, "d") {
				return []string{"disable"}, cobra.ShellCompDirectiveNoFileComp
			}
			return []string{"user:", "role:", "type:", "level:", "filetype:", "disable"}, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
		},
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteStopSignal - Autocomplete stop signal options.
// Autocompletes signals both lower or uppercase depending on the user input.
func AutocompleteStopSignal(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// convertCase will convert a string to lowercase only if the user input is lowercase
	convertCase := func(s string) string { return s }
	if len(toComplete) > 0 && unicode.IsLower(rune(toComplete[0])) {
		convertCase = strings.ToLower
	}

	prefix := ""
	// if input starts with "SI" we have to add SIG in front
	// since the signal map does not have this prefix but the option
	// allows signals with and without SIG prefix
	if strings.HasPrefix(toComplete, convertCase("SI")) {
		prefix = "SIG"
	}

	stopSignals := make([]string, 0, len(signal.SignalMap))
	for sig := range signal.SignalMap {
		stopSignals = append(stopSignals, convertCase(prefix+sig))
	}
	return stopSignals, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSystemdFlag - Autocomplete systemd flag options.
// -> "true", "false", "always"
func AutocompleteSystemdFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	systemd := []string{"true", "false", "always"}
	return systemd, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteUserFlag - Autocomplete user flag based on the names and groups (includes ids after first char) in /etc/passwd and /etc/group files.
// -> user:group
func AutocompleteUserFlag(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if strings.Contains(toComplete, ":") {
		// It would be nice to read the file in the image
		// but at this point we don't know the image.
		file, err := os.Open("/etc/group")
		if err != nil {
			cobra.CompErrorln(err.Error())
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer file.Close()

		var groups []string
		scanner := bufio.NewScanner(file)
		user, _, _ := strings.Cut(toComplete, ":")
		for scanner.Scan() {
			entries := strings.SplitN(scanner.Text(), ":", 4)
			groups = append(groups, user+":"+entries[0])
			// complete ids after at least one char is given
			if len(user)+1 < len(toComplete) {
				groups = append(groups, user+":"+entries[2])
			}
		}
		if err = scanner.Err(); err != nil {
			cobra.CompErrorln(err.Error())
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return groups, cobra.ShellCompDirectiveNoFileComp
	}

	// It would be nice to read the file in the image
	// but at this point we don't know the image.
	file, err := os.Open("/etc/passwd")
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer file.Close()

	var users []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		entries := strings.SplitN(scanner.Text(), ":", 7)
		users = append(users, entries[0]+":")
		// complete ids after at least one char is given
		if len(toComplete) > 0 {
			users = append(users, entries[2]+":")
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return users, cobra.ShellCompDirectiveNoSpace
}

// AutocompleteMountFlag - Autocomplete mount flag options.
// -> "type=bind,", "type=volume,", "type=tmpfs,"
func AutocompleteMountFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := []string{"type=bind,", "type=volume,", "type=tmpfs,"}
	// TODO: Add support for all different options
	return types, cobra.ShellCompDirectiveNoSpace
}

// AutocompleteVolumeFlag - Autocomplete volume flag options.
// -> volumes and paths
func AutocompleteVolumeFlag(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	volumes, _ := getVolumes(cmd, toComplete)
	directive := cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveDefault
	if strings.Contains(toComplete, ":") {
		// add space after second path
		directive = cobra.ShellCompDirectiveDefault
	}
	return volumes, directive
}

// AutocompleteNetworkFlag - Autocomplete network flag options.
func AutocompleteNetworkFlag(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"container:": func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"ns:": func(_ string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveDefault
		},
		"bridge":  nil,
		"none":    nil,
		"host":    nil,
		"private": nil,
	}

	networks, _ := getNetworks(cmd, toComplete, completeDefault)
	suggestions, dir := completeKeyValues(toComplete, kv)
	return append(networks, suggestions...), dir
}

type formatSuggestion struct {
	fieldname string
	suffix    string
}

func convertFormatSuggestions(suggestions []formatSuggestion) []string {
	completions := make([]string, 0, len(suggestions))
	for _, f := range suggestions {
		completions = append(completions, f.fieldname+f.suffix)
	}
	return completions
}

// AutocompleteFormat - Autocomplete json or a given struct to use for a go template.
// The input can be nil, In this case only json will be autocompleted.
// This function will only work for pointer to structs other types are not supported.
// When "{{." is typed the field and method names of the given struct will be completed.
// This also works recursive for nested structs.
func AutocompleteFormat(o any) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// this function provides shell completion for go templates
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// autocomplete json when nothing or json is typed
		// gocritic complains about the argument order but this is correct in this case
		//nolint:gocritic
		if strings.HasPrefix("json", toComplete) {
			return []string{"json"}, cobra.ShellCompDirectiveNoFileComp
		}

		// special(expensive) flow for "podman inspect"
		if cmd != nil && cmd.Name() == "inspect" && cmd.Parent() == cmd.Root() {
			if len(args) == 0 {
				o = &define.InspectContainerData{}
			} else {
				o = getEntityType(cmd, args, o)
			}
		}

		// no input struct we cannot provide completion return nothing
		if o == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// toComplete could look like this: {{ .Config }} {{ .Field.F
		// 1. split the template variable delimiter
		vars := strings.Split(toComplete, "{{")
		if len(vars) == 1 {
			// no variables return no completion
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		// clean the spaces from the last var
		field := strings.Split(vars[len(vars)-1], " ")
		// split this into it struct field names
		fields := strings.Split(field[len(field)-1], ".")
		f := reflect.ValueOf(o)
		if f.Kind() != reflect.Pointer {
			// We panic here to make sure that all callers pass the value by reference.
			// If someone passes a by value then all podman commands will panic since
			// this function is run at init time.
			panic("AutocompleteFormat: passed value must be a pointer to a struct")
		}
		for i := 1; i < len(fields); i++ {
			// last field get all names to suggest
			if i == len(fields)-1 {
				suggestions := getStructFields(f, fields[i])
				// add the current toComplete value in front so that the shell can complete this correctly
				toCompArr := strings.Split(toComplete, ".")
				toCompArr[len(toCompArr)-1] = ""
				toComplete = strings.Join(toCompArr, ".")
				return prefixSlice(toComplete, convertFormatSuggestions(suggestions)), cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
			}

			// first follow pointer and create element when it is nil
			f = actualReflectValue(f)
			switch f.Kind() {
			case reflect.Struct:
				for j := 0; j < f.NumField(); j++ {
					field := f.Type().Field(j)
					// ok this is a bit weird but when we have an embedded nil struct
					// calling FieldByName on a name which is present on this struct will panic
					// Therefore we have to init them (non nil ptr), https://github.com/containers/podman/issues/14223
					if field.Anonymous && f.Field(j).Type().Kind() == reflect.Pointer {
						f.Field(j).Set(reflect.New(f.Field(j).Type().Elem()))
					}
				}
				// set the next struct field
				f = f.FieldByName(fields[i])
			case reflect.Map:
				rtype := f.Type().Elem()
				if rtype.Kind() == reflect.Pointer {
					rtype = rtype.Elem()
				}
				f = reflect.New(rtype)
			case reflect.Func:
				if f.Type().NumOut() != 1 {
					// unsupported type return nothing
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				f = reflect.New(f.Type().Out(0))
			default:
				// unsupported type return nothing
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func getEntityType(cmd *cobra.Command, args []string, o any) any {
	// container logic
	if containers, _ := getContainers(cmd, args[0], completeDefault); len(containers) > 0 {
		return &define.InspectContainerData{}
	}
	// image logic
	if images, _ := getImages(cmd, args[0]); len(images) > 0 {
		return &inspect.ImageData{}
	}
	// volume logic
	if volumes, _ := getVolumes(cmd, args[0]); len(volumes) > 0 {
		return &define.InspectVolumeData{}
	}
	// pod logic
	if pods, _ := getPods(cmd, args[0], completeDefault); len(pods) > 0 {
		return &entities.PodInspectReport{}
	}
	// network logic
	if networks, _ := getNetworks(cmd, args[0], completeDefault); len(networks) > 0 {
		return &entities.NetworkInspectReport{}
	}
	// artifact logic
	if artifacts, _ := getArtifacts(cmd, args[0]); len(artifacts) > 0 {
		return &entities.ArtifactInspectReport{}
	}
	return o
}

// actualReflectValue takes the value,
// if it is pointer it will dereference it and when it is nil,
// it will create a new value from it
func actualReflectValue(f reflect.Value) reflect.Value {
	// follow the pointer first
	if f.Kind() == reflect.Pointer {
		// if the pointer is nil we create a new value from the elements type
		// this allows us to follow nil pointers and get the actual type
		if f.IsNil() {
			f = reflect.New(f.Type().Elem())
		}
		f = f.Elem()
	}
	return f
}

// getStructFields reads all struct field names and method names and returns them.
func getStructFields(f reflect.Value, prefix string) []formatSuggestion {
	var suggestions []formatSuggestion
	if f.IsValid() {
		suggestions = append(suggestions, getMethodNames(f, prefix)...)
	}

	f = actualReflectValue(f)
	// we only support structs
	if f.Kind() != reflect.Struct {
		return suggestions
	}

	var anonymous []formatSuggestion
	// loop over all field names
	for j := 0; j < f.NumField(); j++ {
		field := f.Type().Field(j)
		// check if struct field is not exported, templates only use exported fields
		// PkgPath is always empty for exported fields
		if field.PkgPath != "" {
			continue
		}
		fname := field.Name
		suffix := "}}"
		kind := field.Type.Kind()
		if kind == reflect.Pointer {
			// make sure to read the actual type when it is a pointer
			kind = field.Type.Elem().Kind()
		}
		// when we have a nested struct do not append braces instead append a dot
		if kind == reflect.Struct || kind == reflect.Map {
			suffix = "."
		}
		// if field is anonymous add the child fields as well
		if field.Anonymous {
			anonymous = append(anonymous, getStructFields(f.Field(j), prefix)...)
		}
		if strings.HasPrefix(fname, prefix) {
			// add field name with suffix
			suggestions = append(suggestions, formatSuggestion{fieldname: fname, suffix: suffix})
		}
	}
outer:
	for _, ano := range anonymous {
		// we should only add anonymous child fields if they are not already present.
		for _, sug := range suggestions {
			if ano.fieldname == sug.fieldname {
				continue outer
			}
		}
		suggestions = append(suggestions, ano)
	}
	return suggestions
}

func getMethodNames(f reflect.Value, prefix string) []formatSuggestion {
	suggestions := make([]formatSuggestion, 0, f.NumMethod())
	for j := 0; j < f.NumMethod(); j++ {
		method := f.Type().Method(j)
		// in a template we can only run functions with one return value
		if method.Func.Type().NumOut() != 1 {
			continue
		}
		// when we have a nested struct do not append braces instead append a dot
		kind := method.Func.Type().Out(0).Kind()
		suffix := "}}"
		if kind == reflect.Struct || kind == reflect.Map {
			suffix = "."
		}
		// From a template user's POV it is not important whether they use a struct field or method.
		// They only notice the difference when the function requires arguments.
		// So let's be nice and let the user know that this method requires arguments via the help text.
		// Note since this is actually a method on a type the first argument is always fix so we should skip it.
		num := method.Func.Type().NumIn() - 1
		if num > 0 {
			// everything after tab will the completion scripts show as help when enabled
			// overwrite the suffix because it expects the args
			suffix = "\tThis is a function and requires " + strconv.Itoa(num) + " argument"
			if num > 1 {
				// add plural s
				suffix += "s"
			}
		}
		fname := method.Name
		if strings.HasPrefix(fname, prefix) {
			// add method name with closing braces
			suggestions = append(suggestions, formatSuggestion{fieldname: fname, suffix: suffix})
		}
	}
	return suggestions
}

// AutocompleteEventFilter - Autocomplete event filter flag options.
// -> "container=", "event=", "image=", "pod=", "volume=", "type=", "artifact="
func AutocompleteEventFilter(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	event := func(_ string) ([]string, cobra.ShellCompDirective) {
		return []string{
			events.Attach.String(), events.AutoUpdate.String(), events.Checkpoint.String(), events.Cleanup.String(),
			events.Commit.String(), events.Create.String(), events.Exec.String(), events.ExecDied.String(),
			events.Exited.String(), events.Export.String(), events.Import.String(), events.Init.String(), events.Kill.String(),
			events.LoadFromArchive.String(), events.Mount.String(), events.NetworkConnect.String(),
			events.NetworkDisconnect.String(), events.Pause.String(), events.Prune.String(), events.Pull.String(),
			events.PullError.String(), events.Push.String(), events.Refresh.String(), events.Remove.String(),
			events.Rename.String(), events.Renumber.String(), events.Restart.String(), events.Restore.String(),
			events.Save.String(), events.Start.String(), events.Stop.String(), events.Sync.String(), events.Tag.String(),
			events.Unmount.String(), events.Unpause.String(), events.Untag.String(), events.Update.String(),
		}, cobra.ShellCompDirectiveNoFileComp
	}
	eventTypes := func(_ string) ([]string, cobra.ShellCompDirective) {
		return []string{
			events.Container.String(), events.Image.String(), events.Network.String(),
			events.Pod.String(), events.System.String(), events.Volume.String(), events.Secret.String(),
			events.Artifact.String(),
		}, cobra.ShellCompDirectiveNoFileComp
	}
	kv := keyValueCompletion{
		"container=": func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"image=":     func(s string) ([]string, cobra.ShellCompDirective) { return getImages(cmd, s) },
		"artifact=":  func(s string) ([]string, cobra.ShellCompDirective) { return getArtifacts(cmd, s) },
		"pod=":       func(s string) ([]string, cobra.ShellCompDirective) { return getPods(cmd, s, completeDefault) },
		"volume=":    func(s string) ([]string, cobra.ShellCompDirective) { return getVolumes(cmd, s) },
		"event=":     event,
		"label=":     nil,
		"type=":      eventTypes,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteSystemdRestartOptions - Autocomplete systemd restart options.
// -> "no", "on-success", "on-failure", "on-abnormal", "on-watchdog", "on-abort", "always"
func AutocompleteSystemdRestartOptions(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return systemdDefine.RestartPolicies, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteTrustType - Autocomplete trust type options.
// -> "signedBy", "accept", "reject"
func AutocompleteTrustType(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := []string{"signedBy", "accept", "reject"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImageSort - Autocomplete images sort options.
// -> "created", "id", "repository", "size", "tag"
func AutocompleteImageSort(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	sortBy := []string{"created", "id", "repository", "size", "tag"}
	return sortBy, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteInspectType - Autocomplete inspect type options.
func AutocompleteInspectType(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := []string{AllType, ArtifactType, ContainerType, ImageType, NetworkType, PodType, VolumeType}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteManifestFormat - Autocomplete manifest format options.
// -> "oci", "v2s2"
func AutocompleteManifestFormat(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := []string{"oci", "v2s2"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteNetworkDriver - Autocomplete network driver option.
func AutocompleteNetworkDriver(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	engine, err := setupContainerEngine(cmd)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	info, err := engine.Info(registry.Context())
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return info.Plugins.Network, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteNetworkIPAMDriver - Autocomplete network ipam driver option.
// -> "bridge", "macvlan"
func AutocompleteNetworkIPAMDriver(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	drivers := []string{types.HostLocalIPAMDriver, types.DHCPIPAMDriver, types.NoneIPAMDriver}
	return drivers, cobra.ShellCompDirectiveNoFileComp
}

// AutocompletePodShareNamespace - Autocomplete pod create --share flag option.
// -> "ipc", "net", "pid", "user", "uts", "cgroup", "none"
func AutocompletePodShareNamespace(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	namespaces := []string{"ipc", "net", "pid", "user", "uts", "cgroup", "none"}
	split := strings.Split(toComplete, ",")
	split[len(split)-1] = ""
	toComplete = strings.Join(split, ",")
	return prefixSlice(toComplete, namespaces), cobra.ShellCompDirectiveNoFileComp
}

// AutocompletePodPsSort - Autocomplete images sort options.
// -> "created", "id", "name", "status", "number"
func AutocompletePodPsSort(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	sortBy := []string{"created", "id", "name", "status", "number"}
	return sortBy, cobra.ShellCompDirectiveNoFileComp
}

// AutocompletePsSort - Autocomplete images sort options.
// -> "command", "created", "id", "image", "names", "runningfor", "size", "status"
func AutocompletePsSort(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	sortBy := []string{"command", "created", "id", "image", "names", "runningfor", "size", "status"}
	return sortBy, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImageSaveFormat - Autocomplete image save format options.
func AutocompleteImageSaveFormat(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return ValidSaveFormats, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteImageScpFormat - Autocomplete image scp format options (oci-archive, docker-archive).
func AutocompleteImageScpFormat(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return ValidScpFormats, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteWaitCondition - Autocomplete wait condition options.
// -> "unknown", "configured", "created", "running", "stopped", "paused", "exited", "removing"
func AutocompleteWaitCondition(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	states := []string{
		"unknown", "configured", "created", "exited",
		"healthy", "initialized", "paused", "removing", "running",
		"stopped", "stopping", "unhealthy",
	}
	return states, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCgroupManager - Autocomplete cgroup manager options.
// -> "cgroupfs", "systemd"
func AutocompleteCgroupManager(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := []string{config.CgroupfsCgroupsManager, config.SystemdCgroupsManager}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteContainersConfModules- Autocomplete containers.conf modules.
func AutocompleteContainersConfModules(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	dirs, err := config.ModuleDirectories()
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	var modules []string
	for _, d := range dirs {
		cleanedD := filepath.Clean(d)
		moduleD := cleanedD + string(os.PathSeparator)
		_ = filepath.WalkDir(d,
			func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				moduleName := strings.TrimPrefix(path, moduleD)

				if toComplete != "" && !strings.HasPrefix(moduleName, toComplete) {
					return nil
				}

				if filepath.Clean(path) == cleanedD || d.IsDir() {
					return nil
				}

				modules = append(modules, moduleName)
				return nil
			})
	}
	return modules, cobra.ShellCompDirectiveDefault
}

// AutocompleteEventBackend - Autocomplete event backend options.
// -> "file", "journald", "none"
func AutocompleteEventBackend(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := []string{events.LogFile.String(), events.Journald.String(), events.Null.String()}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteLogLevel - Autocomplete log level options.
// -> "trace", "debug", "info", "warn", "error", "fatal", "panic"
func AutocompleteLogLevel(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return LogLevels, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSDNotify - Autocomplete sdnotify options.
// -> "container", "conmon", "ignore"
func AutocompleteSDNotify(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := []string{define.SdNotifyModeConmon, define.SdNotifyModeContainer, define.SdNotifyModeHealthy, define.SdNotifyModeIgnore}
	return types, cobra.ShellCompDirectiveNoFileComp
}

var containerStatuses = []string{"created", "running", "paused", "stopped", "exited", "unknown"}

var quadletStatuses = []string{entities.QuadletStatusNotLoaded, entities.QuadletStatusLoadedTemplate, "active/running", "inactive/dead", "failed/failed", "activating/start", "deactivating/stop"}

// AutocompletePsFilters - Autocomplete ps filter options.
func AutocompletePsFilters(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"ancestor=": func(s string) ([]string, cobra.ShellCompDirective) { return getImages(cmd, s) },
		"before=":   func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"command=":  func(s string) ([]string, cobra.ShellCompDirective) { return getCommands(cmd, s) },
		"exited=":   nil,
		"health=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return []string{
				define.HealthCheckHealthy,
				define.HealthCheckUnhealthy,
			}, cobra.ShellCompDirectiveNoFileComp
		},
		"id=":         func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeIDs) },
		"annotation=": nil,
		"label=":      nil,
		"name=":       func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeNames) },
		"network=":    func(s string) ([]string, cobra.ShellCompDirective) { return getNetworks(cmd, s, completeDefault) },
		"pod=":        func(s string) ([]string, cobra.ShellCompDirective) { return getPods(cmd, s, completeDefault) },
		"restart-policy=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return []string{
				define.RestartPolicyAlways,
				define.RestartPolicyNo,
				define.RestartPolicyOnFailure,
				define.RestartPolicyUnlessStopped,
			}, cobra.ShellCompDirectiveNoFileComp
		},
		"should-start-on-boot=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
		},
		"since=": func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeDefault) },
		"status=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return containerStatuses, cobra.ShellCompDirectiveNoFileComp
		},
		"until=":  nil,
		"volume=": func(s string) ([]string, cobra.ShellCompDirective) { return getVolumes(cmd, s) },
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteQuadletFilters - Autocomplete quadlet filter options.
func AutocompleteQuadletFilters(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"name=": func(s string) ([]string, cobra.ShellCompDirective) { return getQuadlets(cmd, s) },
		"pod=":  func(s string) ([]string, cobra.ShellCompDirective) { return getQuadlets(cmd, s) },
		"status=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return quadletStatuses, cobra.ShellCompDirectiveNoFileComp
		},
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompletePodPsFilters - Autocomplete pod ps filter options.
func AutocompletePodPsFilters(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"ctr-ids=":    func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeIDs) },
		"ctr-names=":  func(s string) ([]string, cobra.ShellCompDirective) { return getContainers(cmd, s, completeNames) },
		"ctr-number=": nil,
		"ctr-status=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return containerStatuses, cobra.ShellCompDirectiveNoFileComp
		},
		"id=":      func(s string) ([]string, cobra.ShellCompDirective) { return getPods(cmd, s, completeIDs) },
		"label=":   nil,
		"name=":    func(s string) ([]string, cobra.ShellCompDirective) { return getPods(cmd, s, completeNames) },
		"network=": func(s string) ([]string, cobra.ShellCompDirective) { return getNetworks(cmd, s, completeDefault) },
		"status=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return []string{
				"stopped", "running",
				"paused", "exited", "dead", "created", "degraded",
			}, cobra.ShellCompDirectiveNoFileComp
		},
		"until=": nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteImageFilters - Autocomplete image ls --filter options.
func AutocompleteImageFilters(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	getImg := func(s string) ([]string, cobra.ShellCompDirective) { return getImages(cmd, s) }
	kv := keyValueCompletion{
		"after=":        getImg,
		"before=":       getImg,
		"containers=":   getBoolCompletion,
		"dangling=":     getBoolCompletion,
		"digest=":       nil,
		"id=":           getImg,
		"intermediate=": getBoolCompletion,
		"label=":        nil,
		"manifest=":     getImg,
		"readonly=":     getBoolCompletion,
		"reference=":    nil,
		"since=":        getImg,
		"until=":        nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompletePruneFilters - Autocomplete container/image prune --filter options.
func AutocompletePruneFilters(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"label=": nil,
		"until=": nil,
	}
	if cmd.CommandPath() == "podman container prune" {
		kv["annotation="] = nil
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteNetworkFilters - Autocomplete network ls --filter options.
func AutocompleteNetworkFilters(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"dangling=": getBoolCompletion,
		"driver=": func(_ string) ([]string, cobra.ShellCompDirective) {
			return []string{types.BridgeNetworkDriver, types.MacVLANNetworkDriver, types.IPVLANNetworkDriver}, cobra.ShellCompDirectiveNoFileComp
		},
		"id=":    func(s string) ([]string, cobra.ShellCompDirective) { return getNetworks(cmd, s, completeIDs) },
		"label=": nil,
		"name=":  func(s string) ([]string, cobra.ShellCompDirective) { return getNetworks(cmd, s, completeNames) },
		"until=": nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteNetworkInterfaceNames - Autocomplete network create --interface-name options.
func AutocompleteNetworkInterfaceNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	interfaces, err := net.Interfaces()
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveDefault
	}
	interfaceNames := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		interfaceNames = append(interfaceNames, iface.Name)
	}
	return interfaceNames, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteVolumeFilters - Autocomplete volume ls --filter options.
func AutocompleteVolumeFilters(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	local := func(_ string) ([]string, cobra.ShellCompDirective) {
		return []string{"local"}, cobra.ShellCompDirectiveNoFileComp
	}
	getImg := func(s string) ([]string, cobra.ShellCompDirective) { return getImages(cmd, s) }
	kv := keyValueCompletion{
		"after=":     getImg,
		"anonymous=": getBoolCompletion,
		"dangling=":  getBoolCompletion,
		"driver=":    local,
		"label=":     nil,
		"name=":      func(s string) ([]string, cobra.ShellCompDirective) { return getVolumes(cmd, s) },
		"opt=":       nil,
		"scope=":     local,
		"since=":     getImg,
		"until=":     nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteVolumePruneFilters - Autocomplete volume prune --filter options.
func AutocompleteVolumePruneFilters(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	getImg := func(s string) ([]string, cobra.ShellCompDirective) { return getImages(cmd, s) }
	kv := keyValueCompletion{
		"after=":     getImg,
		"all=":       getBoolCompletion,
		"anonymous=": getBoolCompletion,
		"label=":     nil,
		"since=":     getImg,
		"until=":     nil,
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteSecretFilters - Autocomplete secret ls --filter options.
func AutocompleteSecretFilters(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kv := keyValueCompletion{
		"id=":   func(s string) ([]string, cobra.ShellCompDirective) { return getSecrets(cmd, s, completeIDs) },
		"name=": func(s string) ([]string, cobra.ShellCompDirective) { return getSecrets(cmd, s, completeNames) },
	}
	return completeKeyValues(toComplete, kv)
}

// AutocompleteCheckpointCompressType - Autocomplete checkpoint compress type options.
// -> "gzip", "none", "zstd"
func AutocompleteCheckpointCompressType(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := []string{"gzip", "none", "zstd"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteCompressionFormat - Autocomplete compression-format type options.
func AutocompleteCompressionFormat(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	types := []string{"gzip", "zstd", "zstd:chunked"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteClone - Autocomplete container and image names
func AutocompleteClone(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	switch len(args) {
	case 0:
		if cmd.Parent().Name() == "pod" { // needs to be " pod " to exclude 'podman'
			return getPods(cmd, toComplete, completeDefault)
		}
		return getContainers(cmd, toComplete, completeDefault)
	case 2:
		if cmd.Parent().Name() == "pod" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return getImages(cmd, toComplete)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSSH - Autocomplete ssh modes
func AutocompleteSSH(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !ValidCurrentCmdLine(cmd, args, toComplete) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return []string{string(ssh.GolangMode), string(ssh.NativeMode)}, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteHealthOnFailure - action to take once the container turns unhealthy.
func AutocompleteHealthOnFailure(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return define.SupportedHealthCheckOnFailureActions, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteSysctl - autocomplete list all sysctl names
func AutocompleteSysctl(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var completions []string
	sysPath := "/proc/sys"

	err := filepath.WalkDir(sysPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// /proc/sys is a volatile virtual filesystem whose contents change dynamically.
			// Skip directories on any error (race conditions, permission denied, etc.) to
			// provide partial completion results rather than failing completely.
			// See: https://github.com/containers/podman/issues/27252
			return filepath.SkipDir
		}

		if !d.IsDir() {
			rel, err := filepath.Rel(sysPath, path)
			if err != nil {
				return err
			}
			sysctlName := strings.ReplaceAll(rel, string(os.PathSeparator), ".")

			if strings.HasPrefix(sysctlName, toComplete) {
				completions = append(completions, sysctlName)
			}
		}

		return nil
	})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
