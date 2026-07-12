# Change Request

<!--
This template is used to propose and discuss major new features to be added to Podman, Buildah, Skopeo, Netavark, and associated libraries.
The creation of a design document prior to feature implementation is not mandatory, but is encouraged.
Before major features are implemented, a pull request should be opened against the Podman repository with a completed version of this template.
Discussion on the feature will occur in the pull request.
Merging the pull request will constitute approval by project maintainers to proceed with implementation work.
When the feature is completed and merged, this document should be removed to avoid cluttering the repository.
It will remain in the Git history for future retrieval if necessary.
-->

## **Short Summary**

To meet numerous user requests for enhanced logging capabilities and produce more maintainable code, we will restructure and rearchitect Conmon to better suit our future needs

## **Objective**

There has, in the past, been a recognition that Conmon - which was written very quickly to support the early development of CRI-O - is not fit for Podman's purposes.
This spawned Conmon-rs, a Conmon rewrite in Rust spearheaded by the CRI-O maintainers that offered a number of additional features, including the ability to run multiple containers under a single Conmon and an API for remote access.
The promise of a smarter Conmon, written in a language more maintainers are familiar with, was very attractive, but the effort was derailed by the degree of change required to adopt Conmon-rs and a number of problems inherent to its design, which will be detailed below.
The effort to adopt Conmon-rs failed as a result.

With the failure of the first attempt to rework Conmon, we now propose a second attempt, which we are calling Conmon v3.
We will learn from the issues of Conmon-rs by, initially, producing a direct port of the original Conmon’s CLI and API interfaces, built on a fundamentally new architecture built for future expansion.
This will minimize the degree of change required for adoption.
Almost all changes in the initial release will be purely internal.

The first goal and the major focus of this rework will be a plugin-based logging system which will allow us to satisfy numerous user requests about logging improvements to Podman.
The second major goal is to move towards a new architecture better suited to error handling, resolving numerous cases where problems with Conmon do not result in any logs or debug information being forwarded to Podman.
Finally, we aim to produce code that is more understandable and maintainable for our maintainers.
Conmon has seen only minimal maintenance over the past 5 years due to the difficulty in working with the existing codebase, which we aim to solve.
Future work after Podman 6 - which will exclusively use the new Conmon - will expand functionality further and begin evolving the Conmon API and CLI interfaces, but this work is deliberately out of scope for this document to keep the initial implementation as simple as possible.

## **Why not enhance the original Conmon?**

