package pods

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	psDescription = "List all pods on system including their names, ids and current state."

	// Command: podman pod _ps_
	psCmd = &cobra.Command{
		Use:               "ps [options]",
		Aliases:           []string{"ls", "list"},
		Short:             "List pods",
		Long:              psDescription,
		RunE:              pods,
		Args:              validate.NoArgs,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	inputFilters []string
	noTrunc      bool
	psInput      entities.PodPSOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: psCmd,
		Parent:  podCmd,
	})
	flags := psCmd.Flags()
	flags.BoolVar(&psInput.CtrNames, "ctr-names", false, "Display the container names")
	flags.BoolVar(&psInput.CtrIds, "ctr-ids", false, "Display the container UUIDs. If no-trunc is not set they will be truncated")
	flags.BoolVar(&psInput.CtrStatus, "ctr-status", false, "Display the container status")

	filterFlagName := "filter"
	flags.StringArrayVarP(&inputFilters, filterFlagName, "f", []string{}, "Filter output based on conditions given")
	_ = psCmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePodPsFilters)

	formatFlagName := "format"
	flags.StringVar(&psInput.Format, formatFlagName, "", "Pretty-print pods to JSON or using a Go template")
	_ = psCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&ListPodReporter{}))

	flags.BoolP("noheading", "n", false, "Do not print headers")
	flags.BoolVar(&psInput.Namespace, "ns", false, "Display namespace information of the pod")
	flags.BoolVar(&noTrunc, "no-trunc", false, "Do not truncate pod and container IDs")
	flags.BoolVarP(&psInput.Quiet, "quiet", "q", false, "Print the numeric IDs of the pods only")

	sortFlagName := "sort"
	flags.StringVar(&psInput.Sort, sortFlagName, "created", "Sort output by created, id, name, or number")
	_ = psCmd.RegisterFlagCompletionFunc(sortFlagName, common.AutocompletePodPsSort)

	validate.AddLatestFlag(psCmd, &psInput.Latest)

	flags.SetNormalizeFunc(utils.AliasFlags)
}

