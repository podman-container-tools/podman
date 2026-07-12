# Podman Releases

## Overview

Podman (and podman-remote) versioning is mostly based on [semantic-versioning
standards](https://semver.org).
Significant versions
are tagged, including *release candidates* (`rc`).
All relevant **minor** releases (`vX.Y`) have their own branches.  The **latest**
development efforts occur on the *main* branch.  Branches with a
*rhel* suffix are use for long-term support of downstream RHEL releases.

## Release workflow expectations

* You have push access to the [upstream podman repository](https://github.com/containers/podman.git), and the upstream [podman-machine-os repository](https://github.com/containers/podman-machine-os)
* You understand all basic `git` operations and concepts, like creating commits,
  local vs. remote branches, rebasing, and conflict resolution.
* You have access to your public and private *GPG* keys. They should also be documented on our [release keys repo](https://github.com/containers/release-keys).
* You have reliable internet access (i.e. not the public WiFi link at McDonalds)
* Other podman maintainers are online/available for assistance if needed.
* For a **major** release, you have 4-8 hours of time available, most of which will
  be dedicated to writing release notes.
* For a **minor** or **patch** release, you have 2-4 hours of time available
  (minimum depends largely on the speed/reliability of automated testing)
* You will announce the release on the proper platforms
  (i.e. Podman blog, Twitter, Mastodon Podman and Podman-Desktop mailing lists)

# Release cadence

Upstream major or minor releases occur the 2nd week of February, May, August, November.
Branching and RC's may start several weeks beforehand.
Patch releases occur as-needed.

# Releases

## Major (***X***.y.z) release

These releases always begin from *main*, and are contained in a branch
named with the **major** and **minor** version. **Major** release branches
begin in a *release candidate* phase, with prospective release tags being
created with an `-rc` suffix.  There may be multiple *release candidate*
tags before the final/official **major** version is tagged and released.

## Significant minor (x.**Y**.z) and patch (x.y.**Z**) releases

Significant **minor** and **patch** level releases are normally
branched from *main*, but there are occasional exceptions.
Additionally, these branches may be named with `-rhel` (or another)
suffix to signify a specialized purpose.  For example, `-rhel` indicates
a release intended for downstream *RHEL* consumption.

## Unreleased Milestones

Non-release versions may occasionally appear tagged on a branch, without
the typical (major) receive media postings or artifact distribution.  For
example, as required for the (separate) RHEL release process.  Otherwise
these tags are simply milestones of reference purposes and may
generally be safely ignored.

## Process

***Note:*** This is intended as a guideline, and generalized process.
Not all steps are applicable in all situations.  Not all steps are
spelled with complete minutiae.

1. Create a new upstream release branch (if none already exist).

   1. Check if a release branch is needed. All major and minor releases should be branched before RC1.
      Patch releases typically already have a branch created.
      Branching ensures all changes are curated before inclusion in the
      release, and no new features land after the *release-candidate* phases
      are complete.
   1. Ensure your local clone is fully up to date with the remote upstream
      (`git remote update`).  Switch to this branch (`git checkout upstream/main`).
   1. Make a new local branch for the release based on *main*. For example,
      `git checkout -b vX.Y`.  Where `X.Y` represent the complete release
      version-name, including any suffix (if any) like `-rhel`.  ***DO NOT***
      include any `-rc` suffix in the branch name.
   1. Push the new branch otherwise unmodified (`git push upstream vX.Y`).
   1. Check if a release branch is needed on the `podman-machine-os` repo.
      If so, repeat above steps for `podman-machine-os`.
   1. Back on the podman repo, automation will begin executing on the branch immediately.
      Because the repository allows out-of-sequence PR merging, it is possible that
      merge order introduced bugs/defects.  To establish a clean
      baseline, observe the initial CI run on the branch for any unexpected
      failures.  This can be done by going directly to
      `https://cirrus-ci.com/github/containers/podman/vX.Y`
   1. If there are CI test or automation boops that need fixing on the branch,
      attend to them using normal PR process (to *main* first, then backport
      changes to the new branch).  Ideally, CI should be "green" on the new
      branch before proceeding.

   1. Create a new branch-verification Cirrus-Cron entry.

      1. This is to ensure CI's VM image timestamps are refreshed.  Without this,
         the VM images ***will*** be permanently pruned after 60 days of inactivity
         and are hard/impossible to re-create accurately.
      1. Go to
         [https://cirrus-ci.com/github/containers/podman](https://cirrus-ci.com/github/containers/podman)
         and press the "gear" (Repository Settings) button on the top-right.
      1. At the bottom of the settings page is a table of cron-job names, branches,
         schedule, and recent status.  Below that is an editable new-entry line.
      1. Set the new job's `name` and `branch` to the name of new release branch.
      1. Set the `expression` using the form `X X X ? * 1-6` where 'X' is a number
         between 0-23 and not already taken by another job in the table.  The 1-hour
         interval is used because it takes about that long for the job to run.
      1. Add the new job by pressing the `+` button on the right-side of the
         new-entry line.


1. Create a new local working-branch to develop the release PR
   1. Ensure your local clone is fully up to
      date with the remote upstream (`git remote update`).
   1. Create a local working branch based on `upstream/main` or the correct upstream branch.
      Example: `git checkout -b bump_vX.Y.Z --no-track upstream/vX.Y`

1. Compile release notes.

   1. Ensure any/all intended PR's are completed and merged prior to any
      processing of release notes.
   1. Find all commits since the last release. There is a script, `/hack/branch_commits.rb`
      that is helpful for finding all commits in one branch, but not in another,
      accounting for cherry-picks. Commits in base branch that are not in
      the old branch will be reported. `ruby branch_commits.rb upstream/main upstream/vX.Y`
      Keep this list open/available for reference as you edit.
   1. Edit `RELEASE_NOTES.md`

      * Add/update the version-section of with sub-sections for *Features*
        (new functionality), *Changes* (Altered podman behaviors),
        *Bugfixes* (self-explanatory), *API* (All related features,
        changes, and bugfixes), and *Misc* (include any **major**
        library bumps, e.g. `c/buildah`, `c/storage`, `c/common`, etc).
      * Use your merge-bot reference PR-listing to examine each PR in turn,
        adding an entry for it into the appropriate section.
      * Use the list of commits to find the PR that the commit came from.
        Write a release note if needed.

        * Use the release note field in the PR as a guideline.
          It may be helpful but also may need rewording for consistency.
          Some PR's with a release note field may not need one, and some PR's
          without a release note field may need one.
        * Be sure to link any issue the PR fixed.
        * Do not include any PRs that are only documentation or test/automation
          changes.
        * Do not include any PRs that fix bugs which we introduced due to
          new features/enhancements.  In other words, if it was working, broke, then
          got fixed, there's no need to mention those items.

   1. Commit the `RELEASE_NOTES.md` changes, using the description
      `Create release notes for vX.Y.Z` (where `X`, `Y`, and `Z` are the
      actual version numbers).
   1. Open a Release Notes PR, or include this commit with the version bump PR.

1. Update version numbers and push tag

   1. Edit `version/rawversion/version.go` and bump the `Version` value to the new
      release version.  If there were API changes, also bump `APIVersion` value.
      Make sure to also bump the version in the swagger.yaml `pkg/api/server/docs.go`
   1. Commit this and sign the commit (`git commit -a -s -S`). The commit message
      should be `Bump to vX.Y.Z` (using the actual version numbers).
   1. Push this single change to your GitHub fork, and make a new PR,
      **being careful** to select the proper release branch as its base.
   1. Wait for all automated tests pass (including on an RC-branch PR).  Re-running
      and/or updating code as needed.
   1. In the PR, under the *Checks* tab, locate and clock on the Cirrus-CI
      task `Optional Release Test`.  In the right-hand window pane, click
      the `trigger` button and wait for the test to go green.  *This is a
      critical step* which confirms the commit is worthy of becoming a release.
   1. In the PR, under the *Checks* tab, a GitHub actions [task](https://github.com/containers/podman/actions/workflows/machine-os-pr.yml) will run.
      This action opens a PR on the [podman-machine-os repo](https://github.com/containers/podman-machine-os), which builds VM images for the release. The action will also link the `podman-machine-os` pr in a comment on the podman PR
      This action also automatically applies the `do-not-merge/wait-machine-image-build` to the Podman PR, which blocks merging until VM images are built and published.
   1. Go to the `podman-machine-os` bump pr, by clicking the link in the comment, or by finding it in the [podman-machine-os repo](https://github.com/containers/podman-machine-os/pulls).
      1. Wait for automation to finish running
      1. Once you are sure that there will be no more force pushes on the Podman release PR, merge the `podman-machine-os` bump PR
      1. Tag the `podman-machine-os` bump commit with the same version as the podman release. (git tag -s -m 'vX.Y.Z' vX.Y.Z)
      1. Push the tag.
      1. The tag will automatically trigger a Cirrus task, named “Publish Image”,
         to publish the release images. It will push the images to Quay and cut a release on the `podman-machine-os` repo. Wait for this task to complete. You can monitor the task on the [Cirrus CI dashboard](https://cirrus-ci.com/github/containers/podman-machine-os)
   1. Return to the Podman repo
   1. The `do-not-merge/wait-podman-machine-os` label should be automatically
      un-set once the `podman-machine-os` release is finished.
   1. Wait for all other PR checks to pass.
   1. Wait for other maintainers to merge the PR.
   1. Tag the `Bump to vX.Y.Z` commit as a release by running
      `git tag -s -m 'vX.Y.Z' vX.Y.Z $HASH` where `$HASH` is specified explicitly and carefully, to avoid (basically) unfixable accidents
      (if they are pushed).
   1. **Note:** This is the last point where any test-failures can be addressed
      by code changes. After pushing the new version-tag upstream, no further
      changes can be made to the code without lots of unpleasant efforts.  Please
      seek assistance if needed, before proceeding.
   1. Assuming the "Bump to ..." PR merged successfully, and you're **really**
      confident the correct commit has been tagged, push it with
      `git push upstream vX.Y.Z`
1. Monitor release automation
   1. After the tag is pushed, the release GitHub action should run.
      This action creates the GitHub release from the pushed tag,
      and automatically builds and uploads the binaries and installers to the release.
      1. The following artifacts should be attached to the release:
         * podman-installer-macos-arm64.pkg
         * podman-installer-windows-amd64.exe
         * podman-installer-windows-arm64.exe
         * podman-remote-release-darwin_arm64.zip
         * podman-remote-release-windows_amd64.zip
         * podman-remote-release-windows_arm64.zip
         * podman-remote-static-linux_amd64.tar.gz
         * podman-remote-static-linux_arm64.tar.gz
         * shasums
      1. An email should have been sent to the [podman](mailto:podman@lists.podman.io) mailing list.
         Keep an eye on it make sure the email went through to the list.
   1. The release action will also bump the Podman version on podman.io. It will open a PR if a non-rc latest version is released. Go to the [podman.io](https://github.com/containers/podman.io) repo and merge the PR opened by this action, if needed.
   1. After the tag is pushed, an action to bump to -dev will run. A PR will be opened for this bump. Merge this PR if needed.


1. Locate, Verify release testing is proceeding

   1. When the tag was pushed, an automated build was created. Locate this
      by starting from
      `https://github.com/containers/podman/tags` and finding the recent entry
      for the pushed tag.  Under the tag name will be a timestamp and abbreviated
      commit hash, for example `<> 5b2585f`.  Click the commit-hash link.
   1. In the upper-left most corner, just to the left of the "Bump to vX.Y"
      text, will be a small status icon (Yellow circle, Red "X", or green check).
      Click this, to open a small pop-up/overlay window listing all the status
      checks.
   1. In the small pop-up/overlay window, press the "Details" link on one of the
      Cirrus-CI status check entries (doesn't matter which one).
   1. On the following page, in the lower-right pane, will be a "View more details
      on Cirrus CI" link, click this.
   1. A Cirrus-CI task details page will open, click the button labeled
      "View All Tasks".
   1. Keep this page open to monitor its progress and for use in future steps.

1. Update Cirrus-CI cron job list
   1. After any Major or significant minor (esp. `-rhel`) releases, it's critical to
      maintain the Cirrus-CI cron job list.  This applies to all containers-org repos,
      not just podman.
   1. Access the repo. settings WebUI by navigating to
      `https://cirrus-ci.com/github/containers/<repo name>`
      and clicking the gear-icon in the upper-right.
   1. For minor (i.e. **NOT** `-rhel`) releases, (e.x. `vX.Y`), the previous release
      should be removed from rotation (e.x. `vX.<Y-1>`) assuming it's no longer supported.
      Simply click the trash-can icon to the right of the job definition.
   1. For `-rhel` releases, these are tied to products with specific EOL dates.  They should
      *never* be disabled unless you (and a buddy) are *absolutely* certain the product is EOL
      and will *never* ever see another backport (CVE or otherwise).
   1. On the settings page, pick a "less used" time-slot based on the currently defined
      jobs.  For example, if three jobs specify `12 12 12 ? * 1-6`, choose another.  Any
      spec. `H`/`M`/`S` value between 12 and 22 is acceptable (e.x. `22 22 22 ? * 1-6`).
      The point is to not overload the clouds with CI jobs.
   1. Following the pattern of the already defined jobs, at the bottom of the settings
      page add a new entry.  The "Name" should reflect the version number, the "Branch"
      is simply the newly created release branch name (must be exact), and the "Expression"
      is the time slot you selected (copy-paste).
   1. Click the "+" button next to the new-job row you just filled out.

1. Announce the release
      1. For major and minor releases, write a blog post and publish it to blogs.podman.io
         Highlight key features and important changes or fixes. Link to the GitHub release.
         Make sure the blog post is properly tagged with the Announcement, Release, and Podman tags,
         and any other appropriate tags.
      1. Tweet the release. Make a Mastodon post about the release.
      1. RC's can also be announced if needed.
