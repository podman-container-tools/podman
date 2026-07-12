#!/usr/bin/perl -w

# Don't care if these modules don't exist in CI; only Ed runs this test
use v5.14;
use Test::More;
use Test::Differences;
use File::Basename;
use File::Path          qw(make_path remove_tree);
use File::Temp          qw(tempdir);
use FindBin;

# Simpleminded parser tests. LHS gets glommed together into one long
# github message; RHS (when present) is the expected subset of issue IDs
# that will be parsed from it.
#
# Again, we glom the LHS into one long multiline string. There doesn't
# seem to be much point to testing line-by-line.
my $parser_tests = <<'END_PARSER_TESTS';
Fixes 100
Fixes: 101
Closes 102

Fixes: #103                           | 103
Fix: #104, #105                       | 104
Resolves: #106 closes #107            | 106 107

fix: #108, FIXES: #109, FiXeD: #110   | 108 109 110
Close:   #111 resolved: #112          | 111 112
END_PARSER_TESTS


# Read tests from __END__ section of this script
my @full_tests;
while (my $line = <DATA>) {
    chomp $line;

    if ($line =~ /^==\s+(.*)/) {
        push @full_tests,
            { name => $1, issues => [], files => {}, expect => [] };
    }
    elsif ($line =~ /^\[([\d\s,]+)\]$/) {
        $full_tests[-1]{issues} = [ split /,\s+/, $1 ];
    }

    #                  1     1   23   3 4   4 5  52
    elsif ($line =~ m!^(\!|\+)\s+((\S+):(\d+):(.*))$!) {
        push @{$full_tests[-1]{expect}}, $2 if $1 eq '+';

        $full_tests[-1]{files}{$3}[$4] = $5;
    }
}

plan tests => 1 + 1 + @full_tests;

require_ok "$FindBin::Bin/pr-removes-fixed-skips";

#
# Parser tests. Just run as one test.
#
my $msg = '';
my @parser_expect;
for my $line (split "\n", $parser_tests) {
    if ($line =~ s/\s+\|\s+([\d\s]+)$//) {
        push @parser_expect, split ' ', $1;
    }
    $msg .= $line . "\n";
}

my @parsed = Podman::CI::PrRemovesFixedSkips::fixed_issues($msg);
eq_or_diff \@parsed, \@parser_expect, "parser parses issue IDs";

###############################################################################

#
# Full tests. Create dummy source-code trees and verify that our check runs.
#
my $tmpdir = tempdir(basename($0) . ".XXXXXXXX", TMPDIR => 1, CLEANUP => 1);
chdir $tmpdir
    or die "Cannot cd $tmpdir: $!";
mkdir $_        for qw(cmd libpod pkg test);
for my $t (@full_tests) {
    for my $f (sort keys %{$t->{files}}) {
        my $lineno = 0;
        make_path(dirname($f));
        open my $fh, '>', $f or die;

        my @lines = @{$t->{files}{$f}};
        for my $i (1 .. @lines + 10) {
            my $line = $lines[$i] || "[line $i intentionally left blank]";
            print { $fh } $line, "\n";
        }
        close $fh
            or die;
    }

    # FIXME: run test
    my @actual = Podman::CI::PrRemovesFixedSkips::unremoved_skips(@{$t->{issues}});
    eq_or_diff \@actual, $t->{expect}, $t->{name};

    # clean up
    unlink $_   for sort keys %{$t->{files}};
}

chdir '/';

__END__

== basic test
[12345]
! test/foo/bar/foo.bar:10:   skip "#12345: not a .go file"
+ test/foo/bar/foo.go:17:    skip "#12345: this one should be found"
+ test/zzz/foo.bats:10:   # FIXME: #12345: we detect FIXMEs also

== no substring matches
[123]
! test/system/123-foo.bats:12:    skip "#1234: should not match 123"
! test/system/123-foo.bats:13:    skip "#0123: should not match 123"

== multiple matches
[456, 789]
+ cmd/podman/foo_test.go:10:    Skip("#456 - blah blah")
! cmd/podman/foo_test.go:15:    Skip("#567 - not a match")
+ cmd/podman/foo_test.go:19:    Skip("#789 - match 2nd issue")
+ cmd/podman/zzz_test.go:12:    Skip("#789 - in another file")

== no match on bkp files
[10101]
! pkg/podman/foo_test.go~:10:    Skip("#10101: no match in ~ file")
! pkg/podman/foo_test.go.bkp:10:    Skip("#10101: no match in .bkp file")

== no match if Skip is commented out
[123]
! test/e2e/foo_test.go:10:   // Skip("#123: commented out")
! test/system/012-foo.bats:20:      # skip "#123: commented out"
