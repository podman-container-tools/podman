# Triaging of Podman issues
To manage new GitHub issues, maintainers perform issue triage on a regular basis and categorize the issues based on priority, type of issue, and other factors.

This process includes:
1. Ensure the issue is relevant to the correct repository (i.e. build issues go to buildah repo, podman desktop issues go to Podman Desktop repo, etc) and transfer as needed.
2. Categorize issues by type and assign its associated label ([see below](#labels)) and “traiged” label. If the issue is a bug and it is of high impact, please assign a high-impact label.
3. Assign high-impact issues to either themselves or a [core maintainer](https://github.com/containers/podman/blob/main/OWNERS#L1).
4. If [essential information is lacking](#checks-for-triaging), request it from the submitter and apply the 'needs-info' label.
5. Once all the necessary information is gathered, the maintainer will assign the high-impact label if needed and removes the ‘needs-info’ label
6. Check our [issue closing policy](https://github.com/containers/podman/blob/main/ISSUE.md#why-was-my-issue-report-closed) and close the new issue if it matches the listed criteria.


## Checks for triaging
While triaging, the maintainer has to look for the following information in the issue and ask the reporter for any missing information.

### Bugs:
1. Check what version of Podman, the distro, and any pertinent environmental notes the reporter is experiencing the problem on. This should come in the form of podman info as the issue template states.
2. If the issue is distribution specific, then suggest in the comment that it should also be brought to the attention of the distribution and close the issue.
3. If the reporter is not using the latest (or very near latest) version of Podman, the reporter should be asked to verify this still exists in main or at least in the latest release.  The triager can also verify this.
4. Check if there is a good reproducer that preferably reproduces on the latest Podman version
5. Any other missing information that could help with debugging.
6. Check for similar issues and act accordingly
7. If the issue is related to Brew. Chocolatey or another package manager, suggest the reporter to use the latest binaries on the release page

### Features:
1. Check if the feature is already added to the newer Podman releases, if it is, add the appropriate suggestion and close the issue.
2. Check if the feature is reasonable and is under the project’s scope
3. Check if the feature is clear and ask for any missing information.


### High Impact Bug Definition
1. An issue that impacts multiple users
2. An issue that is encountered on each run of Podman
3. An issue that breaks basic Podman functionality like `podman run` or `podman build`
4. A regression caused by new release

## Labels:
1. network
2. quadlet
3. machine
4. kube
5. storage
6. build
7. windows
8. macos
9. documentation
10. pasta
11. remote
12. compose
13. regression
