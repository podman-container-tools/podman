#!/usr/bin/perl
#
# tests for logformatter
#
(our $ME = $0) =~ s|^.*/||;

use v5.14;
use strict;
use warnings;

use FindBin;
use File::Temp          qw(tempdir);
use Test::More;

# To test links to source files
$ENV{CIRRUS_CHANGE_IN_REPO} = 'ceci-nest-pas-une-sha';

#
# Read the test cases (see __END__ section below)
#
my @tests;
my $context = '';
while (my $line = <DATA>) {
    chomp $line;

    if ($line =~ /^==\s+(.*)/) {
        push @tests, { name => $1, input => [], expect => [] };
        $context = '';
    }
    elsif ($line =~ /^<<</) {
        $context = 'input';
    }
    elsif ($line =~ /^>>>/) {
        $context = 'expect';
    }
    elsif (@tests && $line) {
        # Handles trailing spaces in log, because we can't have actual ones.
        $line =~ s/\&TRAILINGSPACE;/ /g;
        push @{ $tests[-1]{$context} }, $line;
    }
}

plan tests => scalar(@tests);

my $tempdir = tempdir("logformatter-test.XXXXXX", TMPDIR => 1, CLEANUP => !$ENV{DEBUG});

chdir $tempdir
    or die "$ME: Could not cd $tempdir: $!\n";

for my $t (@tests) {
    my $name = $t->{name};
    (my $fname = $name) =~ s/\s+/_/g;

    open my $fh_out, '>', "$fname.txt"
        or die "$ME: Cannot create $tempdir/$fname.txt: $!\n";
    print { $fh_out } "$_\n" for @{$t->{input}};
    close $fh_out
        or die "$ME: Error writing $tempdir/$fname.txt: $!\n";

    system("$FindBin::Bin/logformatter $fname <$fname.txt >/dev/null");
    open my $fh_in, '<', "$fname.log.html"
        or die "$ME: Fatal: $fname: logformatter did not create .log.html\n";
    my @actual;
    while (my $line = <$fh_in>) {
        chomp $line;
        push @actual, $line  if $line =~ / begin processed output / .. $line =~ / end processed output /;
    }
    close $fh_in;

    # Strip off leading and trailing "<pre>"
    shift @actual; pop @actual;

    # For debugging: preserve expected results
    if ($ENV{DEBUG}) {
        open my $fh_out, '>', "$fname.expect";
        print { $fh_out } "$_\n" for @{$t->{expect}};
        close $fh_out;
    }

    is_deeply \@actual, $t->{expect}, $name;
}

chdir '/';



__END__

== simple bats

<<<
1..4
ok 1 hi
ok 2 bye # skip no reason
not ok 3 fail
# (from function `assert' in file ./helpers.bash, line 343,
#  from function `expect_output' in file ./helpers.bash, line 370,
#  in test file ./run.bats, line 786)
# $ /path/to/podman foo -bar
# time="2023-01-05T15:15:20Z" level=debug msg="this is debug"
# time="2023-01-05T15:15:20Z" level=warning msg="this is warning"
# #| FAIL: exit code is 123; expected 321
ok 4 blah
>>>
1..4
<span class='bats-passed'><a name='t--00001'>ok 1 hi</a></span>
<span class='bats-skipped'><a name='t--00002'>ok 2 bye # skip no reason</a></span>
<span class='bats-failed'><a name='t--00003'>not ok 3 fail</a></span>
<span class='bats-log'># (from function `assert&#39; in file ./<a class="codelink" href="https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/system/helpers.bash#L343">helpers.bash, line 343</a>,</span>
<span class='bats-log'>#  from function `expect_output&#39; in file ./<a class="codelink" href="https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/system/helpers.bash#L370">helpers.bash, line 370</a>,</span>
<span class='bats-log'>#  in test file ./<a class="codelink" href="https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/system/run.bats#L786">run.bats, line 786</a>)</span>
<span class='bats-log'># $ <b><span title="/path/to/podman">podman</span> foo -bar</b></span>
<span class='bats-log'># time=<span class='log-debug'>&quot;2023-01-05T15:15:20Z&quot;</span> level=<span class='log-debug'>debug</span> msg=<span class='log-debug'>&quot;this is debug&quot;</span></span>
<span class='bats-log'># time=<span class='log-warning'>&quot;2023-01-05T15:15:20Z&quot;</span> level=<span class='log-warning'>warning</span> msg=<span class='log-warning'>&quot;this is warning&quot;</span></span>
<span class='bats-log-failblock'># #| FAIL: exit code is 123; expected 321</span>
<span class='bats-passed'><a name='t--00004'>ok 4 blah</a></span>
<hr/><span class='bats-summary'>Summary: <span class='bats-passed'>2 Passed</span>, <span class='bats-failed'>1 Failed</span>, <span class='bats-skipped'>1 Skipped</span>. Total tests: 4</span>


