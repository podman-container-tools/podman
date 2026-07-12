package system

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	eventsDescription = `Monitor podman system events.

  By default, streaming mode is used, printing new events as they occur.  Previous events can be listed via --since and --until.`
	eventsCommand = &cobra.Command{
		Use:               "events [options]",
		Args:              validate.NoArgs,
		Short:             "Show podman system events",
		Long:              eventsDescription,
		RunE:              eventsCmd,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman events
podman events --filter event=create
podman events --format {{.Image}}
podman events --since 1h30s`,
	}

	systemEventsCommand = &cobra.Command{
		Args:              eventsCommand.Args,
		Use:               eventsCommand.Use,
		Short:             eventsCommand.Short,
		Long:              eventsCommand.Long,
		RunE:              eventsCommand.RunE,
		ValidArgsFunction: eventsCommand.ValidArgsFunction,
		Example:           `podman system events`,
	}
)

var (
	eventOptions entities.EventsOptions
	eventFormat  string
	noTrunc      bool
)

type Event struct {
	// containerExitCode is for storing the exit code of a container which can
	// be used for "internal" event notification
	ContainerExitCode *int `json:",omitempty"`
	// OOMKilled indicates whether the container was killed by an OOM condition
	OOMKilled *bool `json:",omitempty"`
	// ID can be for the container, image, volume, etc
	ID string `json:",omitempty"`
	// Image used where applicable
	Image string `json:",omitempty"`
	// Name where applicable
	Name string `json:",omitempty"`
	// Network is the network name in a network event
	Network string `json:"network,omitempty"`
	// Status describes the event that occurred
	Status events.Status
	// Time the event occurred
	Time int64 `json:"time,omitempty"`
	// timeNano the event occurred in nanoseconds
	TimeNano int64 `json:"timeNano,omitempty"`
	// Type of event that occurred
	Type events.Type
	// Health status of the current container
	HealthStatus string `json:"health_status,omitempty"`
	// Error code for certain events involving errors.
	Error string `json:",omitempty"`

	events.Details
}

func newEventFromLibpodEvent(e *events.Event) Event {
	return Event{
		ContainerExitCode: e.ContainerExitCode,
		OOMKilled:         e.OOMKilled,
		ID:                e.ID,
		Image:             e.Image,
		Name:              e.Name,
		Network:           e.Network,
		Status:            e.Status,
		Time:              e.Time.Unix(),
		Type:              e.Type,
		HealthStatus:      e.HealthStatus,
		Details:           e.Details,
		TimeNano:          e.Time.UnixNano(),
		Error:             e.Error,
	}
}

func (e *Event) ToJSONString() (string, error) {
	b, err := json.Marshal(e)
	return string(b), err
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemEventsCommand,
		Parent:  systemCmd,
	})
	eventsFlags(systemEventsCommand)
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: eventsCommand,
	})
	eventsFlags(eventsCommand)
}

func eventsFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	filterFlagName := "filter"
	flags.StringArrayVarP(&eventOptions.Filter, filterFlagName, "f", []string{}, "filter output")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteEventFilter)

	formatFlagName := "format"
	flags.StringVar(&eventFormat, formatFlagName, "", "format the output using a Go template")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&Event{}))

	flags.BoolVar(&eventOptions.Stream, "stream", true, "stream events and do not exit when returning the last known event")

	sinceFlagName := "since"
	flags.StringVar(&eventOptions.Since, sinceFlagName, "", "show all events created since timestamp")
	_ = cmd.RegisterFlagCompletionFunc(sinceFlagName, completion.AutocompleteNone)

	flags.BoolVar(&noTrunc, "no-trunc", true, "do not truncate the output")

	untilFlagName := "until"
	flags.StringVar(&eventOptions.Until, untilFlagName, "", "show all events until timestamp")
	_ = cmd.RegisterFlagCompletionFunc(untilFlagName, completion.AutocompleteNone)
}

func eventsCmd(cmd *cobra.Command, _ []string) error {
	if len(eventOptions.Since) > 0 || len(eventOptions.Until) > 0 {
		eventOptions.FromStart = true
	}
	eventChannel := make(chan events.ReadResult, 1)
	eventOptions.EventChan = eventChannel

	var (
		rpt    *report.Formatter
		doJSON bool
	)

	if cmd.Flags().Changed("format") {
		doJSON = report.IsJSON(eventFormat)
		if !doJSON {
			var err error
			// Use OriginUnknown so it does not add an extra range since it
			// will only be called for each single element and not a slice.
			rpt, err = report.New(os.Stdout, cmd.Name()).Parse(report.OriginUnknown, eventFormat)
			if err != nil {
				return err
			}
		}
	}

	err := registry.ContainerEngine().Events(context.Background(), eventOptions)
	if err != nil {
		return err
	}

	for evt := range eventChannel {
		if evt.Error != nil {
			logrus.Errorf("Failed to read event: %v", evt.Error)
			continue
		}
		switch {
		case doJSON:
			e := newEventFromLibpodEvent(evt.Event)
			jsonStr, err := e.ToJSONString()
			if err != nil {
				return err
			}
			fmt.Println(jsonStr)
		case cmd.Flags().Changed("format"):
			if err := rpt.Execute(newEventFromLibpodEvent(evt.Event)); err != nil {
				return err
			}
		default:
			fmt.Println(evt.Event.ToHumanReadable(!noTrunc))
		}
	}
	return nil
}
