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

* You have push access to the [upstream podman repository](https://github.com/podman-container-tools/podman.git), and the upstream [podman-machine-os repository](https://github.com/podman-container-tools/podman-machine-os)
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
      merge order introduced bugs/defects. To establish a clean baseline, observe
      the initial GitHub Actions CI run on the branch for any unexpected failures.
      The runs can be monitored from the repository's
      [Actions page](https://github.com/podman-container-tools/podman/actions).
   1. If there are CI test or automation boops that need fixing on the branch,
      attend to them using normal PR process (to *main* first, then backport
      changes to the new branch). Ideally, CI should be "green" on the new
      branch before proceeding.


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
   1. Wait for all automated tests to pass (including on an RC-branch PR).  Re-running
      and/or updating code as needed.
   1. In the PR, under the *Checks* tab, a GitHub Actions [workflow](https://github.com/podman-container-tools/podman/actions/workflows/machine-os-pr.yml) will run.
      This workflow opens a PR on the [podman-machine-os repo](https://github.com/podman-container-tools/podman-machine-os)
      to build VM images for the release, links that PR in a comment on the Podman PR,
      and applies the `do-not-merge/wait-machine-os-build` label while the machine-os
      release is still pending.
   1. Go to the `podman-machine-os` bump PR, by clicking the link in the comment, or by finding it in the [podman-machine-os repo](https://github.com/podman-container-tools/podman-machine-os/pulls).
      1. Wait for automation to finish running
      1. Once you are sure that there will be no more force pushes on the Podman release PR, merge the `podman-machine-os` bump PR
      1. Tag the `podman-machine-os` bump commit with the same version as the podman release. (git tag -s -m 'vX.Y.Z' vX.Y.Z)
      1. Push the tag.
      1. The tag will automatically trigger the
         [`release`](https://github.com/podman-container-tools/podman-machine-os/actions/workflows/release.yml)
         GitHub Actions workflow. It publishes the images to Quay and creates a
         GitHub release in the `podman-machine-os` repository. Wait for this
         workflow to complete successfully.
   1. Return to the Podman repo.
   1. Once the `podman-machine-os` release completes, remove the
      `do-not-merge/wait-machine-os-build` label before merging the Podman bump PR.
      The Podman [release workflow](https://github.com/podman-container-tools/podman/actions/workflows/release.yml)
      runs only after the Podman tag is pushed, so this matching-release check is a
      secondary failsafe before the artifact build or release jobs start.
      Build-only dispatches skip this check.
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


1. Verify release testing is proceeding

   1. After the tag is pushed, open the
      [GitHub Actions page](https://github.com/podman-container-tools/podman/actions)
      and locate the workflow runs associated with the new tag.
   1. Monitor the `Release` workflow and ensure that all required jobs
      complete successfully.
   1. Keep the relevant workflow run open for monitoring during the remaining
      release steps.

1. Announce the release
      1. For major and minor releases, write a blog post and publish it to blogs.podman.io
         Highlight key features and important changes or fixes. Link to the GitHub release.
         Make sure the blog post is properly tagged with the Announcement, Release, and Podman tags,
         and any other appropriate tags.
      1. Tweet the release. Make a Mastodon post about the release.
      1. RC's can also be announced if needed.
