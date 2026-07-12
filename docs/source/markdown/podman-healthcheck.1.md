% podman-healthcheck 1

## NAME
podman\-healthcheck - Manage healthchecks for containers

## SYNOPSIS
**podman healthcheck** *subcommand*

## DESCRIPTION
The **podman healthcheck** suite of commands are used to manage container healthchecks.

Healthchecks are typically sourced from images. Use `podman image inspect` to identify a healthcheck in the image's Config section.

Healthchecks can also be defined when creating a container via `podman run` or `podman create` using:
- `--health-cmd` to set the check command (string form runs via CMD-SHELL; array form uses CMD)
- `--health-interval`, `--health-timeout`, `--health-retries`, `--health-start-period`
- `--no-healthcheck` to disable an image-defined healthcheck
- Startup healthcheck knobs: `--health-startup-cmd`, `--health-startup-interval`, `--health-startup-retries`, `--health-startup-success`, `--health-startup-timeout`

### Startup Healthcheck vs Regular Healthcheck

**Regular healthcheck** runs continuously throughout the container's lifetime to monitor ongoing health. The `--health-start-period` option provides a grace period during container initialization where failures won't mark the container as unhealthy.

**Startup healthcheck** is a separate healthcheck that runs during container startup and transitions to the regular healthcheck once the container has successfully started. It's designed for containers with extended or unpredictable startup times:
- Define it with `--health-startup-cmd` (requires a regular healthcheck to also be set)
- The startup healthcheck cannot be sourced from an image; it can only be set manually
- Once the startup healthcheck succeeds (based on `--health-startup-success` consecutive successes), it stops and the regular healthcheck takes over
- If it fails too many times (`--health-startup-retries`), the container can be restarted based on `--health-on-failure`

**When to use each:**
- Use `--health-start-period` for simple cases where you know roughly how long startup takes
- Use startup healthcheck (`--health-startup-cmd`) when startup time is unpredictable or you need a different check during startup than during normal operation

To debug or inspect healthchecks:
- Use `podman inspect <container>` and view `.Config.Healthcheck` for the effective settings. Other relevant sections are `.State.Healthcheck`, `Config.StartupHealthCheck`, `.Config.HealthcheckOnFailureAction`, `.Config.HealthMaxLogCount`, `.Config.HealthMaxLogSize`, and `.Config.HealthLogDestination`
- Use `podman inspect --format '{{.State.Health.Status}} {{.Config.Healthcheck}}' <container>` to show current health status and healthcheck config
- Trigger on-demand with `podman healthcheck run <container>` and check the exit code (0=success, 1=failure, 125=error)
    - To get more details on why a healthcheck failed, run `podman --log-level debug healthcheck run <container>`
- Ensure the health command exists inside the container and is quoted properly (prefer single quotes for shell pipelines)

To update healthchecks:
- Use `podman update <flag> <container>` to update the healthcheck settings of a container while the container is running
- Example: `podman update --health-max-log-count=10 <container>` to store up to 10 healthcheck results in the log

## SUBCOMMANDS

| Command | Man Page                                          | Description                                                                    |
| ------- | ------------------------------------------------- | ------------------------------------------------------------------------------ |
| run | [podman-healthcheck-run(1)](podman-healthcheck-run.1.md)    | Run a container healthcheck                                              |

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-run(1)](podman-run.1.md)**, **[podman-create(1)](podman-create.1.md)**, **[podman-inspect(1)](podman-inspect.1.md)**, **[podman-update(1)](podman-update.1.md)**

## HISTORY
Feb 2019, Originally compiled by Brent Baude <bbaude@redhat.com>
