package tunnel

import (
	"context"
	"fmt"
	"strings"

	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/bindings/system"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) Events(_ context.Context, opts entities.EventsOptions) error {
	filters := make(map[string][]string)
	if len(opts.Filter) > 0 {
		for _, filter := range opts.Filter {
			split := strings.Split(filter, "=")
			if len(split) < 2 {
				return fmt.Errorf("invalid filter %q", filter)
			}
			filters[split[0]] = append(filters[split[0]], strings.Join(split[1:], "="))
		}
	}
	binChan := make(chan entities.Event)
	go func() {
		for e := range binChan {
			event, err := entities.ConvertToLibpodEvent(e)
			if err != nil {
				opts.EventChan <- events.ReadResult{Error: fmt.Errorf("converting event from server: %w", err)}
				continue
			}
			opts.EventChan <- events.ReadResult{Event: event}
		}
		close(opts.EventChan)
	}()
	options := new(system.EventsOptions).WithFilters(filters).WithSince(opts.Since).WithStream(opts.Stream).WithUntil(opts.Until)
	return system.Events(ic.ClientCtx, binChan, nil, options)
}
