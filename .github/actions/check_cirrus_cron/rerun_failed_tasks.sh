#!/bin/bash

set -eo pipefail

# Intended to be executed from a github action workflow step.
# Input: File listing space separated failed cron build names and IDs
# Output: $GITHUB_WORKSPACE/artifacts/email_body.txt file
#
# HOW TO TEST:  This script may be manually tested assuming you have
# access to the github containers-org. Cirrus API key.  With that in-hand,
# this script may be manually run by:
# 1. export SECRET_CIRRUS_API_KEY=<value>
# 2. Find an old podman build that failed on `main` or another **branch**.
#    For example, from https://cirrus-ci.com/github/containers/podman/main
#    (pick an old one from the bottom, since re-running it won't affect anybody)
# 3. Create a temp. file, like /tmp/fail with a single line, of the form:
#    <cirrus build id number> <cirrus-cron name>
# 4. export ID_NAME_FILEPATH=/tmp/fail
# 5. execute this script, and refresh the build in the WebUI, all unsuccessful
#    tasks should change status to running or scheduled.  Note: some later
#    tasks may remain red as they wait for dependencies to run and pass.
# 6. After each run, cleanup with 'rm -rf $GITHUB_WORKSPACE/artifacts'
#    (unless you want to examine them)

source $(dirname "${BASH_SOURCE[0]}")/lib.sh

_errfmt="Expecting %s value to not be empty"
# ID_NAME_FILEPATH is defined by workflow YAML
# shellcheck disable=SC2154
if [[ -z "$SECRET_CIRRUS_API_KEY" ]]; then
    err $(printf "$_errfmt" "\$SECRET_CIRRUS_API_KEY")
elif [[ ! -r "$ID_NAME_FILEPATH" ]]; then  # output from cron_failures.sh
    err $(printf "Expecting %s value to be a readable file" "\$ID_NAME_FILEPATH")
fi

confirm_gha_environment

# GITHUB_WORKSPACE confirmed by confirm_gha_environment()
# shellcheck disable=SC2154
mkdir -p $GITHUB_WORKSPACE/artifacts
# If there are no tasks, don't fail reading the file
truncate -s 0 $GITHUB_WORKSPACE/artifacts/rerun_tids.txt

cat "$ID_NAME_FILEPATH" | \
    while read -r BID NAME; do
        if [[ -z "$NAME" ]]; then
            err $(printf "$_errfmt" "\$NAME")
        elif [[ -z "$BID" ]]; then
            err $(printf "$_errfmt" "\$BID")
        fi

        id_status_q="
            query {
              build(id: \"$BID\") {
                tasks {
                  id,
                  status
                }
              }
            }
        "
        task_id_status=$(gql "$id_status_q" '.data.build.tasks[0]')
        # Expected query result like:
        # {
        #   "data": {
        #     "build": {
        #       "tasks": [
        #         {
        #           "id": "6321184690667520",
        #           "status": "COMPLETED"
        #         },
        #         ...
        msg "::group::Selecting failed/aborted tasks to re-run"
        jq -r -e '.data.build.tasks[] | join(" ")' <<<"$task_id_status" | \
            while read -r TID STATUS; do
                if [[ -z "$TID" ]] || [[ -z "$STATUS" ]]; then
                    # assume empty line and/or end of file
                    msg "Skipping TID '$TID' with status '$STATUS'"
                    continue
                # Failed task dependencies will have 'aborted' status
                elif [[ "$STATUS" == "FAILED" ]] || [[ "$STATUS" == "ABORTED" ]]; then
                    msg "Rerunning build $BID task $TID"
                    # Must send result through a file into rerun_tasks array
                    # because this section is executing in a child-shell
                    echo "$TID" >> $GITHUB_WORKSPACE/artifacts/rerun_tids.txt
                fi
            done
        declare -a rerun_tasks
        mapfile rerun_tasks <$GITHUB_WORKSPACE/artifacts/rerun_tids.txt
        msg "::endgroup::"

        if [[ "${#rerun_tasks[*]}" -eq 0 ]]; then
            msg "No tasks to re-run for build $BID"
            continue;
        fi

        msg "::warning::Rerunning ${#rerun_tasks[*]} tasks for build $BID"
        # Check-value returned if the gql call was successful
        canary=$(uuidgen)
        # Ensure the trailing ',' is stripped from the end (would be invalid JSON)
        # Rely on shell word-splitting in this case.
        # shellcheck disable=SC2048
        task_ids=$(printf '[%s]' $(printf '"%s",' ${rerun_tasks[*]} | head -c -1))
        rerun_m="
            mutation {
              batchReRun(input: {
                clientMutationId: \"$canary\",
                taskIds: $task_ids
                }
              ) {
                clientMutationId
              }
            }
        "
        filter='.data.batchReRun.clientMutationId'
        if [[ ! "$NAME" =~ "testing" ]]; then # see test.sh
            result=$(gql "$rerun_m" "$filter")
            if [[ $(jq -r -e "$filter"<<<"$result") != "$canary" ]]; then
                err "Attempt to re-run tasks for build $BID failed: ${rerun_tasks[*]}"
            fi
        else
            warn "Test-mode: Would have sent GraphQL request: '$rerun_m'"
        fi
    done
