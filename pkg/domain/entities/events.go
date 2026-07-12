package entities

import (
	"fmt"
	"strconv"
	"time"

	dockerEvents "github.com/moby/moby/api/types/events"
	libpodEvents "go.podman.io/podman/v6/libpod/events"
	types "go.podman.io/podman/v6/pkg/domain/entities/types"
)

type Event = types.Event

// ConvertToLibpodEvent converts an entities event to a libpod one.
func ConvertToLibpodEvent(e Event) (*libpodEvents.Event, error) {
	var exitCode int
	if ec, ok := e.Actor.Attributes["containerExitCode"]; ok {
		var err error
		exitCode, err = strconv.Atoi(ec)
		if err != nil {
			return nil, fmt.Errorf("parsing containerExitCode %q: %w", ec, err)
		}
	}
	status, err := libpodEvents.StringToStatus(string(e.Action))
	if err != nil {
		return nil, err
	}
	t, err := libpodEvents.StringToType(string(e.Type))
	if err != nil {
		return nil, err
	}
	var (
		oomKilled bool
		hasOOM    bool
	)
	if raw, ok := e.Actor.Attributes["oomKilled"]; ok {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			oomKilled = parsed
			hasOOM = true
		}
	}
	image := e.Actor.Attributes["image"]
	name := e.Actor.Attributes["name"]
	network := e.Actor.Attributes["network"]
	podID := e.Actor.Attributes["podId"]
	errorString := e.Actor.Attributes["error"]
	details := e.Actor.Attributes
	delete(details, "image")
	delete(details, "name")
	delete(details, "network")
	delete(details, "podId")
	delete(details, "error")
	delete(details, "containerExitCode")
	delete(details, "oomKilled")
	newEvent := &libpodEvents.Event{
		ContainerExitCode: &exitCode,
		ID:                e.Actor.ID,
		Image:             image,
		Name:              name,
		Network:           network,
		Status:            status,
		Time:              time.Unix(0, e.TimeNano),
		Type:              t,
		HealthStatus:      e.HealthStatus,
		Error:             errorString,
		Details: libpodEvents.Details{
			PodID:      podID,
			Attributes: details,
		},
	}
	if hasOOM {
		newEvent.OOMKilled = &oomKilled
	}
	return newEvent, nil
}

// ConvertToEntitiesEvent converts a libpod event to an entities one.
func ConvertToEntitiesEvent(e libpodEvents.Event) *types.Event {
	attributes := e.Details.Attributes
	if attributes == nil {
		attributes = make(map[string]string)
	}
	attributes["image"] = e.Image
	attributes["name"] = e.Name
	if e.ContainerExitCode != nil {
		attributes["containerExitCode"] = strconv.Itoa(*e.ContainerExitCode)
	}
	if e.OOMKilled != nil {
		attributes["oomKilled"] = strconv.FormatBool(*e.OOMKilled)
	}
	attributes["podId"] = e.PodID
	if e.Network != "" {
		attributes["network"] = e.Network
	}
	if e.Error != "" {
		attributes["error"] = e.Error
	}
	message := dockerEvents.Message{
		Type:   dockerEvents.Type(e.Type.String()),
		Action: dockerEvents.Action(e.Status.String()),
		Actor: dockerEvents.Actor{
			ID:         e.ID,
			Attributes: attributes,
		},
		Scope:    "local",
		Time:     e.Time.Unix(),
		TimeNano: e.Time.UnixNano(),
	}

	return &types.Event{
		Message:      message,
		HealthStatus: e.HealthStatus,
		Status:       e.Status.String(),
		ID:           e.ID,
		From:         e.Image,
	}
}
