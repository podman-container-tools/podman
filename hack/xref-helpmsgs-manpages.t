# -*- perl -*-
#
# tests for xref-helpmsgs-manpages
#

use v5.20;
use strict;
use warnings;

use Clone                       qw(clone);
use File::Basename;
use File::Path                  qw(make_path);
use File::Temp                  qw(tempdir);
use FindBin;
use Test::More;
use Test::Differences;

plan tests => 10;

require_ok "$FindBin::Bin/xref-helpmsgs-manpages";


my $workdir = tempdir(basename($0) . ".XXXXXXXX", TMPDIR => 1, CLEANUP => 1);

# Copy man pages to tmpdir, so we can fiddle with them
my $doc_path = do {
    no warnings 'once';
    "$LibPod::CI::XrefHelpmsgsManpages::Markdown_Path";
};

make_path "$workdir/$doc_path"
    or die "Internal error: could not make_path $workdir/$doc_path";
system('rsync', '-a', "$doc_path/." => "$workdir/$doc_path/.") == 0
    or die "Internal error: could not rsync $doc_path to $workdir";

chdir $workdir
    or die "Internal error: could not cd $workdir: $!";

my @warnings_seen;
$SIG{__WARN__} = sub {
    my $msg = shift;
    chomp $msg;
    $msg =~ s/^xref-\S+?:\s+//;         # strip "$ME: "
    $msg =~ s!(^|\s+)$doc_path/!$1!g;   # strip "doc/source/markdown"
    $msg =~ s!:\d+:!:NNN:!;             # file line numbers can change
   push @warnings_seen, $msg;
};

# When we get errors (hopefully only when adding new functionality
# to this test!), this format is MUCH easier to read and copy-paste.
unified_diff;

# Helper function for running xref tests.
sub test_xref {
    my $name = shift;
    my $h = shift;
    my $m = shift;
    my $expect_by_help = shift;
    my $expect_by_man  = shift;

    @warnings_seen = ();
    LibPod::CI::XrefHelpmsgsManpages::xref_by_help($h, $m);
    eq_or_diff_text \@warnings_seen, $expect_by_help, "$name: xref_by_help()";

    @warnings_seen = ();
    LibPod::CI::XrefHelpmsgsManpages::xref_by_man($h, $m);
    eq_or_diff_text \@warnings_seen, $expect_by_man, "$name: xref_by_man()";
}

###############################################################################
# BEGIN Baseline tests
#
# Confirm that everything passes in the current tree

my $help = LibPod::CI::XrefHelpmsgsManpages::podman_help();
eq_or_diff_text \@warnings_seen, [], "podman_help() runs cleanly, no warnings";

@warnings_seen = ();
my $man = LibPod::CI::XrefHelpmsgsManpages::podman_man('podman');
eq_or_diff_text \@warnings_seen, [], "podman_man() runs cleanly, no warnings";

# If this doesn't pass, we've got big problems.
test_xref("baseline", $help, $man, [], []);

#use Data::Dump; dd $man; exit 0;

# END   Baseline tests
##########################################################################
# BEGIN fault injection tests on xref_by_man()
#
# These are really simple: only two different warnings.

my $hclone = clone($help);
my $mclone = clone($man);

delete $hclone->{network}{ls}{"--format"};
delete $hclone->{save};
$mclone->{"command-in-man"} = {};
$mclone->{"system"}{"subcommand-in-man"} = {};

# --format field documented in man page but not in autocomplete
delete $hclone->{events}{"--format"}{".HealthStatus"};

test_xref("xref_by_man injection", $hclone, $mclone,
          [],
          [
              "'podman ': 'command-in-man' in podman.1.md, but not in --help",
              "'podman events --format': '.HealthStatus' in podman-events.1.md, but not in command completion",
              "'podman network ls': --format options documented in man page, but not available via autocomplete",
              "'podman ': 'save' in podman.1.md, but not in --help",
              "'podman system': 'subcommand-in-man' in podman-system.1.md, but not in --help",
          ],
      );

