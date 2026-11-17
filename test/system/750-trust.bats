#!/usr/bin/env bats   -*- bats -*-
#
# tests for podman image trust
#

load helpers

@test "podman image trust set" {
      skip_if_remote "trust only works locally"
      policypath=$PODMAN_TMPDIR/policy.json
      run_podman 125 image trust set --signature-policy=$policypath --type=bogus default
      is "$output" "Error: invalid choice: bogus.*" "error from --type=bogus"

      run_podman image trust set --signature-policy=$policypath --type=accept default
      run_podman image trust show --signature-policy=$policypath
      is "$output" ".*all  *default  *accept" "default policy should be accept"

      run_podman image trust set --signature-policy=$policypath --type=reject default
      run_podman image trust show --signature-policy=$policypath
      is "$output" ".*all  *default  *reject" "default policy should be reject"

      run_podman image trust set --signature-policy=$policypath --type=reject docker.io
      run_podman image trust show --signature-policy=$policypath
      is "$output" ".*all  *default  *reject" "default policy should still be reject"
      is "$output" ".*repository  *docker.io  *reject" "docker.io should also be reject"

      run_podman image trust show --signature-policy=$policypath --json
      subset=$(jq -r '.[0] | .repo_name, .type' <<<"$output" | fmt)
      is "$subset" "default reject" "--json also shows default"
      subset=$(jq -r '.[1] | .repo_name, .type' <<<"$output" | fmt)
      is "$subset" "docker.io reject" "--json also shows docker.io"

      run_podman image trust set --signature-policy=$policypath --type=accept docker.io
      run_podman image trust show --signature-policy=$policypath --json
      subset=$(jq -r '.[0] | .repo_name, .type' <<<"$output" | fmt)
      is "$subset" "default reject" "--json, default is still reject"
      subset=$(jq -r '.[1] | .repo_name, .type' <<<"$output" | fmt)
      is "$subset" "docker.io accept" "--json, docker.io should now be accept"

      policy="$(< $policypath)"
      run_podman image trust show --signature-policy=$policypath --raw
      is "$output" "$policy" "output should show match content of policy.json"
}

# vim: filetype=sh
