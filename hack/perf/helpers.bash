ENGINE_A=${ENGINE_A:-podman}
ENGINE_B=${ENGINE_B:-docker}
RUNS=${RUNS:-100}
NUM_CONTAINERS=${NUM_CONTAINERS:-100}
IMAGE=${IMAGE:-docker.io/library/alpine:latest}

BOLD="$(tput bold)"
RESET="$(tput sgr0)"

function echo_bold() {
    echo "${BOLD}$1${RESET}"
}

function pull_image() {
    echo_bold "... pulling $IMAGE"
    $ENGINE_A pull $IMAGE -q > /dev/null
    $ENGINE_B pull $IMAGE -q > /dev/null
}

function setup() {
    echo_bold "---------------------------------------------------"
    echo_bold "... comparing $ENGINE_A with $ENGINE_B"
    echo_bold "... cleaning up previous containers and images"
    $ENGINE_A system prune -f > /dev/null
    $ENGINE_B system prune -f > /dev/null
    pull_image
    echo ""
}

function create_containers() {
    echo_bold "... creating $NUM_CONTAINERS containers"
    for i in $(eval echo "{0..$NUM_CONTAINERS}"); do
        $ENGINE_A create $IMAGE >> /dev/null
        $ENGINE_B create $IMAGE >> /dev/null
    done
}