# END   fault injection tests on xref_by_man()
###############################################################################
# BEGIN fault injection tests on xref_by_help()
#
# These are much more complicated.

$hclone = clone($help);
$mclone = clone($man);

# --format is not documented in man page
delete $mclone->{"auto-update"}{"--format"};
# --format is documented, but without a table
$mclone->{container}{list}{"--format"} = 1;
# --format is documented, with a table, but entries are wrong
$mclone->{events}{"--format"}{".Attributes"} = 0;
$mclone->{events}{"--format"}{".Image"} = '...';
$mclone->{events}{"--format"}{".Status"} = 1;
$hclone->{events}{"--format"}{".Status"} = '...';
$mclone->{pod}{ps}{"--format"}{".Label"} = 3;
$mclone->{ps}{"--format"}{".Label"} = 0;
# --format is documented, with a table, but one entry missing
delete $mclone->{events}{"--format"}{".Type"};

# -l option is not documented
delete $mclone->{pod}{inspect}{"-l"};

# Command and subcommand in podman --help, but not in man pages
$hclone->{"new-command-in-help"} = {};
$hclone->{"secret"}{"subcommand-in-help"} = {};

# Can happen if podman-partlydocumented exists in --help, and is listed
# in podman.1.md, but does not have its own actual man page.
$hclone->{partlydocumented} = { "something" => 1 };
$mclone->{partlydocumented} = undef;

test_xref("xref_by_help() injection", $hclone, $mclone,
          [
              "'podman auto-update --help' lists '--format', which is not in podman-auto-update.1.md",
              "'podman container list': --format options are available through autocomplete, but are not documented in podman-ps.1.md",
              "'podman events --format {{.Attributes' is a nested structure. Please add '...' to man page.",
              "'podman events --format {{.Image' is a simple value, not a nested structure. Please remove '...' from man page.",
              "'podman events --format {{.Status' is a nested structure, but the man page documents it as a function?!?",
              "'podman events --format <TAB>' lists '.Type', which is not in podman-events.1.md",
              "'podman  --help' lists 'new-command-in-help', which is not in podman.1.md",
              "'podman partlydocumented' is not documented in man pages!",
              "'podman pod inspect --help' lists '-l', which is not in podman-pod-inspect.1.md",
              "'podman pod ps --format {{.Label' is a function that calls for 1 args; the man page lists 3. Please fix the man page.",
              "'podman ps --format {{.Label' is a function that calls for 1 args. Please investigate what those are, then add them to the man page. E.g., '.Label *bool*' or '.Label *path* *bool*'",
              "'podman secret --help' lists 'subcommand-in-help', which is not in podman-secret.1.md",
          ],
          [],
      );

# END   fault injection tests on xref_by_help()
###############################################################################
# BEGIN fault injection tests on podman_man()
#
# This function has a ton of sanity checks. To test them we need to
# perform minor surgery on lots of .md files: reordering lines,
# adding inconsistencies.
#

# Ordered list of the warnings we expect to see
my @expect_warnings;

# Helper function: given a filename and a function, reads filename
# line by line, invoking filter on each line and writing out the
# results.
sub sed {
    my $path   = shift;                 # in: filename (something.md)
    my $action = shift;                 # in: filter function

    # The rest of our arguments are the warnings introduced into this man page
    push @expect_warnings, @_;

    open my $fh_in, '<', "$doc_path/$path"
        or die "Cannot read $doc_path/$path: $!";
    my $tmpfile = "$doc_path/$path.tmp.$$";
    open my $fh_out, '>', $tmpfile
        or die "Cannot create $doc_path/$tmpfile: $!";

    while (my $line = <$fh_in>) {
        # This is what does all the magic
        print { $fh_out } $action->($line);
    }
    close $fh_in;
    close $fh_out
        or die "Error writing $doc_path/$tmpfile: $!";
    rename "$tmpfile" => "$doc_path/$path"
        or die "Could not rename $doc_path/$tmpfile: $!";
}

