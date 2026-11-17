Tests for docker-compose v2
===========================

This directory contains tests for docker-compose v2 under podman.
docker-compose v1 is no longer supported upstream so we no longer test with it.

Each subdirectory must contain one docker-compose.yml file along with
all necessary infrastructure for it (e.g. Containerfile, any files
to be copied into the container, and so on.

The `test-compose` script will, for each test subdirectory:

* set up a fresh podman root under an empty working directory;
* run a podman server rooted therein;
* cd to the test subdirectory, and run `docker-compose up -d`;
* source `tests.sh`;
* run `docker-compose down`.

As a special case, `setup.sh` and `teardown.sh` in the test directory
will contain commands to be executed prior to `docker-compose up` and
after `docker-compose down` respectively.

tests.sh will probably contain commands of the form

     test_port 12345 = 'hello there'

Where 12345 is the port to curl to; '=' checks equality, '~' uses `expr`
to check substrings; and 'hello there' is a string to look for in
the curl results.

Usage:

    $ sudo test/compose/test-compose [pattern]

By default, all subdirs will be run. If given a pattern, only those
subdirectories matching 'pattern' will be run.

If `$COMPOSE_WAIT` is set, `test-compose` will pause before running
`docker-compose down`. This can be helpful for you to debug failing tests:

    $ env COMPOSE_WAIT=1 sudo --preserve-env=COMPOSE_WAIT test/compose/test-compose

Then, in another window,

    # ls -lt /var/tmp/
    # X=/var/tmp/test-compose.tmp.XXXXXX <--- most recent results of above
    # podman --root $X/root --runroot $X/runroot ps -a
    # podman --root $X/root --runroot $X/runroot logs -l