== bats with timestamps 2023-05-16
<<<
[+0000s] 1..2
[+0000s] ok 1 this passes
[+0018s] not ok 2 this fails, and includes command timestamps
[+0018s] # (from function `die' in file test/system/helpers.bash, line 564,
[+0018s] #  from function `run_podman' in file test/system/helpers.bash, line 245,
[+0018s] #  in test file test/system/888-foo.bats, line 15)
[+0018s] #   `run_podman image exists sdfsdfsdf' failed
[+0018s] # [07:46:52.641456481] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman rm -t 0 --all --force --ignore
[+0018s] # [07:46:52.688219032] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman ps --all --external --format {{.ID}} {{.Names}}
[+0018s] # [07:46:52.747613611] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman images --all --format {{.Repository}}:{{.Tag}} {{.ID}}
[+0018s] # [07:46:52.791987930] quay.io/libpod/testimage:20221018 f5a99120db64
[+0018s] # [07:46:52.801043907] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman container exists nonesuch
[+0018s] # [07:46:52.842840593] [ rc=1 (expected) ]
[+0018s] # [07:46:52.848492506] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman container exists nonesuch
[+0018s] # [07:46:52.893686015] [ rc=1 (expected) ]
[+0018s] # [07:47:02.910006230] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman image inspect --format {{.Os}} quay.io/libpod/testimage:20221018
[+0018s] # [07:47:02.961306244] linux
[+0018s] # [07:47:02.966100272] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman run --rm quay.io/libpod/testimage:20221018 sleep 7
[+0018s] # [07:47:10.167561880] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman image exists sdfsdfsdf
[+0018s] # [07:47:10.213750267] [ rc=1 (** EXPECTED 0 **) ]
[+0018s] # #/vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv
[+0018s] # #| FAIL: exit code is 1; expected 0
[+0018s] # #\^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
[+0018s] # # [teardown]
[+0018s] # [07:47:10.219704348] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman pod rm -t 0 --all --force --ignore
[+0018s] # [07:47:10.263332041] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman rm -t 0 --all --force --ignore
[+0018s] # [07:47:10.300645440] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman network prune --force
[+0018s] # [07:47:10.337464840] $ /home/esm/src/atomic/2018-02.podman/libpod/bin/podman volume rm -a -f
>>>
<span class="timestamp">[+0000s] </span>1..2
<span class="timestamp">         </span><span class='bats-passed'><a name='t--00001'>ok 1 this passes</a></span>
<span class="timestamp">[+0018s] </span><span class='bats-failed'><a name='t--00002'>not ok 2 this fails, and includes command timestamps</a></span>
<span class="timestamp">         </span><span class='bats-log'># (from function `die&#39; in file test/system/<a class="codelink" href="https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/system/helpers.bash#L564">helpers.bash, line 564</a>,</span>
<span class="timestamp">         </span><span class='bats-log'>#  from function `run_podman&#39; in file test/system/<a class="codelink" href="https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/system/helpers.bash#L245">helpers.bash, line 245</a>,</span>
<span class="timestamp">         </span><span class='bats-log'>#  in test file test/system/<a class="codelink" href="https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/system/888-foo.bats#L15">888-foo.bats, line 15</a>)</span>
<span class="timestamp">         </span><span class='bats-log'>#   `run_podman image exists sdfsdfsdf&#39; failed</span>
<span class="timestamp"><span title="07:46:52.641456481">&lt;+     &gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> rm -t 0 --all --force --ignore</b></span>
<span class="timestamp"><span title="07:46:52.688219032">&lt;+046ms&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> ps --all --external --format {{.ID}} {{.Names}}</b></span>
<span class="timestamp"><span title="07:46:52.747613611">&lt;+059ms&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> images --all --format {{.Repository}}:{{.Tag}} {{.ID}}</b></span>
<span class="timestamp"><span title="07:46:52.791987930">&lt;+044ms&gt;</span> </span><span class='bats-log'># quay.io/libpod/testimage:20221018 f5a99120db64</span>
<span class="timestamp"><span title="07:46:52.801043907">&lt;+009ms&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> container exists nonesuch</b></span>
<span class="timestamp"><span title="07:46:52.842840593">&lt;+041ms&gt;</span> </span><span class='bats-log'># [ rc=1 (expected) ]</span>
<span class="timestamp"><span title="07:46:52.848492506">&lt;+005ms&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> container exists nonesuch</b></span>
<span class="timestamp"><span title="07:46:52.893686015">&lt;+045ms&gt;</span> </span><span class='bats-log'># [ rc=1 (expected) ]</span>
<span class="timestamp"><span title="07:47:02.910006230">&lt;+0010s&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> image inspect --format {{.Os}} quay.io/libpod/testimage:20221018</b></span>
<span class="timestamp"><span title="07:47:02.961306244">&lt;+051ms&gt;</span> </span><span class='bats-log'># linux</span>
<span class="timestamp"><span title="07:47:02.966100272">&lt;+004ms&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> run --rm quay.io/libpod/testimage:20221018 sleep 7</b></span>
<span class="timestamp"><span title="07:47:10.167561880">&lt;+7.20s&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> image exists sdfsdfsdf</b></span>
<span class="timestamp"><span title="07:47:10.213750267">&lt;+046ms&gt;</span> </span><span class='bats-log'># [ rc=1 (** EXPECTED 0 **) ]</span>
<span class="timestamp">         </span><span class='bats-log-failblock'># #/vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv</span>
<span class="timestamp">         </span><span class='bats-log-failblock'># #| FAIL: exit code is 1; expected 0</span>
<span class="timestamp">         </span><span class='bats-log-failblock'># #\^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^</span>
<span class="timestamp">         </span><span class='bats-log'># # [teardown]</span>
<span class="timestamp"><span title="07:47:10.219704348">&lt;+005ms&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> pod rm -t 0 --all --force --ignore</b></span>
<span class="timestamp"><span title="07:47:10.263332041">&lt;+043ms&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> rm -t 0 --all --force --ignore</b></span>
<span class="timestamp"><span title="07:47:10.300645440">&lt;+037ms&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> network prune --force</b></span>
<span class="timestamp"><span title="07:47:10.337464840">&lt;+036ms&gt;</span> </span><span class='bats-log'># $ <b><span title="/home/esm/src/atomic/2018-02.podman/libpod/bin/podman">podman</span> volume rm -a -f</b></span>
<hr/><span class='bats-summary'>Summary: <span class='bats-passed'>1 Passed</span>, <span class='bats-failed'>1 Failed</span>. Total tests: 2</span>

