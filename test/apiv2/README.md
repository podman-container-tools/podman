API v2 tests
============

This directory contains tests for the podman version 2 API (HTTP).

Tests themselves are in files of the form 'NN-NAME.at' where NN is a
two-digit number, NAME is a descriptive name, and '.at' is just
an extension I picked.

Running Tests
=============

The main test runner is `test-apiv2`. Usage is:

    $ sudo ./test-apiv2 [NAME [...]]

...where NAME is one or more optional test names, e.g. 'image' or 'pod'
or both. By default, `test-apiv2` will invoke all `*.at` tests.

`test-apiv2` connects to *localhost only* and *via TCP*. There is
no support here for remote hosts or for UNIX sockets. This is a
framework for testing the API, not all possible protocols.

`test-apiv2` will start the service if it isn't already running.


Writing Tests
=============

The main test function is `t`. It runs `curl` against the server,
with POST parameters if present, and compares return status and
(optionally) string results from the server:

    t GET /_ping 200 OK
      ^^^ ^^^^^^ ^^^ ^^
      |   |      |   +--- expected string result
      |   |      +------- expected return code
      |   +-------------- endpoint to access
      +------------------ method (GET, POST, DELETE, HEAD)


    t POST libpod/volumes/create name=foo 201 .ID~[0-9a-f]\\{12\\}
           ^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^ ^^^ ^^^^^^^^^^^^^^^^^^^^
           |                     |        |   JSON '.ID': expect 12-char hex
           |                     |        +-- expected code
           |                     +----------- POST params
           +--------------------------------- note the missing slash

Never, ever, ever, seriously _EVER_ `exit` from a test. Just don't.
That skips cleanup, and leaves the system in a broken state.

Notes:

* If the endpoint has a leading slash (`/_ping`), `t` leaves it unchanged.
If there's no leading slash, `t` prepends `/v1.40`. This is a simple
convenience for simplicity of writing tests.

* When method is POST, the argument(s) after the endpoint may be a series
of POST parameters in the form 'key=value', separated by spaces:
     t POST myentrypoint 200                                 ! no params
     t POST myentrypoint id=$id 200                          ! just one
     t POST myentrypoint id=$id filter='{"foo":"bar"}' 200   ! two, with json
     t POST myentrypoint name=$name badparam='["foo","bar"]' 500  ! etc...
`t` will convert the param list to JSON form for passing to the server.
A numeric status code terminates processing of POST parameters.
** As a special case, when one POST argument is a string ending in `.tar`,
`.yaml`, or `.json`, `t` will invoke `curl` with `--data-binary @PATH` and
set `Content-type` as appropriate. This is useful for `build` endpoints.
(To override `Content-type`, simply pass along an extra string argument
matching `application/*`):
      t POST myentrypoint /mytmpdir/myfile.tar application/foo 400
** Like above, when using PUT, `t` does `--upload-time` instead of
`--data-binary`

* The final arguments are one or more expected string results. If an
argument starts with a dot, `t` will invoke `jq` on the output to
fetch that field, and will compare it to the right-hand side of
the argument. If the separator is `=` (equals), `t` will require
an exact match; if `~` (tilde), `t` will use `expr` to compare.

* If your test expects `curl` to time out:
     APIV2_TEST_EXPECT_TIMEOUT=5 t POST /foo 999
