#!/usr/bin/env bash
${CPP:-${CC:-cc} -E} ${CPPFLAGS} - &> /dev/null << EOF
#include <sqlite3.h>
EOF
if test $? -eq 0; then
    echo libsqlite3
fi