== simple ginkgo

<<<
[05:47:08] START - All [+xxxx] lines that follow are relative to 2023-04-17T05:47:08.
[+0004s] CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build  -ldflags '-X github.com/containers/podman/v4/libpod/define.gitCommit=074143b0fac7af72cd92048d27931a92fe745084 -X github.com/containers/podman/v4/libpod/define.buildInfo=1681728432 -X github.com/containers/podman/v4/libpod/config._installPrefix=/usr/local -X github.com/containers/podman/v4/libpod/config._etcDir=/usr/local/etc -X github.com/containers/podman/v4/pkg/systemd/quadlet._binDir=/usr/local/bin -X github.com/containers/common/pkg/config.additionalHelperBinariesDir= ' -tags "   selinux systemd seccomp" -o test/checkseccomp/checkseccomp ./test/checkseccomp
[+0006s] CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build  -ldflags '-X github.com/containers/podman/v4/libpod/define.gitCommit=074143b0fac7af72cd92048d27931a92fe745084 -X github.com/containers/podman/v4/libpod/define.buildInfo=1681728434 -X github.com/containers/podman/v4/libpod/config._installPrefix=/usr/local -X github.com/containers/podman/v4/libpod/config._etcDir=/usr/local/etc -X github.com/containers/podman/v4/pkg/systemd/quadlet._binDir=/usr/local/bin -X github.com/containers/common/pkg/config.additionalHelperBinariesDir= ' -o test/goecho/goecho ./test/goecho
[+0006s] ./hack/install_catatonit.sh
[+0270s] ------------------------------
[+0271s] • [3.327 seconds]
[+0271s] Podman restart
[+0271s] /var/tmp/go/src/github.com/containers/podman/test/e2e/restart_test.go:14
[+0271s]   podman restart non-stop container with short timeout
[+0271s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/restart_test.go:148
[+0271s]&TRAILINGSPACE;
[+0271s]   Timeline >>
[+0271s]   > Enter [BeforeEach] Podman restart - /var/tmp/go/src/github.com/containers/podman/test/e2e/restart_test.go:21 @ 04/17/23 10:00:28.653
[+0271s]   < Exit [BeforeEach] Podman restart - /var/tmp/go/src/github.com/containers/podman/test/e2e/restart_test.go:21 @ 04/17/23 10:00:28.653 (0s)
[+0271s]   > Enter [It] podman restart non-stop container with short timeout - /var/tmp/go/src/github.com/containers/podman/test/e2e/restart_test.go:148 @ 04/17/23 10:00:28.653
[+0271s]   Running: /var/tmp/go/src/github.com/containers/podman/bin/podman --storage-opt vfs.imagestore=/tmp/imagecachedir --root /tmp/podman_test2968516396/root --runroot /tmp/podman_test2968516396/runroot --runtime crun --conmon /usr/bin/conmon --network-config-dir /tmp/podman_test2968516396/root/etc/networks --network-backend netavark --cgroup-manager systemd --tmpdir /tmp/podman_test2968516396 --events-backend file --db-backend sqlite --storage-driver vfs run -d --name test1 --env STOPSIGNAL=SIGKILL quay.io/libpod/alpine:latest sleep 999
[+0271s]   7f5f8fb3d043984cdff65994d14c4fd157479d20e0a0fcf769c35b50e8975edc
[+0271s]   Running: /var/tmp/go/src/github.com/containers/podman/bin/podman --storage-opt vfs.imagestore=/tmp/imagecachedir --root /tmp/podman_test2968516396/root --runroot /tmp/podman_test2968516396/runroot --runtime crun --conmon /usr/bin/conmon --network-config-dir /tmp/podman_test2968516396/root/etc/networks --network-backend netavark --cgroup-manager systemd --tmpdir /tmp/podman_test2968516396 --events-backend file --db-backend sqlite --storage-driver vfs restart -t 2 test1
[+0271s]   time="2023-04-17T10:00:31-05:00" level=warning msg="StopSignal SIGTERM failed to stop container test1 in 2 seconds, resorting to SIGKILL"
[+0271s]   test1
[+0271s]   < Exit [It] podman restart non-stop container with short timeout - /var/tmp/go/src/github.com/containers/podman/test/e2e/restart_test.go:148 @ 04/17/23 10:00:31.334 (2.681s)
[+0271s]   > Enter [AfterEach] Podman restart - /var/tmp/go/src/github.com/containers/podman/test/e2e/restart_test.go:30 @ 04/17/23 10:00:31.334
[+0271s]   Running: /var/tmp/go/src/github.com/containers/podman/bin/podman --storage-opt vfs.imagestore=/tmp/imagecachedir --root /tmp/podman_test2968516396/root --runroot /tmp/podman_test2968516396/runroot --runtime crun --conmon /usr/bin/conmon --network-config-dir /tmp/podman_test2968516396/root/etc/networks --network-backend netavark --cgroup-manager systemd --tmpdir /tmp/podman_test2968516396 --events-backend file --db-backend sqlite --storage-driver vfs stop --all -t 0
[+0271s]   7f5f8fb3d043984cdff65994d14c4fd157479d20e0a0fcf769c35b50e8975edc
[+0271s]   Running: /var/tmp/go/src/github.com/containers/podman/bin/podman --storage-opt vfs.imagestore=/tmp/imagecachedir --root /tmp/podman_test2968516396/root --runroot /tmp/podman_test2968516396/runroot --runtime crun --conmon /usr/bin/conmon --network-config-dir /tmp/podman_test2968516396/root/etc/networks --network-backend netavark --cgroup-manager systemd --tmpdir /tmp/podman_test2968516396 --events-backend file --db-backend sqlite --storage-driver vfs pod rm -fa -t 0
[+0271s]   Running: /var/tmp/go/src/github.com/containers/podman/bin/podman --storage-opt vfs.imagestore=/tmp/imagecachedir --root /tmp/podman_test2968516396/root --runroot /tmp/podman_test2968516396/runroot --runtime crun --conmon /usr/bin/conmon --network-config-dir /tmp/podman_test2968516396/root/etc/networks --network-backend netavark --cgroup-manager systemd --tmpdir /tmp/podman_test2968516396 --events-backend file --db-backend sqlite --storage-driver vfs rm -fa -t 0
[+0271s]   7f5f8fb3d043984cdff65994d14c4fd157479d20e0a0fcf769c35b50e8975edc
[+0271s]   < Exit [AfterEach] Podman restart - /var/tmp/go/src/github.com/containers/podman/test/e2e/restart_test.go:30 @ 04/17/23 10:00:31.979 (645ms)
[+0271s]   << Timeline
[+0296s] ------------------------------
[+0298s] • [FAILED] [6.071 seconds]
[+0298s] TOP-LEVEL [AfterEach]
[+0298s] /var/tmp/go/src/github.com/containers/podman/test/e2e/common_test.go:117
[+0298s]   Podman pod create
[+0298s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/pod_infra_container_test.go:12
[+0298s]     podman pod correctly sets up PIDNS
[+0298s]     /var/tmp/go/src/github.com/containers/podman/test/e2e/pod_infra_container_test.go:154
[+0298s]&TRAILINGSPACE;&TRAILINGSPACE;&TRAILINGSPACE;
[+0298s]   Timeline >>
[+0298s]   << Timeline
[+1741s] ------------------------------
[+1741s]&TRAILINGSPACE;
[+1741s] Summarizing 1 Failure:
[+1741s]   [FAIL] TOP-LEVEL [AfterEach] Podman pod create podman pod correctly sets up PIDNS
[+1741s]   /var/tmp/go/src/github.com/containers/podman/test/e2e/common_test.go:657
[+1741s]&TRAILINGSPACE;
[+1741s] Ran 1889 of 2014 Specs in 1607.919 seconds
[+1741s] FAIL! -- 1881 Passed | 1 Failed | 0 Pending | 125 Skipped
>>>
[05:47:08] START - All [+xxxx] lines that follow are relative to 2023-04-17T05:47:08.
<span class="timestamp">[+0004s] </span>CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build  -ldflags &#39;-X github.com/containers/podman/v4/libpod/define.gitCommit=074143b0fac7af72cd92048d27931a92fe745084 -X github.com/containers/podman/v4/libpod/define.buildInfo=1681728432 -X github.com/containers/podman/v4/libpod/config._installPrefix=/usr/local -X github.com/containers/podman/v4/libpod/config._etcDir=/usr/local/etc -X github.com/containers/podman/v4/pkg/systemd/quadlet._binDir=/usr/local/bin -X github.com/containers/common/pkg/config.additionalHelperBinariesDir= &#39; -tags &quot;   selinux systemd seccomp&quot; -o test/checkseccomp/checkseccomp ./test/checkseccomp
<span class="timestamp">[+0006s] </span>CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build  -ldflags &#39;-X github.com/containers/podman/v4/libpod/define.gitCommit=074143b0fac7af72cd92048d27931a92fe745084 -X github.com/containers/podman/v4/libpod/define.buildInfo=1681728434 -X github.com/containers/podman/v4/libpod/config._installPrefix=/usr/local -X github.com/containers/podman/v4/libpod/config._etcDir=/usr/local/etc -X github.com/containers/podman/v4/pkg/systemd/quadlet._binDir=/usr/local/bin -X github.com/containers/common/pkg/config.additionalHelperBinariesDir= &#39; -o test/goecho/goecho ./test/goecho
<span class="timestamp">         </span>./hack/install_catatonit.sh
</pre>
<hr />
<pre>
<span class="timestamp">[+0271s] </span>• <b>[3.327 seconds]</b>
<span class="timestamp">         </span><a name='t--Podman-restart--1'><h2 class="log-passed">Podman restart</h2></a>
<span class="timestamp">         </span>/var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/restart_test.go#L14'>/containers/podman/test/e2e/restart_test.go:14</a>
<span class="timestamp">         </span><a name='t--Podman-restart-podman-restart-non-stop-container-with-short-timeout--1'><h2 class="log-passed">  podman restart non-stop container with short timeout</h2></a>
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/restart_test.go#L148'>/containers/podman/test/e2e/restart_test.go:148</a>
<span class="timestamp">         </span>
<span class="timestamp">         </span>  Timeline &gt;&gt;
<div class="ginkgo-timeline ginkgo-beforeeach"><span class="timestamp">         </span>  &rarr; Enter [<b>BeforeEach</b>] Podman restart - /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/restart_test.go#L21'>/containers/podman/test/e2e/restart_test.go:21</a> @ 04/17/23 10:00:28.653
<span class="timestamp">         </span>  &larr; Exit  [BeforeEach] Podman restart - /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/restart_test.go#L21'>/containers/podman/test/e2e/restart_test.go:21</a> @ 04/17/23 10:00:28.653 (0s)
</div><div class="ginkgo-timeline ginkgo-it"><span class="timestamp">         </span>  &rarr; Enter [<b>It</b>] podman restart non-stop container with short timeout - /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/restart_test.go#L148'>/containers/podman/test/e2e/restart_test.go:148</a> @ 04/17/23 10:00:28.653
<span class="timestamp">         </span><span class="boring">  #</span> <span title="/var/tmp/go/src/github.com/containers/podman/bin/podman"><b>podman</b></span> <span class="boring" title="--storage-opt vfs.imagestore=/tmp/imagecachedir
--root /tmp/podman_test2968516396/root
--runroot /tmp/podman_test2968516396/runroot
--runtime crun
--conmon /usr/bin/conmon
--network-config-dir /tmp/podman_test2968516396/root/etc/networks
--network-backend netavark
--cgroup-manager systemd
--tmpdir /tmp/podman_test2968516396
--events-backend file
--db-backend sqlite
--storage-driver vfs">[options]</span><b> run -d --name test1 --env STOPSIGNAL=SIGKILL quay.io/libpod/alpine:latest sleep 999</b>
<span class="timestamp">         </span>  7f5f8fb3d043984cdff65994d14c4fd157479d20e0a0fcf769c35b50e8975edc
<span class="timestamp">         </span><span class="boring">  #</span> <span title="/var/tmp/go/src/github.com/containers/podman/bin/podman"><b>podman</b></span> <span class="boring" title="--storage-opt vfs.imagestore=/tmp/imagecachedir
--root /tmp/podman_test2968516396/root
--runroot /tmp/podman_test2968516396/runroot
--runtime crun
--conmon /usr/bin/conmon
--network-config-dir /tmp/podman_test2968516396/root/etc/networks
--network-backend netavark
--cgroup-manager systemd
--tmpdir /tmp/podman_test2968516396
--events-backend file
--db-backend sqlite
--storage-driver vfs">[options]</span><b> restart -t 2 test1</b>
<span class="timestamp">         </span>  time=<span class='log-warning'>&quot;2023-04-17T10:00:31-05:00&quot;</span> level=<span class='log-warning'>warning</span> msg=<span class='log-warning'>&quot;StopSignal SIGTERM failed to stop container test1 in 2 seconds, resorting to SIGKILL&quot;</span>
<span class="timestamp">         </span>  test1
<span class="timestamp">         </span>  &larr; Exit  [It] podman restart non-stop container with short timeout - /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/restart_test.go#L148'>/containers/podman/test/e2e/restart_test.go:148</a> @ 04/17/23 10:00:31.334 (2.681s)
</div><div class="ginkgo-timeline ginkgo-aftereach"><span class="timestamp">         </span>  &rarr; Enter [<b>AfterEach</b>] Podman restart - /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/restart_test.go#L30'>/containers/podman/test/e2e/restart_test.go:30</a> @ 04/17/23 10:00:31.334
<span class="timestamp">         </span><span class="boring">  #</span> <span title="/var/tmp/go/src/github.com/containers/podman/bin/podman"><b>podman</b></span> <span class="boring" title="--storage-opt vfs.imagestore=/tmp/imagecachedir
--root /tmp/podman_test2968516396/root
--runroot /tmp/podman_test2968516396/runroot
--runtime crun
--conmon /usr/bin/conmon
--network-config-dir /tmp/podman_test2968516396/root/etc/networks
--network-backend netavark
--cgroup-manager systemd
--tmpdir /tmp/podman_test2968516396
--events-backend file
--db-backend sqlite
--storage-driver vfs">[options]</span><b> stop --all -t 0</b>
<span class="timestamp">         </span>  7f5f8fb3d043984cdff65994d14c4fd157479d20e0a0fcf769c35b50e8975edc
<span class="timestamp">         </span><span class="boring">  #</span> <span title="/var/tmp/go/src/github.com/containers/podman/bin/podman"><b>podman</b></span> <span class="boring" title="--storage-opt vfs.imagestore=/tmp/imagecachedir
--root /tmp/podman_test2968516396/root
--runroot /tmp/podman_test2968516396/runroot
--runtime crun
--conmon /usr/bin/conmon
--network-config-dir /tmp/podman_test2968516396/root/etc/networks
--network-backend netavark
--cgroup-manager systemd
--tmpdir /tmp/podman_test2968516396
--events-backend file
--db-backend sqlite
--storage-driver vfs">[options]</span><b> pod rm -fa -t 0</b>
<span class="timestamp">         </span><span class="boring">  #</span> <span title="/var/tmp/go/src/github.com/containers/podman/bin/podman"><b>podman</b></span> <span class="boring" title="--storage-opt vfs.imagestore=/tmp/imagecachedir
--root /tmp/podman_test2968516396/root
--runroot /tmp/podman_test2968516396/runroot
--runtime crun
--conmon /usr/bin/conmon
--network-config-dir /tmp/podman_test2968516396/root/etc/networks
--network-backend netavark
--cgroup-manager systemd
--tmpdir /tmp/podman_test2968516396
--events-backend file
--db-backend sqlite
--storage-driver vfs">[options]</span><b> rm -fa -t 0</b>
<span class="timestamp">         </span>  7f5f8fb3d043984cdff65994d14c4fd157479d20e0a0fcf769c35b50e8975edc
<span class="timestamp">         </span>  &larr; Exit  [AfterEach] Podman restart - /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/restart_test.go#L30'>/containers/podman/test/e2e/restart_test.go:30</a> @ 04/17/23 10:00:31.979 (645ms)
</div><span class="timestamp">         </span>  &lt;&lt; Timeline
</pre>
<hr />
<pre>
<span class="timestamp">[+0298s] </span><span class="log-failed">• [FAILED] <b><span class='log-slow'>[6.071 seconds]</span></b></span>
<span class="timestamp">         </span><a name='t--TOP-LEVEL--AfterEach---1'><h2 class="log-failed">TOP-LEVEL [AfterEach]</h2></a>
<span class="timestamp">         </span>/var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/common_test.go#L117'>/containers/podman/test/e2e/common_test.go:117</a>
<span class="timestamp">         </span><a name='t--Podman-pod-create--1'><h2 class="log-failed">  Podman pod create</h2></a>
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/pod_infra_container_test.go#L12'>/containers/podman/test/e2e/pod_infra_container_test.go:12</a>
<span class="timestamp">         </span><a name='t--Podman-pod-create-podman-pod-correctly-sets-up-PIDNS--1'><h2 class="log-failed">    podman pod correctly sets up PIDNS</h2></a>
<span class="timestamp">         </span>    /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/pod_infra_container_test.go#L154'>/containers/podman/test/e2e/pod_infra_container_test.go:154</a>
<span class="timestamp">         </span>&TRAILINGSPACE;&TRAILINGSPACE;
<span class="timestamp">         </span>  Timeline &gt;&gt;
<span class="timestamp">         </span>  &lt;&lt; Timeline
</pre>
<hr />
<pre>
<span class="timestamp">[+1741s] </span>
<span class="timestamp">         </span>Summarizing 1 Failure:
<span class="timestamp">         </span><span class="log-error">  [FAIL] TOP-LEVEL [AfterEach] <a href='#t--Podman-pod-create-podman-pod-correctly-sets-up-PIDNS--1'>Podman pod create podman pod correctly sets up PIDNS</a></span>
<span class="timestamp">         </span>  /var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/074143b0fac7af72cd92048d27931a92fe745084/test/e2e/common_test.go#L657'>/containers/podman/test/e2e/common_test.go:657</a>
<span class="timestamp">         </span>
<span class="timestamp">         </span>Ran 1889 of 2014 Specs in 1607.919 seconds
<span class="timestamp">         </span><span class="ginkgo-final-fail">FAIL!</span> -- <span class="bats-passed"><b>1881</b> Passed</span> | <span class="bats-failed"><b>1</b> Failed</span> | 0 Pending | <span class="bats-skipped"><b>125</b> Skipped</span>

== simple python

<<<
[+0234s] env CONTAINERS_CONF=/var/tmp/go/src/github.com/containers/podman/test/apiv2/containers.conf PODMAN=./bin/podman /usr/bin/python3 -m unittest discover -v ./test/python/docker
[+0238s] test_copy_to_container (compat.test_containers.TestContainers) ... /usr/lib/python3.10/site-packages/docker/utils/utils.py:269: DeprecationWarning: urllib.parse.splitnport() is deprecated as of 3.8, use urllib.parse.urlparse() instead
[+0238s]   host, port = splitnport(parsed_url.netloc)
[+0241s] ok
[+0243s] test_create_container (compat.test_containers.TestContainers) ... ok
[+0244s] test_create_network (compat.test_containers.TestContainers) ... ok
[+0245s] test_filters (compat.test_containers.TestContainers) ... skipped 'TODO Endpoint does not yet support filters'
[+0246s] test_kill_container (compat.test_containers.TestContainers) ... /usr/lib64/python3.10/threading.py:372: ResourceWarning: unclosed <socket.socket fd=4, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=('127.0.0.1', 55054), raddr=('127.0.0.1', 8080)>
[+0246s]   waiters_to_notify = _deque(_islice(all_waiters, n))
[+0246s] ResourceWarning: Enable tracemalloc to get the object allocation traceback
[+0247s] ok
[+0248s] test_list_container (compat.test_containers.TestContainers) ... ok
[+0252s] test_mount_preexisting_dir (compat.test_containers.TestContainers) ... ok
[+0253s] test_mount_rw_by_default (compat.test_containers.TestContainers) ... ok
[+0257s] test_non_existent_workdir (compat.test_containers.TestContainers) ... ok
[+0258s] test_pause_container (compat.test_containers.TestContainers) ... ok
[+0260s] test_pause_stopped_container (compat.test_containers.TestContainers) ... ok
[+0261s] test_remove_container (compat.test_containers.TestContainers) ... ok
[+0262s] test_remove_container_without_force (compat.test_containers.TestContainers) ... /usr/lib64/python3.10/email/feedparser.py:89: ResourceWarning: unclosed <socket.socket fd=4, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=('127.0.0.1', 55068), raddr=('127.0.0.1', 8080)>
[+0262s]   for ateof in reversed(self._eofstack):
[+0262s] ResourceWarning: Enable tracemalloc to get the object allocation traceback
[+0262s] /usr/lib64/python3.10/email/feedparser.py:89: ResourceWarning: unclosed <socket.socket fd=5, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=('127.0.0.1', 55074), raddr=('127.0.0.1', 8080)>
[+0262s]   for ateof in reversed(self._eofstack):
[+0262s] ResourceWarning: Enable tracemalloc to get the object allocation traceback
[+0262s] ok
[+0264s] test_restart_container (compat.test_containers.TestContainers) ... ok
[+0265s] test_start_container (compat.test_containers.TestContainers) ... ok
[+0267s] test_start_container_with_random_port_bind (compat.test_containers.TestContainers) ... ok
[+0268s] test_stop_container (compat.test_containers.TestContainers) ... ok
[+0269s] test_unpause_container (compat.test_containers.TestContainers) ... ok
[+0273s] test_build_image (compat.test_images.TestImages) ... ok
[+0273s] test_get_image_exists_not (compat.test_images.TestImages)
[+0274s] Negative test for get image ... ok
[+0274s] test_image_history (compat.test_images.TestImages)
[+0274s] Image history ... ok
[+0274s] test_list_images (compat.test_images.TestImages)
[+0276s] List images ... ok
[+0276s] test_load_corrupt_image (compat.test_images.TestImages)
[+0277s] Import|Load Image failure ... ok
[+0277s] test_load_image (compat.test_images.TestImages)
[+0279s] Import|Load Image ... ok
[+0279s] test_remove_image (compat.test_images.TestImages)
[+0280s] Remove image ... ok
[+0280s] test_retag_valid_image (compat.test_images.TestImages)
[+0280s] Validates if name updates when the image is retagged ... ok
[+0280s] test_save_image (compat.test_images.TestImages)
[+0282s] Export Image ... ok
[+0282s] test_search_bogus_image (compat.test_images.TestImages)
[+0290s] Search for bogus image should throw exception ... ok
[+0290s] test_search_image (compat.test_images.TestImages)
[+0291s] Search for image ... FAIL
[+0291s] test_tag_valid_image (compat.test_images.TestImages)
[+0292s] Validates if the image is tagged successfully ... ok
[+0296s] test_Info (compat.test_system.TestSystem) ... ok
[+0298s] test_info_container_details (compat.test_system.TestSystem) ... ok
[+0299s] test_version (compat.test_system.TestSystem) ... ok
[+0299s] ======================================================================
[+0299s] FAIL: test_search_image (compat.test_images.TestImages)
[+0299s] Search for image
[+0299s] ----------------------------------------------------------------------
[+0299s] Traceback (most recent call last):
[+0299s]   File "/var/tmp/go/src/github.com/containers/podman/test/python/docker/compat/test_images.py", line 90, in test_search_image
[+0299s]     self.assertIn("alpine", r["Name"])
[+0299s] AssertionError: 'alpine' not found in 'docker.io/docker/desktop-kubernetes'
[+0299s] ----------------------------------------------------------------------
[+0299s] Ran 33 tests in 63.138s
[+0299s] FAILED (failures=1, skipped=1)
[+0299s] make: *** [Makefile:616: localapiv2] Error 1
>>>
<span class="timestamp">[+0234s] </span>env CONTAINERS_CONF=/var/tmp/go/src/github.com/containers/podman/test/apiv2/containers.conf PODMAN=./bin/podman /usr/bin/python3 -m unittest discover -v ./test/python/docker
<span class="timestamp">[+0238s] </span>test_copy_to_container (compat.test_containers.TestContainers) ... /usr/lib/python3.10/site-packages/docker/utils/utils.py:269: DeprecationWarning: urllib.parse.splitnport() is deprecated as of 3.8, use urllib.parse.urlparse() instead
<span class="timestamp">         </span>  host, port = splitnport(parsed_url.netloc)
<span class="timestamp">[+0241s] </span>ok
<span class="timestamp">[+0243s] </span><span class='bats-passed'>test_create_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0244s] </span><span class='bats-passed'>test_create_network (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0245s] </span><span class='bats-skipped'>test_filters (compat.test_containers.TestContainers) ... skipped &#39;TODO Endpoint does not yet support filters&#39;</span>
<span class="timestamp">[+0246s] </span>test_kill_container (compat.test_containers.TestContainers) ... /usr/lib64/python3.10/threading.py:372: ResourceWarning: unclosed &lt;socket.socket fd=4, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=(&#39;127.0.0.1&#39;, 55054), raddr=(&#39;127.0.0.1&#39;, 8080)&gt;
<span class="timestamp">         </span>  waiters_to_notify = _deque(_islice(all_waiters, n))
<span class="timestamp">         </span>ResourceWarning: Enable tracemalloc to get the object allocation traceback
<span class="timestamp">[+0247s] </span>ok
<span class="timestamp">[+0248s] </span><span class='bats-passed'>test_list_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0252s] </span><span class='bats-passed'>test_mount_preexisting_dir (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0253s] </span><span class='bats-passed'>test_mount_rw_by_default (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0257s] </span><span class='bats-passed'>test_non_existent_workdir (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0258s] </span><span class='bats-passed'>test_pause_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0260s] </span><span class='bats-passed'>test_pause_stopped_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0261s] </span><span class='bats-passed'>test_remove_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0262s] </span>test_remove_container_without_force (compat.test_containers.TestContainers) ... /usr/lib64/python3.10/email/feedparser.py:89: ResourceWarning: unclosed &lt;socket.socket fd=4, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=(&#39;127.0.0.1&#39;, 55068), raddr=(&#39;127.0.0.1&#39;, 8080)&gt;
<span class="timestamp">         </span>  for ateof in reversed(self._eofstack):
<span class="timestamp">         </span>ResourceWarning: Enable tracemalloc to get the object allocation traceback
<span class="timestamp">         </span>/usr/lib64/python3.10/email/feedparser.py:89: ResourceWarning: unclosed &lt;socket.socket fd=5, family=AddressFamily.AF_INET, type=SocketKind.SOCK_STREAM, proto=6, laddr=(&#39;127.0.0.1&#39;, 55074), raddr=(&#39;127.0.0.1&#39;, 8080)&gt;
<span class="timestamp">         </span>  for ateof in reversed(self._eofstack):
<span class="timestamp">         </span>ResourceWarning: Enable tracemalloc to get the object allocation traceback
<span class="timestamp">         </span>ok
<span class="timestamp">[+0264s] </span><span class='bats-passed'>test_restart_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0265s] </span><span class='bats-passed'>test_start_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0267s] </span><span class='bats-passed'>test_start_container_with_random_port_bind (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0268s] </span><span class='bats-passed'>test_stop_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0269s] </span><span class='bats-passed'>test_unpause_container (compat.test_containers.TestContainers) ... ok</span>
<span class="timestamp">[+0273s] </span><span class='bats-passed'>test_build_image (compat.test_images.TestImages) ... ok</span>
<span class="timestamp">         </span>test_get_image_exists_not (compat.test_images.TestImages)
<span class="timestamp">[+0274s] </span><span class='bats-passed'>Negative test for get image ... ok</span>
<span class="timestamp">         </span>test_image_history (compat.test_images.TestImages)
<span class="timestamp">         </span><span class='bats-passed'>Image history ... ok</span>
<span class="timestamp">         </span>test_list_images (compat.test_images.TestImages)
<span class="timestamp">[+0276s] </span><span class='bats-passed'>List images ... ok</span>
<span class="timestamp">         </span>test_load_corrupt_image (compat.test_images.TestImages)
<span class="timestamp">[+0277s] </span><span class='bats-passed'>Import|Load Image failure ... ok</span>
<span class="timestamp">         </span>test_load_image (compat.test_images.TestImages)
<span class="timestamp">[+0279s] </span><span class='bats-passed'>Import|Load Image ... ok</span>
<span class="timestamp">         </span>test_remove_image (compat.test_images.TestImages)
<span class="timestamp">[+0280s] </span><span class='bats-passed'>Remove image ... ok</span>
<span class="timestamp">         </span>test_retag_valid_image (compat.test_images.TestImages)
<span class="timestamp">         </span><span class='bats-passed'>Validates if name updates when the image is retagged ... ok</span>
<span class="timestamp">         </span>test_save_image (compat.test_images.TestImages)
<span class="timestamp">[+0282s] </span><span class='bats-passed'>Export Image ... ok</span>
<span class="timestamp">         </span>test_search_bogus_image (compat.test_images.TestImages)
<span class="timestamp">[+0290s] </span><span class='bats-passed'>Search for bogus image should throw exception ... ok</span>
<span class="timestamp">         </span>test_search_image (compat.test_images.TestImages)
<span class="timestamp">[+0291s] </span><span class='bats-failed'>Search for image ... FAIL</span>
<span class="timestamp">         </span>test_tag_valid_image (compat.test_images.TestImages)
<span class="timestamp">[+0292s] </span><span class='bats-passed'>Validates if the image is tagged successfully ... ok</span>
<span class="timestamp">[+0296s] </span><span class='bats-passed'>test_Info (compat.test_system.TestSystem) ... ok</span>
<span class="timestamp">[+0298s] </span><span class='bats-passed'>test_info_container_details (compat.test_system.TestSystem) ... ok</span>
<span class="timestamp">[+0299s] </span><span class='bats-passed'>test_version (compat.test_system.TestSystem) ... ok</span>
<div class='log-error'>
<span class="timestamp">         </span>======================================================================
<span class="timestamp">         </span>FAIL: test_search_image (compat.test_images.TestImages)
<span class="timestamp">         </span>Search for image
<span class="timestamp">         </span>----------------------------------------------------------------------
<span class="timestamp">         </span>Traceback (most recent call last):
<span class="timestamp">         </span>  File &quot;/var/tmp/go/src/github.com<a class="codelink" href='https://github.com/containers/podman/blob/ceci-nest-pas-une-sha/test/python/docker/compat/test_images.py#L90'>/containers/podman/test/python/docker/compat/test_images.py&quot;, line 90</a>, in test_search_image
<span class="timestamp">         </span>    self.assertIn(&quot;alpine&quot;, r[&quot;Name&quot;])
<span class="timestamp">         </span>AssertionError: &#39;alpine&#39; not found in &#39;docker.io/docker/desktop-kubernetes&#39;
<span class="timestamp">         </span>----------------------------------------------------------------------
</div>
<span class="timestamp">         </span>Ran 33 tests in 63.138s
<span class="timestamp">         </span><span class='bats-failed'>FAILED (failures=1, skipped=1)</span>
<span class="timestamp">         </span>make: *** [Makefile:616: localapiv2] Error 1
<hr/><span class='bats-summary'>Summary: <span class='bats-passed'>28 Passed</span>, <span class='bats-failed'>1 Failed</span>, <span class='bats-skipped'>1 Skipped</span>. Total tests: 30 <span class='bats-failed'>(WARNING: expected 33)</span></span>
