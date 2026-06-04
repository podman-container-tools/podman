# Project Governance

* [Subprojects](#subprojects)
    * [Adding Subprojects](#adding-subprojects)
    * [Removing Subprojects](#removing-subprojects)
* [Contributor Ladder](#contributor-ladder)
    * [Contributor](#contributor)
    * [Reviewer](#reviewer)
    * [Maintainer](#maintainer)
    * [Core Maintainer](#core-maintainer)
    * [Community Manager](#community-manager)
    * [Emeritus Maintainer](#emeritus-maintainer)
* [Maintainers File](#maintainers-file)
* [Inactivity](#inactivity)
* [Involuntary Removal](#involuntary-removal-or-demotion)
* [Stepping Down/Emeritus Process](#stepping-downemeritus-process)
* [Updates to this Document](#updates-to-this-document)
* [Contact](#contact)

# Podman Project

This document defines the governance of the Podman Project, including its subprojects. It defines the various roles our maintainers fill, how to become a maintainer, and how project-level decisions are made.

# Subprojects

The Podman project currently consists of the Podman project (the repository containing this file) and two subprojects:
* [Buildah](https://github.com/containers/buildah)
* [Skopeo](https://github.com/containers/skopeo/)

## Adding Subprojects

During a Core Maintainers meeting, any Maintainer or Core Maintainer may recommend projects to become a subproject of Podman Container Tools. These projects should have the following characteristics:
* Are strongly related to Podman Container Tools
* Share the Podman Container Tools mission of delivering a set of tools for building and managing containers on a single node
* Are appropriately licensed and governed or willing to become so
* Are under active development
* Consist of high quality code and designs

Before submitting an application to the Core Maintainers, the project in consideration must hold a consensus vote among all major contributors to join Podman Container Tools. The Core Maintainers will then review the application, and decide whether or not to accept it. If it is accepted, the Core Maintainers will assign at least one person to assist with the new subproject's integration. The definitive list of subprojects is contained above in the [Subprojects](#subprojects) section above, and the new subproject will be added to that list after its acceptance. The new subproject must map its existing maintainers to the roles defined in the [Contributor Ladder](#contributor-ladder) upon being accepted. Appointments of Community Managers and Core Maintainers from the new subproject are subject to approval by the existing Core Maintainers; other roles may be assigned freely initially, but the Contributor Ladder advancement process must be followed after the project is accepted.

## Removing Subprojects

In some cases, subprojects will become inactive or unmaintainable, or wish to separate from Podman Container Tools. Any Core Maintainer may propose the removal of a subproject on these grounds. Additionally, the Maintainers of any subproject may request removal of their subproject by a publicly visible majority vote (for example, via GitHub issue). The Core Maintainers must confirm removals with a majority vote. The subproject will be removed as from the list of subprojects in the [Subprojects](#subprojects) section above.

Subprojects which still have contributors will then be moved to a repository in their own namespace. Projects which have ceased all activity will be kept in the same namespace but archived.

# Contributor Ladder

The Podman project has a number of maintainer roles arranged in a ladder. Each role is a rung on the ladder, with different responsibilities and privileges. Community members generally start at the first levels of the "ladder" and advance as their involvement in the project grows. Our project members are happy to help you advance along the contributor ladder. At all levels, contributors are required to follow the CNCF Code of Conduct (COC).

Each of the project member roles below is organized into lists of three types of things.

* "Responsibilities" – functions of a member
* "Requirements" –  qualifications of a member
* "Privileges" –  entitlements of member

### Contributor
Description: A Contributor supports the project and adds value to it. Contributions need not be code. People at the Contributor level may be new contributors, or they may only contribute occasionally.

* Responsibilities include:
    * Follow the CNCF CoC
    * Follow the project contributing guide
* Requirements (one or several of the below):
    * Report and sometimes resolve issues against any of the project’s repositories
    * Occasionally submit PRs against any of the project’s repositories
    * Contribute to project documentation, including the manpages, tutorials, and Podman.io
    * Attend community meetings when reasonable
    * Answer questions from other community members on the mailing list, Slack, Matrix, and other communication channels
    * Assist in triaging issues, following the [issue triage guide](./TRIAGE.md)
    * Assist in reviewing pull requests, including testing patches when applicable
    * Test release candidates and provide feedback
    * Promote the project in public
    * Help run the project infrastructure
* Privileges:
    * Invitations to contributor events
    * Eligible to become a Reviewer
	* For repositories hosted on GitHub, Contributors receive no special privileges by default. Contributors who are active in answering questions and assisting with bug reports upstream may request Triage privileges from the repository's maintainers.

### Reviewer
Description: A Reviewer has responsibility for the triage of issues and review of pull requests on the Podman project or a subproject, consisting of one or more of the Git repositories that form the project. They are collectively responsible, with other Reviewers, for reviewing changes to the repository or repositories and indicating whether those changes are ready to merge. They have a track record of contribution and review in the project.

Reviewers have all the rights and responsibilities of a Contributor, plus:

* Responsibilities include:
    * Regular contribution of pull requests to the Podman project or its subprojects
    * Triage of GitHub issues on the Podman project or its subprojects
    * Regularly fixing GitHub issues on the Podman project or its subprojects
    * Following the [reviewing guide](./REVIEWING.md) and [issue triage guide](./TRIAGE.md)
    * A sustained high level of pull request reviews on the Podman project or one of its subprojects
    * Assisting new Contributors in their interactions with the project
    * Helping other contributors become reviewers
* Requirements:
    * Has a proven record of good-faith contributions to the project as a Contributor for a period of at least 6 months. The time requirement may be overridden by a supermajority (66%) vote of Maintainers and Core Maintainers.
    * Has participated in pull request review and/or issue triage on the project for at least 6 months. The time requirement may be overridden by a supermajority (66%) vote of Maintainers and Core Maintainers.
    * Is supportive of new and occasional contributors and helps get useful PRs in shape to merge
* Additional privileges:
    * Has rights to approve pull requests in the Podman project or a subproject, marking them as ready for a Maintainer to review and merge
    * Can recommend and review other contributors to become Reviewers
    * Has permissions to change labels on GitHub to aid in triage
	* For repositories hosted on GitHub, Reviewers receive Triage privileges by being added to the `<repo-name>-reviewers` team.
      They also will be invited to the `podman-container-tools` GitHub organization as member.

#### The process of becoming a Reviewer is:
1. The contributor must be sponsored by a Maintainer. That sponsor will open a PR against the appropriate repository, which adds the nominee to the [MAINTAINERS.md](./MAINTAINERS.md) file as a reviewer.
2. The contributor will add a comment to the pull request indicating their willingness to assume the responsibilities of a Reviewer.
3. At least two Maintainers of the repository must concur to merge the PR.

### Maintainer
Description: Maintainers are established contributors with deep technical knowledge of the Podman project and/or one of its subprojects. Maintainers are granted the authority to merge pull requests, and are expected to participate in making decisions about the strategy and priorities of the project. Maintainers are responsible for code review and merging in a single repository or subproject. It is possible to become Maintainer of additional repositories or subprojects, but each additional repository or project will require a separate application and vote. They are able to participate in all maintainer activities, including Core Maintainer meetings, but do not have a vote at Core Maintainer meetings.

A Maintainer must meet the responsibilities and requirements of a Reviewer, plus:
* Responsibilities include:
    * Sustained high level of reviews of pull requests to the project or subproject, with a goal of one or more a week when averaged across the year.
    * Merging pull requests which pass review
    * Mentoring new Reviewers
    * Participating in CNCF maintainer activities for the projects they are maintainers of
    * Assisting Core Maintainers in determining strategy and policy for the project
    * Participating in, and leading, community meetings
* Requirements
    * Experience as a Reviewer for at least 6 months, or status as an Emeritus Maintainer. The time requirement may be overridden by a supermajority (66%) vote of Maintainers and Core Maintainers.
    * Demonstrates a broad knowledge of the project or one or more of its subprojects
    * Is able to exercise judgment for the good of the project, independent of their employer, friends, or team
    * Mentors contributors, reviewers, and new maintainers
    * Collaborates with other Maintainers to work on complex contributions
    * Can commit to maintaining a high level of contribution to the project or one of its subprojects
* Additional privileges:
    * Represent the project in public as a senior project member
    * Represent the project in interactions with the CNCF
    * Have a voice, but not a vote, in Core Maintainer decision-making meetings
	* For repositories hosted on GitHub, Maintainers receive Maintain privileges by default by being added to the `<repo-name>-maintainers` team and removed from the `<repo-name>-reviewers` teams.
      If a Maintainer needs or wants to work on the CI/CD pipelines and requires elevated access to manage things like secrets or the github runners they
      can petition a Core Maintainer to be granted these additional privileges in GitHub. Depending on what kind of access is required they can be given
      the "CI/CD Admin" *Organization* role which enables them access to organization wide runners or secret configuration or they can get the "CI/CD Admin" *Repository* role (must be granted per repository) to access the repository level runners or secrets.

#### Process of becoming a maintainer:
1. A current reviewer must be sponsored by a Maintainer of the repository in question or a Core Maintainer. The Maintainer or Core Maintainer will open a PR against the repository and add the nominee as a Maintainer in the [MAINTAINERS.md](./MAINTAINERS.md) file. The need for a sponsor is removed for Emeritus Maintainers, who may open this pull request themselves.
2. The nominee will add a comment to the PR confirming that they agree to all requirements and responsibilities of becoming a Maintainer.
3. A majority of the current Maintainers of the repository or subproject (including Core Maintainers) must then approve the PR. The need for a majority is removed for Emeritus Maintainers, who require only 2 current Maintainers or Core Maintainers to approve their return.

### Core Maintainer
Description: As the Podman project is composed of a number of subprojects, most maintainers will not have full knowledge of the full project and all its technical aspects. Those that do are eligible to become Core Maintainers, responsible for decisions affecting the entire project. Core Maintainers may act as a maintainer in all repositories and subprojects of the Podman Project. It is recognized that fulfilling all responsibilities of a maintainer on all project repositories is an excessive time commitment, so Core Maintainers are encouraged to choose one repository to specialize in and to spend most of their time working in that repository. Core Maintainers are encouraged to assist other repositories that require additional reviews as time allows, and should make an effort to review pull requests in other repositories that will affect multiple repositories (especially ones that will effect the repository they have chosen to specialize in).

* Responsibilities include:
    * All responsibilities of a maintainer on a single repository
    * Determining strategy and policy for the project
* Requirements
    * Experience as a Maintainer for at least 3 months
    * Demonstrates a broad knowledge of all components, repositories, and subprojects of the Podman project.
    * Is able to exercise judgment for the good of the project, independent of their employer, friends, or team
    * Mentors new Maintainers and Core Maintainers
    * Able to make decisions and contributions affecting the whole project, including multiple subprojects and repositories
    * Can commit to maintaining a high level of contribution to the project as a whole
* Additional privileges:
    * Merge privileges on all repositories in the project
    * Represent the project in public as a senior project member
    * Represent the project in interactions with the CNCF
    * Have a vote in Core Maintainer decision-making meetings
	* For all repositories hosted on GitHub, Core Maintainers receive Admin privileges by being added as `podman-container-tools` GitHub organization Owner.
    * For all repositories hosted on GitHub, Core Maintainers will be added to *all* `<repo-name>-maintainers` teams. Separate from the permissions granted
      above, this ensures that Core Maintainers receive pings meant for repository maintainers, as part of their responsibility to assist in code review
      and decision making in all repositories.

#### Process of becoming a Core Maintainer:
1. A current maintainer must be sponsored by Core Maintainer. The Core Maintainer will open a PR against the main Podman repository and add the nominee as a Core Maintainer in the [MAINTAINERS.md](./MAINTAINERS.md) file.
2. The nominee will add a comment to the PR confirming that they agree to all requirements and responsibilities of becoming a Core Maintainer.
3. A majority of the current Core Maintainers must then approve the PR.
4. If, for some reason, all existing members are inactive according to the Inactivity policy below or there are no Core Maintainers due to resignations, a supermajority (66%) vote of maintainers can bypass this process and approve new Core Maintainers directly.

### Community Manager
Description: Community managers are responsible for the project’s community interactions, including project social media, website maintenance, gathering metrics, managing the new contributor process, ensuring documentation is easy to use and welcoming to new users, and managing the project’s interactions with the CNCF. This is a nontechnical role, and as such does not require technical contribution to the project.

* Responsibilities include:
    * Participating in CNCF maintainer activities
    * Arranging, participating in, and leading, community meetings
    * Managing the project website and gathering associated metrics
    * Managing the project’s social media accounts and mailing lists and gathering associated metrics
    * Creating and publishing minutes from Core Maintainer meetings
	* Assisting in triage of upstream issues
* Requirements
    * Sustained high level of contribution to the community, including attending and engaging in community meetings, contributions to the website, and contributions to documentation, for at least six months
    * Is able to exercise judgment for the good of the project, independent of their employer, friends, or team
    * Can commit to maintaining a high level of contribution to the project's community, website, and social media presence
    * Advocates for the community in Maintainer and Core Maintainer meetings
* Additional privileges:
    * Represent the project in public
    * Represent the project in interactions with the CNCF
    * Have a voice, but not a vote, in Core Maintainer decision-making meetings
	* For all repositories hosted on GitHub, Community Managers receive Triage privileges.

#### Process of becoming a Community Manager:
1. Community Managers must be sponsored by a Core Maintainer. The Core Maintainer will open a PR against the main Podman repository and add the nominee as a Community Manager in the [MAINTAINERS.md](./MAINTAINERS.md) file.
2. The nominee will add a comment to the PR confirming that they agree to all requirements and responsibilities of becoming a Community Manager.
3. A majority of the current Core Maintainers must then approve the PR.

### Emeritus Maintainer
Emeritus Maintainers are former Maintainers or Core Maintainers whose status has lapsed, either voluntarily or through inactivity. We recognize that these former maintainers still have valuable experience and insights, and maintain Emeritus status as a way of recognizing this. Emeritus Maintainer also offers a fast-tracked path to becoming a Maintainer again, should the contributor wish to return to the project.

Emeritus Maintainers have no responsibilities or requirements beyond those of an ordinary Contributor.

#### Process of becoming an Emeritus Maintainer:
1. A current Maintainer or Core Maintainer may voluntarily resign from their position by making a pull request changing their role in the MAINTAINERS file. They may choose to remove themselves entirely or to change their role to Emeritus Maintainer.
2. Maintainers and Core Maintainers removed due to the Inactivity policy below may be moved to Emeritus Status.

---

# Maintainers File

The definitive source of truth for maintainers of this repository is the local [MAINTAINERS.md](./MAINTAINERS.md) file. The [MAINTAINERS.md](./MAINTAINERS.md) file in the main Podman repository is used for project-spanning roles, including Core Maintainer and Community Manager.

---

# Inactivity

* Inactivity is measured by one or more of the following:
    * Periods of no contribution of code, pull request review, or participation in issue triage for longer than 12 months
    * Periods of no communication for longer than 3 months
* Consequences of being inactive include:
    * Involuntary removal or demotion
    * Being asked to move to Emeritus status

---

# Involuntary Removal or Demotion

Involuntary removal/demotion of a contributor happens when responsibilities and requirements aren't being met. This may include repeated patterns of inactivity, an extended period of inactivity, a period of failing to meet the requirements of your role, and/or a violation of the Code of Conduct. This process is important because it protects the community and its deliverables while also opening up opportunities for new contributors to step in.

Involuntary removal or demotion of Maintainers and Reviewers is handled through a vote by a majority of the current Maintainers. Core Maintainers may be involuntarily removed by a majority vote of current Core Maintainers or, if all Core Maintainers have stepped down or are inactive according to the inactivity policy, by a supermajority (66%) vote of maintainers.

---

# Stepping Down/Emeritus Process
If and when contributors' commitment levels change, contributors can consider stepping down (moving down the contributor ladder) vs moving to emeritus status (completely stepping away from the project).

Maintainers and Reviewers should contact the Maintainers about changing to Emeritus status, or reducing your contributor level. Core Maintainers should contact other Core Maintainers.

---

# Updates to this document
Updates to this Governance document require approval from a supermajority (66%) vote of the Core Maintainers.

# Contact
* For inquiries, please reach out to:
    *  [Tom Sweeney, Community Manager](tsweeney@redhat.com)
