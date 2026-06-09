#!/usr/bin/env bash
if pkg-config libapparmor 2> /dev/null; then
    echo apparmor
fi
