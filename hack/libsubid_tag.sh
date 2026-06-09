#!/usr/bin/env bash
if test $(${GO:-go} env GOOS) != "linux"; then
    exit 0
fi
tmpdir="$PWD/tmp.$RANDOM"
mkdir -p "$tmpdir"
trap 'rm -fr "$tmpdir"' EXIT
cc -o "$tmpdir"/libsubid_tag -x c - -l subid > /dev/null 2> /dev/null << EOF
#include <shadow/subid.h>
#include <stdio.h>
#include <stdlib.h>

const char *Prog = "test";
FILE *shadow_logfd = NULL;

int main() {
	struct subid_range *ranges = NULL;
#if SUBID_ABI_MAJOR >= 4
	subid_get_uid_ranges("root", &ranges);
#else
	get_subuid_ranges("root", &ranges);
#endif
	free(ranges);
	return 0;
}
EOF
if test $? -eq 0; then
    echo libsubid
fi
