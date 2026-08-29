#!/usr/bin/perl
#
# tests for swagger-check
#
# swagger-check takes a directory to scan, so each test writes a small
# throwaway .go file and checks the exit status.
#
use v5.14;
use strict;
use warnings;

use File::Temp qw(tempdir);
use Test::More;

my $script = $0;
$script =~ s/\.t$//;
die "$0: cannot find $script\n" if ! -e $script;

# Run swagger-check over a directory containing just $content.
# Returns the exit status: 0 = consistent, 1 = mismatch reported.
sub check {
    my $content = shift;

    my $dir = tempdir(CLEANUP => 1);
    open my $fh, '>', "$dir/register_test.go"
        or die "cannot write $dir/register_test.go: $!\n";
    print { $fh } $content;
    close $fh;

    my $out = qx{$script $dir 2>&1};
    return $? >> 8;
}

#
# Routes registered directly on the router. This has always worked; the
# tests are here so a future refactor cannot quietly break it.
#
is(check(<<'END_GO'), 0, 'r.HandleFunc: matching comment passes');
	// swagger:operation GET /libpod/things/json libpod ThingListLibpod
	// ---
	// summary: List things
	r.HandleFunc(VersionedPath("/libpod/things/json"), s.APIHandler(libpod.ListThings)).Methods(http.MethodGet)
END_GO

is(check(<<'END_GO'), 1, 'r.HandleFunc: wrong tag is caught');
	// swagger:operation GET /libpod/things/json compat ThingListLibpod
	// ---
	// summary: List things
	r.HandleFunc(VersionedPath("/libpod/things/json"), s.APIHandler(libpod.ListThings)).Methods(http.MethodGet)
END_GO

#
# Routes registered on a subrouter. The path passed to Handle() is relative
# to the subrouter prefix, so the full endpoint has to be reassembled before
# it can be compared. These were skipped entirely until 2026-08.
#
is(check(<<'END_GO'), 0, 'subrouter: matching comment passes');
	v4 := r.PathPrefix("/v{version:[4-9][0-9A-Za-z.-]*}/libpod/things").Subrouter()
	// swagger:operation GET /libpod/things/{name}/exists libpod ThingExistsLibpod
	// ---
	// summary: Check if a thing exists
	v4.Handle("/{name:.*}/exists", s.APIHandler(libpod.ThingExists)).Methods(http.MethodGet)
END_GO

is(check(<<'END_GO'), 1, 'subrouter: wrong operation is caught');
	v4 := r.PathPrefix("/v{version:[4-9][0-9A-Za-z.-]*}/libpod/things").Subrouter()
	// swagger:operation GET /libpod/things/{name}/exists libpod ThingWrongNameLibpod
	// ---
	// summary: Check if a thing exists
	v4.Handle("/{name:.*}/exists", s.APIHandler(libpod.ThingExists)).Methods(http.MethodGet)
END_GO

is(check(<<'END_GO'), 1, 'subrouter: wrong path is caught');
	v4 := r.PathPrefix("/v{version:[4-9][0-9A-Za-z.-]*}/libpod/things").Subrouter()
	// swagger:operation GET /libpod/somethingelse/{name}/exists libpod ThingExistsLibpod
	// ---
	// summary: Check if a thing exists
	v4.Handle("/{name:.*}/exists", s.APIHandler(libpod.ThingExists)).Methods(http.MethodGet)
END_GO

#
# PUT is a real method in this tree (ManifestModifyLibpod). It used to make
# the script die with "Cannot grok http.MethodPut".
#
is(check(<<'END_GO'), 0, 'PUT is understood');
	v4 := r.PathPrefix("/v{version:[4-9][0-9A-Za-z.-]*}/libpod/things").Subrouter()
	// swagger:operation PUT /libpod/things/{name} libpod ThingInspectLibpod
	// ---
	// summary: Modify a thing
	v4.Handle("/{name:.*}", s.APIHandler(libpod.ThingModify)).Methods(http.MethodPut)
END_GO

#
# A single swagger comment may cover several consecutive registrations, e.g.
# the v3 and v4 spellings of one endpoint. It is correct if it describes any
# one of them.
#
is(check(<<'END_GO'), 0, 'one comment may cover consecutive routes');
	v3 := r.PathPrefix("/v{version:[0-3][0-9A-Za-z.-]*}/libpod/things").Subrouter()
	v4 := r.PathPrefix("/v{version:[4-9][0-9A-Za-z.-]*}/libpod/things").Subrouter()
	// swagger:operation POST /libpod/things/{name} libpod ThingInspectLibpod
	// ---
	// summary: Create a thing
	v3.Handle("/create", s.APIHandler(libpod.ThingCreate)).Methods(http.MethodPost)
	v4.Handle("/{name:.*}", s.APIHandler(libpod.ThingCreate)).Methods(http.MethodPost)
END_GO

is(check(<<'END_GO'), 1, 'a run matching none of the routes is caught');
	v3 := r.PathPrefix("/v{version:[0-3][0-9A-Za-z.-]*}/libpod/things").Subrouter()
	v4 := r.PathPrefix("/v{version:[4-9][0-9A-Za-z.-]*}/libpod/things").Subrouter()
	// swagger:operation POST /libpod/things/{name} libpod ThingCompletelyWrongLibpod
	// ---
	// summary: Create a thing
	v3.Handle("/create", s.APIHandler(libpod.ThingCreate)).Methods(http.MethodPost)
	v4.Handle("/{name:.*}", s.APIHandler(libpod.ThingCreate)).Methods(http.MethodPost)
END_GO

#
# A route with no swagger comment is not an error. Longstanding behaviour;
# pinned here so it is a deliberate decision rather than an accident.
#
is(check(<<'END_GO'), 0, 'a route with no swagger comment is allowed');
	v4 := r.PathPrefix("/v{version:[4-9][0-9A-Za-z.-]*}/libpod/things").Subrouter()
	v4.Handle("/{name:.*}/exists", s.APIHandler(libpod.ThingExists)).Methods(http.MethodGet)
END_GO

done_testing();