# Start filtering.

# podman-attach is a deliberate choice here: it also serves as the man page
# for podman-container-attach. Prior to 2023-12-20 we would read the file
# twice, issuing two warnings, which is anti-helpful. Here we confirm that
# the dup-removing code works.
sed('podman-attach.1.md', sub {
        my $line = shift;
        $line =~ s/^(%\s+podman)-(attach\s+1)/$1 $2/;
        $line;
    },

    "podman-attach.1.md:NNN: wrong title line '% podman attach 1'; should be '% podman-attach 1'",
);


# Tests for broken command-line options
# IMPORTANT NOTE: podman-exec precedes podman-container (below),
# because podman-exec.1.md is actually read while podman-container.1.md
# is still processing; so these messages are printed earlier:
#   podman-container.1.md  -> list of subcommands -> exec -> read -exec.1.md
# Sorry for the confusion.
sed('podman-exec.1.md', sub {
        my $line = shift;

        if ($line =~ /^#### \*\*--env\*\*/) {
            $line = $line . "\ndup dup dup\n\n" . $line;
        }
        elsif ($line =~ /^#### \*\*--privileged/) {
            $line = "#### \*\*--put-me-back-in-order\*\*\n\nbogus option\n\n" . $line;
        }
        elsif ($line =~ /^#### \*\*--tty\*\*/) {
            chomp $line;
            $line .= " xyz\n";
        }
        elsif ($line =~ /^#### \*\*--workdir\*\*/) {
            $line = <<"END_FOO";
#### **--workdir**=*dir*, **-w**

blah blah bogus description

#### **--yucca**=*cactus*|*starch*|*both*

blah blah

#### **--zydeco**=*true* | *false*

END_FOO
        }

        return $line;
    },

    "podman-exec.1.md:NNN: flag '--env' is a dup",
    "podman-exec.1.md:NNN: --privileged should precede --put-me-back-in-order",
    "podman-exec.1.md:NNN: could not parse ' xyz' in option description",
    "podman-exec.1.md:NNN: please rewrite as ', **-w**=*dir*'",
    "podman-exec.1.md:NNN: values must be space-separated: '=*cactus*|*starch*|*both*'",
    "podman-exec.1.md:NNN: Do not enumerate true/false for boolean-only options",
);


# Tests for subcommands in a table
sed('podman-container.1.md', sub {
        my $line = shift;

        # "podman container diff": force an out-of-order error
        state $diff;
        if ($line =~ /^\|\s+diff\s+\|/) {
            $diff = $line;
            return '';
        }
        if ($diff) {
            $line .= $diff;
            $diff = undef;
        }

        # "podman init": force a duplicate-command error
        if ($line =~ /^\|\s+init\s+\|/) {
            $line .= $line;
        }

        # "podman container port": force a wrong-man-page error
        if ($line =~ /^\|\s+port\s+\|/) {
            $line =~ s/-port\.1\.md/-top.1.md/;
        }

        return $line;
    },

    "podman-container.1.md:NNN: 'exec' and 'diff' are out of order",
    "podman-container.1.md:NNN: duplicate subcommand 'init'",
    # FIXME: this is not technically correct; it could be the other way around.
    "podman-container.1.md:NNN: 'podman-port' should be 'podman-top' in '[podman-port(1)](podman-top.1.md)'",
);


