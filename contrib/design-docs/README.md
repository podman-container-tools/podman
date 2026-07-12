## Design Documents

This directory contains design information for Podman features that are being worked on or will be worked on in the future.
All documents in this directory should be based on the [template](./TEMPLATE.md) provided.
It is encouraged, but not required, that major features be preceded by a design document.
This is intended to ensure major design changes are agreed to by maintainers before they are made.
By discussing before implementing, we hope to avoid late-breaking issues discovered during code review that require rewrite of the feature.

Design documents should be posted in pull requests that clearly indicate they are a design document, and should include only the design document, with no other code or other changes.
The pull request should remain open for at least 1 week for comment before it is merged. Maintainers for the component the design document refers to should be pinged on the pull request and encouraged to comment with their opinions.
Once committed, the design is considered to be finalized.
This does not mean changes cannot be made, but given a design has already been agreed to, the bar required to force changes has raised substantially.
Design documents should be removed once the feature they reference is implemented.
Removed documents remain in the Git history if they need to be referenced in the future.
