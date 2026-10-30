# Quadlet Verify Command

## Problem

`podman quadlet list` can report a Quadlet as `Not loaded`, but it does not explain why the Quadlet could not be loaded.

The existing Quadlet generator can provide more detailed errors when processing a Quadlet. For example, an invalid container Quadlet may produce an error such as:

```text
no Image or Rootfs key specified
```

Users currently need to inspect the systemd generator output or manually run the Quadlet generator to discover these errors.

## Current Behavior

`podman quadlet list` parses Quadlet files, determines their corresponding systemd unit names, and checks their systemd state.

However, it does not perform the full Quadlet conversion and validation performed by the Quadlet generator.

The Quadlet generator already supports `--dryrun`, which performs the normal Quadlet conversion without writing generated units. Conversion errors are reported and the command exits with a non-zero status.

## Proposed Solution

Add a new command:

```text
podman quadlet verify
```

The command should validate Quadlet files using the existing Quadlet conversion and validation logic and report useful errors to the user.

For example:

```text
$ podman quadlet verify

broken.container:
  ERROR: no Image or Rootfs key specified

valid.container:
  OK
```

The command should verify all discovered Quadlets and report the result for each file.

The command should return exit status `0` when all Quadlets are valid and a non-zero exit status when one or more Quadlets fail validation.

## Implementation Considerations

The implementation should reuse the existing Quadlet conversion and validation logic rather than duplicating validation rules in the Podman CLI.

One possible approach is to share or refactor the existing generator logic so that both the Quadlet generator and `podman quadlet verify` can use the same validation path.

Another possibility is to invoke the existing Quadlet generator functionality directly.

The preferred approach should be discussed with maintainers to ensure it fits the existing Podman architecture.

## Testing

Tests should cover:

* A valid Quadlet file.
* An invalid Quadlet file.
* Multiple Quadlet files containing both valid and invalid files.
* Appropriate exit status for success and failure.
* Useful error messages for conversion failures.
* Different supported Quadlet types where applicable.

## Alternatives

Users can currently run the Quadlet generator with `--dryrun` to identify conversion errors.

However, this is an implementation-level interface and requires users to know the generator location and invocation details. A dedicated `podman quadlet verify` command would provide a simpler user-facing interface.
