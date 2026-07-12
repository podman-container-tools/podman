#!/usr/bin/env bash
if ! [ $(id -u) = 0 ]; then
    echo "Please run as root! '$*' requires root privileges."
    exit 1
fi