func pods(cmd *cobra.Command, _ []string) error {
	if psInput.Quiet && len(psInput.Format) > 0 {
		return errors.New("quiet and format cannot be used together")
	}
	if cmd.Flag("filter").Changed {
		psInput.Filters = make(map[string][]string)
		for _, f := range inputFilters {
			fname, filter, hasFilter := strings.Cut(f, "=")
			if !hasFilter {
				return fmt.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
			}
			psInput.Filters[fname] = append(psInput.Filters[fname], filter)
		}
	}
	responses, err := registry.ContainerEngine().PodPs(context.Background(), psInput)
	if err != nil {
		return err
	}

	if err := sortPodPsOutput(psInput.Sort, responses); err != nil {
		return err
	}

	switch {
	case report.IsJSON(psInput.Format):
		b, err := json.MarshalIndent(responses, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	case psInput.Quiet:
		for _, p := range responses {
			fmt.Println(p.Id)
		}
		return nil
	}

	lpr := make([]ListPodReporter, 0, len(responses))
	for _, r := range responses {
		lpr = append(lpr, ListPodReporter{r})
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	if cmd.Flags().Changed("format") {
		rpt, err = rpt.Parse(report.OriginUser, psInput.Format)
	} else {
		rpt, err = rpt.Parse(report.OriginPodman, podPsFormat())
	}
	if err != nil {
		return err
	}

	renderHeaders := true
	if noHeading, _ := cmd.Flags().GetBool("noheading"); noHeading {
		renderHeaders = false
	}

	if renderHeaders && rpt.RenderHeaders {
		headers := report.Headers(ListPodReporter{}, map[string]string{
			"Id":                 "POD ID",
			"Name":               "NAME",
			"Status":             "STATUS",
			"Labels":             "LABELS",
			"NumberOfContainers": "# OF CONTAINERS",
			"Created":            "CREATED",
			"InfraID":            "INFRA ID",
			"ContainerIds":       "IDS",
			"ContainerNames":     "NAMES",
			"ContainerStatuses":  "STATUS",
			"Cgroup":             "CGROUP",
			"Namespace":          "NAMESPACES",
			"Restarts":           "RESTARTS",
		})

		if err := rpt.Execute(headers); err != nil {
			return err
		}
	}
	return rpt.Execute(lpr)
}

func podPsFormat() string {
	row := []string{"{{.Id}}", "{{.Name}}", "{{.Status}}", "{{.Created}}", "{{.InfraID}}"}

	if psInput.CtrIds {
		row = append(row, "{{.ContainerIds}}")
	}

	if psInput.CtrNames {
		row = append(row, "{{.ContainerNames}}")
	}

	if psInput.CtrStatus {
		row = append(row, "{{.ContainerStatuses}}")
	}

	if psInput.Namespace {
		row = append(row, "{{.Cgroup}}", "{{.Namespace}}")
	}

	if !psInput.CtrStatus && !psInput.CtrNames && !psInput.CtrIds {
		row = append(row, "{{.NumberOfContainers}}")
	}

	return "{{range . }}" + strings.Join(row, "\t") + "\n" + "{{end -}}"
}

// ListPodReporter is a struct for pod ps output
type ListPodReporter struct {
	*entities.ListPodsReport
}

// Created returns a human readable created time/date
func (l ListPodReporter) Created() string {
	return units.HumanDuration(time.Since(l.ListPodsReport.Created)) + " ago"
}

// Labels returns the pod's labels as a sorted, comma-separated list of
// key=value pairs, matching Docker CLI output format.
func (l ListPodReporter) Labels() string {
	return common.FormatLabels(l.ListPodsReport.Labels)
}

// Label returns a map of the pod's labels
func (l ListPodReporter) Label(name string) string {
	return l.ListPodsReport.Labels[name]
}

// Networks returns the infra container network names in string format
func (l ListPodReporter) Networks() string {
	return strings.Join(l.ListPodsReport.Networks, ",")
}

// NumberOfContainers returns an int representation for
// the number of containers belonging to the pod
func (l ListPodReporter) NumberOfContainers() int {
	return len(l.Containers)
}

// ID is a wrapper to Id for compat, typos
func (l ListPodReporter) ID() string {
	return l.Id()
}

// Id returns the Pod id
func (l ListPodReporter) Id() string {
	if noTrunc {
		return l.ListPodsReport.Id
	}
	return l.ListPodsReport.Id[0:12]
}

// Added for backwards compatibility with podmanv1
func (l ListPodReporter) InfraID() string {
	return l.InfraId()
}

// InfraId returns the infra container id for the pod
// depending on trunc
func (l ListPodReporter) InfraId() string {
	if len(l.ListPodsReport.InfraId) == 0 {
		return ""
	}
	if noTrunc {
		return l.ListPodsReport.InfraId
	}
	return l.ListPodsReport.InfraId[0:12]
}

func (l ListPodReporter) ContainerIds() string {
	ctrids := make([]string, 0, len(l.Containers))
	for _, c := range l.Containers {
		id := c.Id
		if !noTrunc {
			id = id[0:12]
		}
		ctrids = append(ctrids, id)
	}
	return strings.Join(ctrids, ",")
}

func (l ListPodReporter) ContainerNames() string {
	ctrNames := make([]string, 0, len(l.Containers))
	for _, c := range l.Containers {
		ctrNames = append(ctrNames, c.Names)
	}
	return strings.Join(ctrNames, ",")
}

func (l ListPodReporter) ContainerStatuses() string {
	statuses := make([]string, 0, len(l.Containers))
	for _, c := range l.Containers {
		statuses = append(statuses, c.Status)
	}
	return strings.Join(statuses, ",")
}

// Restarts returns the total number of restarts for all the containers in the pod
func (l ListPodReporter) Restarts() string {
	restarts := 0
	for _, c := range l.Containers {
		restarts += int(c.RestartCount)
	}
	return strconv.Itoa(restarts)
}

func sortPodPsOutput(sortBy string, lprs []*entities.ListPodsReport) error {
	switch sortBy {
	case "created":
		sort.Sort(podPsSortedCreated{lprs})
	case "id":
		sort.Sort(podPsSortedID{lprs})
	case "name":
		sort.Sort(podPsSortedName{lprs})
	case "number":
		sort.Sort(podPsSortedNumber{lprs})
	case "status":
		sort.Sort(podPsSortedStatus{lprs})
	default:
		return errors.New("invalid option for --sort, options are: created, id, name, number, or status")
	}
	return nil
}

type lprSort []*entities.ListPodsReport

func (a lprSort) Len() int      { return len(a) }
func (a lprSort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type podPsSortedCreated struct{ lprSort }

func (a podPsSortedCreated) Less(i, j int) bool {
	return a.lprSort[i].Created.After(a.lprSort[j].Created)
}

type podPsSortedID struct{ lprSort }

func (a podPsSortedID) Less(i, j int) bool { return a.lprSort[i].Id < a.lprSort[j].Id }

type podPsSortedNumber struct{ lprSort }

func (a podPsSortedNumber) Less(i, j int) bool {
	return len(a.lprSort[i].Containers) < len(a.lprSort[j].Containers)
}

type podPsSortedName struct{ lprSort }

func (a podPsSortedName) Less(i, j int) bool { return a.lprSort[i].Name < a.lprSort[j].Name }

type podPsSortedStatus struct{ lprSort }

func (a podPsSortedStatus) Less(i, j int) bool {
	return a.lprSort[i].Status < a.lprSort[j].Status
}
