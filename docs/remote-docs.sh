#!/usr/bin/env bash
# Assemble remote man pages for darwin or windows from markdown files
set -e

PLATFORM=$1                         ## linux, windows or darwin
TARGET=${2}                         ## where to output files
SOURCES=${@:3}                      ## directories to find markdown files

# This is a *native* binary, one we can run on this host. (This script can be
# invoked in a cross-compilation environment, so even if PLATFORM=windows
# we need an actual executable that we can invoke).
if [[ -z "$PODMAN" ]]; then
    DETECTED_OS=$(env -i HOME="$HOME" PATH="$PATH" GOROOT="$GOROOT" go env GOOS)
    case $DETECTED_OS in
        windows)
            PODMAN=bin/windows/podman.exe ;;
        darwin)
            PODMAN=bin/darwin/podman ;;
        *)  # Assume "linux"
            PODMAN=bin/podman-remote ;;
    esac
fi

function usage() {
    echo >&2 "$0 PLATFORM TARGET SOURCES..."
    echo >&2 "PLATFORM: Is either linux, darwin, windows or freebsd."
    echo >&2 "TARGET: Is the directory where files will be staged. eg, docs/build/remote/linux"
    echo >&2 "SOURCES: Are the directories of source files. eg, docs/source/markdown"
}

function fail() {
    echo >&2 -e "$(dirname $0): $@\n"
    exit 1
}

case $PLATFORM in
darwin|linux|freebsd)
    PUBLISHER=man_fn
    ext=1
    ;;
windows)
    PUBLISHER=html_fn
    ext=1.md
    ;;
-help)
    usage
    exit 0
    ;;
*) fail '"linux", "darwin" and "windows" are the only supported platforms.' ;;
esac

if [[ -z $TARGET ]]; then
    fail 'TARGET directory is required'
fi

if [[ -z $SOURCES ]]; then
    fail 'At least one SOURCE directory is required'
fi

if [[ ! -x $PODMAN ]]; then
    fail "$PODMAN does not exist"
fi

## man_fn copies the man page or link to flattened directory
function man_fn() {
    local page=$1
    local file=$(basename $page)
    local dir=$(dirname $page)

    if [[ ! -f $page ]]; then
        page=$dir/links/${file%.*}.$ext
    fi
    install $page $TARGET/${file%%.*}.1
}

## html_fn converts the markdown page or link to HTML
function html_fn() {
    local markdown=$1
    local file=$(basename $markdown)
    local dir=$(dirname $markdown)

    if [[ ! -f $markdown ]]; then
        local link=$(sed -e 's?.so man1/\(.*\)?\1?' <$dir/links/${file%.md})
        markdown=$dir/$link.md
    fi
    pandoc --ascii --standalone --from markdown-smart \
        --lua-filter=docs/links-to-html.lua \
        --lua-filter=docs/use-pagetitle.lua \
        -o $TARGET/${file%%.*}.html $markdown
}

function html_standalone() {
    local markdown=$1
    local title=$2
    local file=$(basename $markdown)
    local dir=$(dirname $markdown)
    (cd $dir; pandoc --ascii --from markdown-smart -c ../standalone-styling.css \
           --standalone --self-contained --metadata title="$2" -V title= \
           $file)  > $TARGET/${file%%.*}.html
}

# Run 'podman help' (possibly against a subcommand, e.g. 'podman help image')
# and return a list of each first word under 'Available Commands', that is,
# the command name but not its description.
function podman_commands() {
    $PODMAN help "$@" |\
        awk '/^Available Commands:/{ok=1;next}/^Options:/{ok=0}ok { print $1 }' |\
        grep .
}

function podman_all_commands(){
    for cmd in $(podman_commands "$@") ; do
		echo $@ $cmd
        podman_all_commands $@ $cmd
	done
}

## pub_pages finds and publishes the remote manual pages
function pub_pages() {
    local source=$1
    local publisher=$2
    for f in $(ls $source/podman-remote*); do
        $publisher $f
    done


    while IFS= read -r cmd; do
        file="podman-${cmd// /-}"

        # Source dir may have man (.1) files (linux/darwin) or .1.md (windows)
        # but the links subdir will always be .1 (man) files
        if [ -f $source/$file.$ext -o -f $source/links/$file.1 ]; then
            $publisher $(echo $source/$file.$ext)
        else
            # This is worth failing CI for
            fail "no doc file nor link $source/$file.$ext for 'podman $cmd'"
        fi
    done <<< "$(podman_all_commands)"
}

## sed syntax is different on darwin and linux
## sed --help fails on mac, meaning we have to use mac syntax
function sed_os(){
    if sed --help > /dev/null 2>&1 ; then
        $(sed -i "$@")
    else
        $(sed -i "" "$@")
    fi
}

## rename renames podman-remote.ext to podman.ext, and fixes up contents to reflect change
function rename (){
    if [[ "$PLATFORM" != linux ]]; then
        local remote=$(echo $TARGET/podman-remote.*)
        local ext=${remote##*.}
        mv $remote $TARGET/podman.$ext

        sed_os "s/podman\\\*-remote/podman/g" $TARGET/podman.$ext
        sed_os "s/A\ remote\ CLI\ for\ Podman\:\ //g" $TARGET/podman.$ext
        case $PLATFORM in
        darwin|linux)
            sed_os "s/Podman\\\*-remote/Podman\ for\ Mac/g" $TARGET/podman.$ext
        ;;
        windows)
            sed_os "s/Podman\\\*-remote/Podman\ for\ Windows/g" $TARGET/podman.$ext
        ;;
        esac
    fi
}

## walk the SOURCES for markdown sources
mkdir -p $TARGET
for s in $SOURCES; do
    if [[ -d $s ]]; then
        pub_pages $s $PUBLISHER
    else
        echo >&2 "Warning: $s does not exist"
    fi
done
rename
if [[ "$PLATFORM" == "windows" ]]; then
    html_standalone docs/tutorials/podman-for-windows.md 'Podman for Windows'
fi
