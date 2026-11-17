![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# How to use libpod for custom/derivative projects

libpod today is a Golang library and a CLI.  The choice of interface you make has advantages and disadvantages.

Using the REST API
---

Advantages:

 - Stable, versioned API
 - Language-agnostic
 - [Well-documented](http://docs.podman.io/en/latest/_static/api.html) API

Disadvantages:

 - Error handling is less verbose than Golang API
 - May be slower

Running as a subprocess
---

Advantages:

 - Many commands output JSON
 - Works with languages other than Golang
 - Easy to get started

Disadvantages:

 - Error handling is harder
 - May be slower
 - Can't hook into or control low-level things like how images are pulled

Vendoring into a Go project
---

Advantages:

 - Significant power and control

Disadvantages:

 - You are now on the hook for container runtime security updates (partially, `runc`/`crun` are separate)
 - Binary size
 - Potential skew between multiple libpod versions operating on the same storage can cause problems

Making the choice
---

A good question to ask first is: Do you want users to be able to use `podman` to manipulate the containers created by your project?
If so, that makes it more likely that you want to run `podman` as a subprocess or using the HTTP API.  If you want a separate image store and a fundamentally
different experience; if what you're doing with containers is quite different from those created by the `podman` CLI,
that may drive you towards vendoring.