# Tests for --format specifiers in a table
sed('podman-image-inspect.1.md', sub {
        my $line = shift;

        state $digest;
        if ($line =~ /^\|\s+\.Digest\s+\|/) {
            $digest = $line;
            return '';
        }
        if ($digest) {
            $line .= $digest;
            $digest = undef;
        }

        if ($line =~ /^\|\s+\.ID\s+\|/) {
            $line = $line . $line;
        }

        $line =~ s/^\|\s+\.Parent\s+\|/| .Parent BAD-ARG |/;
        $line =~ s/^\|\s+\.Size\s+\|/| .Size *arg1* arg2 |/;

        return $line;
    },

    "podman-image-inspect.1.md:NNN: format specifier '.Digest' should precede '.GraphDriver'",
    "podman-image-inspect.1.md:NNN: format specifier '.ID' is a dup",
    "podman-image-inspect.1.md:NNN: unknown args 'BAD-ARG' for '.Parent'. Valid args are '...' for nested structs or, for functions, one or more asterisk-wrapped argument names.",
    "podman-image-inspect.1.md:NNN: unknown args '*arg1* arg2' for '.Size'. Valid args are '...' for nested structs or, for functions, one or more asterisk-wrapped argument names.",
);


# Tests for SEE ALSO section
sed('podman-version.1.md', sub {
        my $line = shift;

        if ($line =~ /^## SEE ALSO/) {
            $line .= "**foo**,**bar**"
                . ", **baz**baz**"
                . ", missingstars"
                . ", **[podman-info(1)](podman-cp.1.md)**"
                . ", **[podman-foo(1)](podman-wait.1.md)**"
                . ", **[podman-x](podman-bar.1.md)**"
                . ", **podman-logs(1)**"
                . ", **podman-image-rm(1)**"
                . ", **sdfsdf**"
                . "\n";
        }

        return $line;
    },

    "podman-version.1.md:NNN: please add space after comma: '**foo**,**bar**'",
    "podman-version.1.md:NNN: invalid token 'baz**baz'",
    "podman-version.1.md:NNN: 'missingstars' should be bracketed by '**'",
    "podman-version.1.md:NNN: inconsistent link podman-info(1) -> podman-cp.1.md, expected podman-info.1.md",
    "podman-version.1.md:NNN: invalid link podman-foo(1) -> podman-wait.1.md",
    "podman-version.1.md:NNN: could not parse 'podman-x' as 'manpage(N)'",
    "podman-version.1.md:NNN: 'podman-logs(1)' should be '[podman-logs(1)](podman-logs.1.md)'",
    "podman-version.1.md:NNN: 'podman-image-rm(1)' refers to a command alias; please use the canonical command name instead",
    "podman-version.1.md:NNN: invalid token 'sdfsdf'"
);


# Tests for --filter specifiers
sed('podman-volume-prune.1.md', sub {
        my $line = shift;

        if ($line =~ /^\|\s+driver\s+\|/) {
            $line = "| name! | sdfsdf |\n" . $line;
        }
        if ($line =~ /^\|\s+opt\s+\|/) {
            $line .= $line;
        }

        return $line;
    },

    "podman-volume-prune.1.md:NNN: filter 'name!' only allowed immediately after its positive",
    "podman-volume-prune.1.md:NNN: filter specifier 'opt' is a dup",
);

# DONE with fault injection. Reread man pages and verify warnings.
@warnings_seen = ();
{
    no warnings 'once';
    %LibPod::CI::XrefHelpmsgsManpages::Man_Seen = ();
}
$man = LibPod::CI::XrefHelpmsgsManpages::podman_man('podman');
eq_or_diff_text \@warnings_seen, \@expect_warnings, "podman_man() with faults";

# END   fault injection tests on podman_man()
###############################################################################
# BEGIN fault injection tests on podman_help()
#
# Nope, this is not likely to happen. In order to do this we'd need to:
#
#   * instrument podman and cobra to emit fake output; or
#   * write a podman wrapper that selectively munges output; or
#   * write a dummy podman that generates the right form of (broken) output.
#
# podman_help() has few sanity checks, and those are unlikely, so doing this
# is way more effort than it's worth.
#
# END   fault injection tests on podman_help()
###############################################################################

1;
