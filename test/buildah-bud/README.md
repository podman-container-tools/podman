# buildah-bud tests under podman

This directory contains tools for running 'buildah bud' tests
under podman. The key concept of the workflow is:

* Pull buildah @ version specified in go.mod
* Apply a small set of patches to buildah's tests directory, such that
  * BATS will use 'podman build' instead of 'buildah bud'; and
  * some not-applicable-under-podman tests are skipped

It's a teeny bit more complicated than that, but that's really most of
what you need to know for most purposes. The tests run in podman CI,
and for the most part are expected to just pass.

## Troubleshooting

If you're reading this, it's probably because something went wrong.
At the time of this writing (March 2021, initial commit) it is
impossible to foresee what typical failures will look like, but
my prediction is that they will fit one of two categories:

* Failure when vendoring new buildah (e.g., by dependabot)
* Other failure

Let's examine those in reverse order:

## Failure when not vendoring

Aside from flakes, my only guess here is that you broke 'podman build'.
If this is the case, it is very likely that you are aware of what you
did; and if this is the case, your change likely falls into one of
these two categories:

* "OOPS! I didn't mean to break that". Solution: fix it!
* "Uh, yeah, this is deliberate, and we choose to be incompatible with buildah". In this case, you'll need to skip or edit the failing test(s); see below.

If neither of those is the case, then I'm sorry, you're on your own.
When you figure it out, please remember to update these instructions.


## Failure when vendoring new buildah

This is what I predict will be the usual case; and I predict that
failures will fall into one of two bins:

* failure to apply the patches; and/or
* failure because there are new buildah tests for functionality not in podman

In either case, the process for solving is the same:

* Start with a checked-out podman tree with the failing PR applied
* run `./test/buildah-bud/run-buildah-bud-tests`

Presumably, something will fail here. Whatever the failure, your next step is:

* `cd test-buildah-v<TAB>` (this is a new directory created by the script)

Now there are three possible failures:

### Failure in `git am`

If the failure was in `git am`, it probably means that buildah
`tests/helpers.bash` got updated in such a way as to cause a conflict
with the patches we apply. Your best bet is to:

* Look at `tests/*.rej`
* For each rejected patch, try to figure out where it should go and how to apply it. Do so.
* `git add tests/helpers.bash` - this is for `git am`, next
* `git am --continue` - this continues the failed patch. Make sure it succeeds.
* `./make-new-buildah-diffs` - this updates your podman working directory
* `cd ..; git diff test/buildah-bud`. This will show you a diff of a .diff file, which is really painful to read. I'm sorry. Just try to confirm that the changes look like what you expect.

Proceed with 'In all cases' below.

### Failure when applying podman-custom deltas

Failure in the `apply-podman-deltas` script means that one of the
hand-crafted exceptions was not found, e.g., there's a `skip` or
`errmsg` looking for a specific `@test` in `bud.bats` that is
no longer there.

Solution:
* Inspect the error message(s) from `apply-podman-deltas`. Each message will list a specific `@test` name.
* Look at the diffs in `tests/bud.bats` between main and your PR. (I'm really sorry; there's no quick easy command-line way to do that. You will need a checked-out buildah tree, and you will need to know the old and new buildah tags).
  * In those diffs, look for changes related to each `@test` listed as an error. For example, a test being renamed or even removed.
  * Update `test/buildah-bud/apply-podman-deltas` accordingly.

Proceed with 'In all cases' below.

### Failure when running tests

If the failure was in tests run, and you're vendoring, your only real choice is to add a new `skip`:

* Identify the failing test(s)
* File a new podman issue, e.g. "podman build fails buildah XYZ test"
* Edit `test/buildah-bud/apply-podman-deltas`. Search for "actual podman bugs" near the bottom, and add a new `skip` line with the reason (INCLUDE THE ISSUE NUMBER!) and the test name.

### In all cases

You will probably want to rerun `run-buildah-bud-tests` to save yourself
the hassle of having it fail in CI. (`rm -rf test-buildah-v<TAB>` first).
If you're debugging problems that run on a specific test, you can
use `--filter="pattern"` to run only tests that match "pattern".

If everything passes, `git commit --amend` your PR, adding the
files you changed under `test/buildah-bud`, then `git push --force`.
