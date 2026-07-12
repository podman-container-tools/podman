####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--source-policy-file**=*pathname*

Specifies the path to a BuildKit-compatible source policy JSON file.  When
specified, source references (e.g., base images in FROM instructions) are
evaluated against the policy rules before being used.

Source policies allow controlling which images can be used as base images and
optionally converting image references (e.g., pinning tags to specific digests)
without modifying Containerfiles.  This is useful for enforcing organizational
policies and ensuring build reproducibility.

The policy file is a JSON document containing an array of rules.  Each rule has:
- **action**: The action to take when the rule matches.  Valid actions are:
  - **ALLOW**: Explicitly allow the source (no transformation).
  - **DENY**: Block the source and fail the build.
  - **CONVERT**: Transform the source to a different reference specified in `updates`.
- **selector**: Specifies which sources the rule applies to.
  - **identifier**: The source identifier to match (e.g., `docker-image://docker.io/library/alpine:latest`).
  - **matchType**: How to match the identifier.  Valid types are `EXACT` and `WILDCARD` (supports `*` and `?` glob patterns).  Defaults to `WILDCARD` if not specified.
- **updates**: For `CONVERT` actions, specifies the replacement identifier.

Rules are evaluated in order; the first matching rule wins.  If no rule matches,
the source is allowed by default.

Note: Source policy CONVERT rules are processed after **--build-context** substitutions
but before any substitutions specified in **containers-registries.conf(5)**.  This provides
multiple ways to override which base image is used for a particular stage, in order of
precedence: `--build-context`, then source policy, then registries.conf.

Example policy file that pins alpine:latest to a specific digest:
```json
{
  "rules": [
    {
      "action": "CONVERT",
      "selector": {
        "identifier": "docker-image://docker.io/library/alpine:latest"
      },
      "updates": {
        "identifier": "docker-image://docker.io/library/alpine@sha256:..."
      }
    }
  ]
}
```

Example policy file that denies all ubuntu images:
```json
{
  "rules": [
    {
      "action": "DENY",
      "selector": {
        "identifier": "docker-image://docker.io/library/ubuntu:*",
        "matchType": "WILDCARD"
      }
    }
  ]
}
```
