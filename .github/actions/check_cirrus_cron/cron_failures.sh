#!/bin/bash

set -eo pipefail

# Intended to be executed from a github action workflow step.
# Outputs the Cirrus cron names and IDs of any failed builds

source $(dirname "${BASH_SOURCE[0]}")/lib.sh

_errfmt="Expecting %s value to not be empty"
if [[ -z "$GITHUB_REPOSITORY" ]]; then  # <owner>/<repo>
    err $(printf "$_errfmt" "\$GITHUB_REPOSITORY")
elif [[ -z "$ID_NAME_FILEPATH" ]]; then  # output filepath
    err $(printf "$_errfmt" "\$ID_NAME_FILEPATH")
fi

confirm_gha_environment

mkdir -p ./artifacts
cat > ./artifacts/query_raw.json << "EOF"
query {
  ownerRepository(platform: "github", owner: "@@OWNER@@", name: "@@REPO@@") {
    cronSettings {
      name
      lastInvocationBuild {
        id
        status
      }
    }
  }
}
EOF
# Makes for easier copy/pasting query to/from
# https://cirrus-ci.com/explorer
owner=$(cut -d '/' -f 1 <<<"$GITHUB_REPOSITORY")
repo=$(cut -d '/' -f 2 <<<"$GITHUB_REPOSITORY")
sed -r -e "s/@@OWNER@@/$owner/g" -e "s/@@REPO@@/$repo/g" \
    ./artifacts/query_raw.json > ./artifacts/query.json

if grep -q '@@' ./artifacts/query.json; then
    err "Found unreplaced substitution token in query JSON"
fi

# The query should never ever return an empty-list, unless there are no cirrus-cron
# jobs defined for the repository.  In that case, this monitoring script shouldn't
# be running anyway.
filt_head='.data.ownerRepository.cronSettings'

gql "$(<./artifacts/query.json)" "$filt_head" > ./artifacts/reply.json
# e.x. reply.json
# {
#   "data": {
#     "ownerRepository": {
#       "cronSettings": [
#         {
#           "name": "Keepalive_v2.0",
#           "lastInvocationBuild": {
#             "id": "5776050544181248",
#             "status": "EXECUTING"
#           }
#         },
#         {
#           "name": "Keepalive_v1.9",
#           "lastInvocationBuild": {
#             "id": "5962921081569280",
#             "status": "COMPLETED"
#           }
#         },
#         {
#           "name": "Keepalive_v2.0.5-rhel",
#           "lastInvocationBuild": {
#             "id": "5003065549914112",
#             "status": "FAILED"
#         }
#         ...

# Output format: <build id> <cron-job name>
# Where <cron-job name> may contain multiple words
filt="$filt_head | map(select(.lastInvocationBuild.status==\"FAILED\") | {id:.lastInvocationBuild.id, name:.name} | join(\" \")) | join(\"\n\")"
jq --raw-output "$filt" ./artifacts/reply.json > "$ID_NAME_FILEPATH"

# Print out the file to assist in job debugging
echo "<Failed Build ID> <Cron Name>"
cat "$ID_NAME_FILEPATH"

# Count non-empty lines (in case there are any)
records=$(awk -r -e '/\w+/{print $0}' "$ID_NAME_FILEPATH" | wc -l)
# Set the output of this step.
# Ref: https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#setting-an-output-parameter
# shellcheck disable=SC2154
echo "failures=$records" >> $GITHUB_OUTPUT
echo "Total failed Cirrus-CI cron builds: $records"