The original Conmon has served Podman for the last 8 years, but it has a number of serious limitations that make us reluctant to continue using it.
1. **Architectural issues**. Conmon was written to support the initial CRI-O efforts almost a decade ago. Long-term maintenance was not an original focus of the design.
2. **Language barrier**. Conmon is written in C, which we have very few maintainers with any experience in, or desire to acquire that experience.
3. **Maintenance issues**. Because of the first two points, Common was effectively unmaintained for an extended period (~4 years), only ending recently when @jnovy stepped up as package maintainer. As a result, a large number of user requests for logging-related features have gone unanswered for extended periods - in some cases, close to 5 years.
4. **Lack of testing**. The existing Conmon has some unit tests, no integration tests, and only basic E2E testing using parts of the CRI-O test suite. We have had a number of regressions in the past because of this (for example: https://github.com/containers/conmon/issues/477).
5. **Error handling**. The existing Conmon architecture does a very poor job of error handling. In many cases, Conmon will simply exit without propagating an error to Podman, resulting in a complete lack of debug information for why a container failed. This causes significant difficulty assisting users as they attempt to debug container start issues.
6. **Maintainer pool**. A rewrite of Conmon in a language more of our maintainers are familiar with would allow us to greatly increase the pool of contributors available and increase the pace of feature development.

There is a perception at present that Conmon is battle-proven, safe, and stable.
I would argue that is not a property of the codebase, but the fact it received almost no changes for an extended period of time due to being effectively unmaintained.
Soon after we began maintaining it again and merging changes to the project, serious problems - such as https://github.com/containers/conmon/issues/477 - began to emerge.
Some of these are not being caught before the project releases because of a lack of testing.
If we cannot ignore user requests for logging improvements any more - and I believe that we cannot - then proceeding as we have is no longer viable.

That being said, continuing to use the existing Conmon is possible.
We have enough maintainers with C experience available that a refactor of Conmon to include some of the features mentioned below - specifically pluggable logging - is possible.
I would consider this a backup plan if we determine that developing a replacement is not worthwhile, but it does not address many of the serious issues with the current Conmon.
In particular, remedying the lack of testing is necessary for continued use if we are going to begin adding features again, and would be a substantial amount of work to add to the existing project.

## **Why not use Conmon-rs?**

A core question here is why we should not make appropriate changes to Conmon-rs to enable the usage we desire, instead of producing an alternative.
We find the following problems with Conmon-rs:
1. Too **complex**. The decision to implement a full GRPC API adds significant utility, but Podman doesn’t need most of that functionality.
2. **Resource consumption** is too high. It was originally intended to have 1 Conmon-rs per pod in CRI-O, which meant that higher resource utilization was acceptable. However, this is not possible in Podman - a 1:1 conmon:container relationship is necessary for systemd integration and Quadlets, one of our major features. Further, a 1:1 conmon:container relationship ensures a vulnerability in Conmon - whose purpose is to process potentially unsafe data from within the container - can only affect the container in question, and not spread to other containers. Resource consumption in general is a problem for Podman in edge environments, one of our more prominent use-cases. We do not believe there is a technical solution to this without a major rework of Conmon-rs and the abandonment of some features including the GRPC API.
3. Too **different** from existing Conmon. The degree of change required to integrate Conmon-rs into Podman has proven a barrier to adoption.

For these reasons, we do not believe it is viable to use Conmon-rs as a Conmon replacement.
If we decide not to go ahead with a rework of Conmon, continuing to use the original Conmon is a better option than using Conmon-rs.
The large resource overhead of Conmon-rs is the largest reason for this.

## **Detailed Description:**

Our preferred solution is a reimplementation of Conmon in a language which more of our maintainers are familiar with.
This language must not be garbage-collected to maintain a memory utilization broadly similar to that of the existing Conmon - which we have estimated to use 600kb of ram per container.
This excludes common languages like Go and Python, and leaves us with only two realistic options: C and Rust.
C would have the same limited maintainer pool as the existing Conmon, and thus we prefer Rust.

To attempt to avoid the growth in size and resource utilization that makes conmon-rs unsuitable, we must be very careful.
Rust is a static-linked language, which means every package we include adds to binary size (which is not directly correlated to memory usage, but does increase the time required to start containers as exec calls will take longer).
By using as few libraries as possible (and selecting for small size and minimal dependency set when we use them) we believe we can minimize both memory usage and binary size.
If this proves insufficient, we can remove the Rust standard library altogether and build against the C standard library.
We believe we can keep memory usage to approximately 1.2-1.5MB of ram per Conmon - 2 to 2.5 times as much as the original Conmon, but still a very, very low number.
We are also considering whether it is possible to use no_alloc to further restrict dynamic memory allocation and ensure that memory usage does not grow at runtime.

Additional resource utilization from a rewritten Conmon could prove a problem for low-resource edge environments.
We have a proposal for a separate solution to this problem which will completely eliminate Conmon.
This will reduce Podman’s already-minimal at-rest memory consumption and CPU utilization even further by leveraging Systemd to manage containers directly.
The cost will be reduced functionality in some areas - attaching to containers, logging, etc - but from our understanding, these will largely be acceptable for edge use-cases.
This will not be covered in this design document, and we expect to implement it at some point in the next few years regardless of whether Conmon v3 is implemented.

Architecturally, we believe that a very simple event loop is the best way to ensure resource utilization remains low.
By avoiding async code and the Tokio library and instead directly using the C epoll API, we can write a very minimal event loop with few to no external dependencies.
This will undo some of the advantages of Rust in familiarity and code quality, but given the limited goals of Conmon v3, this should not be a serious issue; the event loop will remain quite simple as well.
We will focus on handling the container’s STDOUT and STDERR (copy to log driver, copy to all active attach sessions) and exec sessions’s STDIN (copy to container) in this loop.
We can investigate whether adding process reaping to the loop makes sense or whether we should do that in a separate thread; both ideas have benefits.
Specifically for reaping, we should also investigate using PIDFDs instead of simply waiting for SIGCHLD - particularly as we might consider moving additional processes, e.g. container exec sessions, under the same Conmon.
We should also consider limiting the maximum number of simultaneous attach sessions.
The original Conmon allows an arbitrary number of `podman attach` commands against a single container, but this does not appear to be common knowledge (I helped write the podman attach code and I had forgotten this feature existed until today).
If we limit maximum simultaneous attach sessions to, perhaps, 128, we can simplify code (statically-sized array, conducive to enable no_alloc) while still preserving just about any valid use-case we’re aware of.

It is important to recognize that Conmon is a tool built to handle untrusted data from a container and process it; this means it must be built carefully to avoid vulnerabilities where malicious data from the container could affect it, potentially causing a crash or, worse, a container escape.
The natural memory safety of Rust should be useful here, but care must still be taken to ensure security.
Integration of additional security capabilities to further confine Conmon - potentially dropping capabilities or adding Seccomp profiles - can be investigated after initial completion to improve security.

For the logging plugins, dynamically-linked plugins are our preferred solution, but we do not have enough data to say this is the direction we want to take.
Other alternatives require separate processes and some form of IPC, which could increase our resource utilization, conflicting with our other goals.
However, using plugins in separate processes with RPC communication would be far easier to implement; a comparative evaluation could help decide, as the benefits of dynamic-linked plugins could be too small to justify the work required.
As Rust cannot handle dynamic linking natively, dynamic-linked plugins would require a C API that Rust can call via FFI.
The API can be quite simple, as logging plugins simply need to be initialized (accepting an arbitrarily-sized array of arguments for configuration) and write logs.
Backpressure from the log driver is a consideration, but given the event-loop architecture, we do not believe implementing this will be a serious issue.
There should only be a single log plugin active at a time in the initial implementation, for simplicity and compatibility with the CLI of existing Conmon; we can reevaluate this decision and expand to multiple simultaneous drivers at a later date if we determine there is a real need.
This will also not include Podman work to enable reading the logs with `podman logs`.
Similar to the existing `none` logs driver, plugins will not have Podman integration and thus logs will be unable to be read from Podman.
This matches Docker’s behavior.

We will likely integrate basic logging functionality (any drivers that are supported directly by Podman) in Conmon v3 without use of plugins, as these drivers must act in concert with Podman code to properly display logs.
These will receive additional testing given their status as preferred drivers with full Podman integration.
These drivers should be the existing `passthrough`, `k8s-file`, and `journald` drivers.
We may also consider adding a Docker-compatible `json-file` driver as a directly-integrated, fully-supported driver.
Finally, we should enhance these preferred drivers according to user requests when sensible - particularly, integration of features like log rotation.

For user interface, the initial version of Conmon v3 will aim to be a 1 for 1 copy that can use the existing Podman code for interacting with Conmon without changes.
We do not see this remaining the case for long; once Podman depends on Conmon v3, we can begin incrementing the version required, with each new version bringing new functionality to replace areas of Conmon we have issues - for example, the overly-verbose CLI arguments, or the need for a new Conmon for every exec session.
We can deprecate old interfaces as they are replaced, then remove deprecated functionality with the release of Podman 7.0.
We expect that we will likely need to lockstep Podman and Conmon releases (Podman 6.0 and Conmon 3.0, 6.1 and 3.1, etc) to take advantage of functionality as it is added.

As a future improvement, we desire to consolidate `podman exec` to use the existing Conmon session of the container the exec session was launched into.
This offers opportunity for substantial simplification (particularly, cleaning up a stopped container with exec sessions running - which would, at present, require 1 `podman container cleanup` per exec session, but could be simplified down to just 1 per container) but will also make Conmon v3 more complicated, as we will need to expose an API of some sort to allow creating these new exec sessions.
This is not proposed as part of the initial Conmon v3 work; we should instead investigate the costs and benefits as future work once Conmon v3 is included in Podman.

As another future improvement, adding the ability to perform healthchecks - including startup healthchecks - to Conmon v3 would offer notable benefits.
Firstly, it would allow us to run healthchecks in environments without systemd present - e.g. when Podman is used inside a container.
Secondly, it would remove the need for managing a systemd timer service for each container with a healthcheck, simplifying logic.
We do not consider this essential for initial release, but it is unlikely to add significant resource utilization and would be a strong candidate for future integration.

To ensure that Conmon v3 does not regress on functionality relative to Conmon, we will need to implement a robust test suite for the original Conmon (which has relatively few tests at present), which we can then use to verify the new code once implemented.
This is a very good idea regardless of whether Conmon v3 is implemented (our lack of testing in Conmon is already beginning to bite as we restart development work there), but is a necessary precondition to serious implementation work on a new Conmon.
This work can proceed independent of the main Conmon v3 effort.
I propose that this effort focus on a set of tests written in BATS (ensuring compatibility with multiple languages) which validate all core Conmon functionality with as few external dependencies as possible - not using the Go interface for Conmon whenever possible, as an example.
As part of these, I would propose removing direct reverse-dependency testing for tools like CRI-O and Podman from the Conmon repo, and instead trusting that to the projects in question; the Conmon repo, upstream, should focus on the new BATS test suite which validates core behavior.

## **Use cases**

Improved logging and error handling for all Podman containers (and, potentially, CRI-O containers)

## **Target Podman Release**

Podman 6.0, Spring 2026

## **Link(s)**

- https://github.com/containers/podman/issues/6377

## **Stakeholders**

- [X] Podman Users
- [X] Podman Developers
- [ ] Buildah Users
- [ ] Buildah Developers
- [ ] Skopeo Users
- [ ] Skopeo Developers
- [ ] Podman Desktop
- [X] CRI-O
- [ ] Storage library
- [ ] Image library
- [ ] Common library
- [ ] Netavark and aardvark-dns

## ** Assignee(s) **

@mheon, @ashley-cui, @jankaluza

## **Impacts**

### **CLI**

No changes should be required.

### **Libpod**

Usage of the existing Conmon interface code means changes should be kept minimal.
It is possible that logging options code may be required to change as we expand the list of supported log drivers.

### **Others**

Conmon (original) will be deprecated in podman.

## **Further Description (Optional):**

<!--
Is there anything not covered above that needs to be mentioned?
-->

## **Test Descriptions (Optional):**

Tests must first be written for the original Conmon, covering all its core functionality - including logging, attach, exit files, the cleanup process, and exec sessions.
These should be E2E tests.
They must all pass for the existing code, and will be used to ensure compatibility between Conmon and Conmon v3.
Once a comprehensive test suite exists, we can use it to test both legacy Conmon and Conmon v3.

In addition to these E2E tests, we can add unit tests to critical functionality in the Rust code as it is written.
