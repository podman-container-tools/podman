#!/bin/bash

set -eo pipefail

# This script attempts to confirm functional networking and
# connectivity to essential external servers.  It also verifies
# some basic environmental expectations and shell-script sanity.
# It's intended for use early on in the podman CI system, to help
# prevent wasting time on tests that can't succeed due to some
# outage, failure, or missed expectation.

set -a
source /etc/automation_environment
source $AUTOMATION_LIB_PATH/common_lib.sh
# shellcheck source=contrib/cirrus/lib.sh
source $(dirname $0)/lib.sh
set +a

req_env_vars CI DEST_BRANCH IMAGE_SUFFIX TEST_FLAVOR TEST_ENVIRON \
             PODBIN_NAME PRIV_NAME DISTRO_NV AUTOMATION_LIB_PATH \
             SCRIPT_BASE CIRRUS_WORKING_DIR FEDORA_NAME \
             VM_IMAGE_NAME

# Defined by the CI system
# shellcheck disable=SC2154
cd $CIRRUS_WORKING_DIR

msg "Checking Cirrus YAML"
# Defined by CI config.
# shellcheck disable=SC2154
showrun $SCRIPT_BASE/cirrus_yaml_test.py

# Lookup 'env' dict. string value from key specified as argument from YAML file.
get_env_key() {
    local yaml
    local script

    yaml="$CIRRUS_WORKING_DIR/.github/workflows/scan-secrets.yml"
    script="from yaml import safe_load; print(safe_load(open('$yaml'))['env']['$1'])"
    python -c "$script"
}

# Only need to check CI-stuffs on a single build-task, there's
# generally one latest fedora task so use that one. In some cases
# when we have to drop testing for the prior fedora task we may
# run twice on current fedora but this is not a problem.
# Envars all defined by CI config.
# shellcheck disable=SC2154
if [[ "${DISTRO_NV}" == "$FEDORA_NAME" ]]; then
    # Tests for lib.sh
    showrun ${SCRIPT_BASE}/lib.sh.t

    # Run this during daily cron job to prevent a GraphQL API change/breakage
    # from impacting every PR.  Down-side being if it does fail, a maintainer
    # will need to do some archaeology to find it.
    # Defined by CI system
    # shellcheck disable=SC2154
    if [[ "$CIRRUS_CRON" == "main" ]]; then
      export PREBUILD=1
      showrun bash ${CIRRUS_WORKING_DIR}/.github/actions/check_cirrus_cron/test.sh
    fi
fi
