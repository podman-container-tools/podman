#!/usr/bin/perl
# Tests for example prompts; no podman binary or generated docs needed.

use v5.20;
use strict;
use warnings;

use File::Temp qw(tempdir);
use FindBin;
use Test::More;

require "$FindBin::Bin/xref-helpmsgs-manpages";

my $workdir = tempdir(CLEANUP => 1);
my @tests = (
    ['rootless', '```', '$ podman ps', '```'],
    ['rootful', '```console', '# podman mount ctr', '```'],
    ['sudo', '```sh', '$ sudo podman ps', '```'],
    ['PowerShell', '```', 'PS> podman system hyperv-prep', '```'],
    ['labelled PowerShell', '```powershell', 'PS> podman system hyperv-prep', '```'],
    ['PowerShell script', '```powershell', 'podman system hyperv-prep', '```'],
    ['Windows command prompt', '```doscon', 'C:\\> podman machine start', '```'],
    ['Windows batch script', '```bat', 'podman machine start', '```'],
    ['shell block after PowerShell', [6], '```powershell', 'podman machine start', '```', '```sh', 'podman ps', '```'],
    ['output', '```', '$ podman machine start', 'podman machine set --rootful', '```'],
    ['continuation', '```', '$ echo hello |', 'podman run -i alpine cat', '```'],
    ['here-document', '```sh', '$ cat <<EOF', 'podman ps', 'EOF', '```'],
    ['script', '```bash', '#!/bin/bash', 'podman ps', '```'],
    ['data', '```text', 'podman ps', '```'],
    ['prose', 'podman ps is a command.'],
    ['different executable', '```', 'podman-compose up', '```'],
    ['outside examples', '## DESCRIPTION', '```', 'podman ps', '```'],
    ['empty block', '```', '', '```'],
    ['missing prompt', [3], '```', 'podman ps', '```'],
    ['sudo missing prompt', [3], '```bash', 'sudo podman ps', '```'],
    ['bare executable', [3], '```console', 'podman', '```'],
    ['bare executable with whitespace', [3], '```', 'podman' . ' ', '```'],
    ['indented command', [3], '  ```shell', '  podman ps', '  ```'],
    ['comment and blank line', [5], '```', '', '### List containers', 'podman ps', '```'],
    ['singular heading', [4], '## EXAMPLE', '```', 'podman ps', '```'],
    ['subheading', [4], '### List containers', '```', 'podman ps', '```'],
    ['tilde fence', [3], '~~~sh', 'podman ps', '~~~'],
    ['longer closing fence', [6], '```', '$ podman ps', '````', '```', 'podman ps', '```'],
    ['multiple blocks', [3, 6], '```', 'podman ps', '```', '```', 'podman info', '```'],
    ['short fence in output', [9], '````', '$ cat file', '```', '## DESCRIPTION',
        'podman ps', '````', '```', 'podman info', '```'],
    ['wrong fence in output', [8], '```', '$ cat file', '~~~', 'podman ps',
        '```', '```', 'podman info', '```'],
    ['fence with trailing text is output', '```', '$ cat file', '```text', 'podman ps', '```'],
    ['heading in data outside examples', '## DESCRIPTION', '```text', '## EXAMPLES',
        '```', '```', 'podman ps', '```'],
);

for my $test (@tests) {
    my ($name, @lines) = @$test;
    my $bad_lines = ref($lines[0]) ? shift @lines : [];
    subtest $name => sub {
        no warnings 'once';
        local $LibPod::CI::XrefHelpmsgsManpages::Markdown_Path = $workdir;
        local %LibPod::CI::XrefHelpmsgsManpages::Man_Seen;
        local $LibPod::CI::XrefHelpmsgsManpages::Errs = 0;
        local $LibPod::CI::XrefHelpmsgsManpages::ME = 'xref-helpmsgs-manpages';

        my $path = "$workdir/podman-example.1.md";
        open my $fh, '>', $path or die "Cannot write $path: $!";
        print {$fh} join("\n", '## EXAMPLES', @lines), "\n";
        close $fh or die "Cannot close $path: $!";

        my @warnings;
        local $SIG{__WARN__} = sub { push @warnings, @_ };
        LibPod::CI::XrefHelpmsgsManpages::podman_man('podman-example');
        is($LibPod::CI::XrefHelpmsgsManpages::Errs, scalar @$bad_lines, 'error count');
        is_deeply(\@warnings, [map {
            "xref-helpmsgs-manpages: $path:$_: example command needs a '\$' (rootless) or '#' (root-only) prompt\n"
        } @$bad_lines], 'diagnostics include the offending line');

        @warnings = ();
        LibPod::CI::XrefHelpmsgsManpages::podman_man('podman-example');
        is_deeply(\@warnings, [], 'aliases do not repeat diagnostics');
    };
}

done_testing;
