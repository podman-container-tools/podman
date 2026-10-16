![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Podman Roadmap

The Podman development team reviews feature requests from its various stakeholders for consideration
quarterly.  Podman maintainers then prioritize these features.   Top features are then assigned to
one or more engineers.

Active release milestones and upcoming work are tracked on the public **[Podman Milestones](https://github.com/containers/podman/milestones)** page and the **[Podman Issue Tracker](https://github.com/containers/podman/issues)**.


## Future feature considerations

The following features are of general importance to Podman.  While these features have no timeline
associated with them yet, they will likely be on future quarterly milestones.

* Further improvements to `podman machine` to better support Podman Desktop and other developer usecases.
  - Smoother upgrade process for Podman machine operating system (OS) images
  - Convergence of WSL technologies with other providers including its OS
* Remote client support for OCI artifacts and its RESTful API
* Integration of composefs
* Ongoing work around partial pull support (zstd:chunked)
* Improved support for the BuildKit API.
* Performance and stability improvements.
* Reductions to the size of the Podman binary.

## Milestones and commitments by quarter

This section is a historical account of what features were prioritized by quarter.  Results of the prioritization will be added at start of each quarter (Jan, Apr, July, Oct).


### 2026 Q1 / Q2 ####

#### Releases ####
- [x] Podman 5.7
- [x] Podman 5.8
- [ ] Podman 6.0

#### Features ####
- [x] Continuous improvements to Podman Quadlet and systemd integrations
- [ ] Podman 6.0 architecture updates and breaking change cleanup
- [ ] Further enhancements to `podman machine` across platforms
- [ ] Improvements to rootless user configurations

#### CNCF ####
- [ ] Continue progression towards CNCF graduated status

### 2025 Q3 ####

#### Releases ####
- [x] Podman 5.6
- [x] Podman 6 (Spring 2026) High Level Design

#### Features ####

- [x] Ongoing upgrades to support newer Docker API versions in the RESTful service
- [x] Improvements to Quadlet documentation
- [x] Systemwide rootless user configuration
- [x] Improvements to the Windows installer

#### CNCF ####

- [x] Continue towards incubation

### 2025 Q2 ####

#### Releases ####
- [x] Podman 5.5
- [x] Fully automated Podman releases

#### Features ####
- [x] Windows ARM64 installer
- [x] Add support for artifacts in RESTful service
- [x] Reduce binary size of Podman
- [x] Add remote client support for artifacts
- [x] Add support for newer Docker API versions to RESTful service
- [x] Replace Podman pause image with a rootfs

#### CNCF ####
- [x] Add and adhere to Governance model

### 2025 Q1 ####

#### Releases ####
- [x] Podman 5.4
- [x] Podman release automation

#### Features ####
- [x] Artifact add --append
- [x] Artifact extract
- [x] Artifact add --options
- [x] Mount OCI artifacts inside containers
- [x] Determine strategy for configuration files when remote

#### CNCF ####
- [x] Create Maintainers file
- [x] Create Governance documentation
