#!/bin/bash

set -eo pipefail

# Intended to be executed from a github action workflow step.
# Input: File listing space separated failed cron build names and IDs
# Output: $GITHUB_WORKSPACE/artifacts/email_body.txt file

source $(dirname "${BASH_SOURCE[0]}")/lib.sh

_errfmt="Expecting %s value to not be empty"
# ID_NAME_FILEPATH is defined by workflow YAML
# shellcheck disable=SC2154
if [[ -z "$GITHUB_REPOSITORY" ]]; then
    err $(printf "$_errfmt" "\$GITHUB_REPOSITORY")
elif [[ ! -r "$ID_NAME_FILEPATH" ]]; then
    err "Expecting \$ID_NAME_FILEPATH value ($ID_NAME_FILEPATH) to be a readable file"
fi

confirm_gha_environment

# GITHUB_WORKSPACE confirmed by confirm_gha_environment()
# shellcheck disable=SC2154
mkdir -p "$GITHUB_WORKSPACE/artifacts"
(
    echo "Detected one or more Cirrus-CI cron-triggered jobs have failed recently:"
    echo ""

    while read -r BID NAME; do
        echo "Cron build '$NAME' Failed: https://cirrus-ci.com/build/$BID"
    done < "$ID_NAME_FILEPATH"

    echo ""
    # Defined by github-actions
    # shellcheck disable=SC2154
    echo "# Source: ${GITHUB_WORKFLOW} workflow on ${GITHUB_REPOSITORY}."
    # Separate content from sendgrid.com automatic footer.
    echo ""
    echo ""
) > $GITHUB_WORKSPACE/artifacts/email_body.txt
