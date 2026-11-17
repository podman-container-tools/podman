.. include:: includes.rst

Introduction
==================================
Containers_ simplify the production, distribution, discoverability, and usage of applications with all of their dependencies and default configuration files. Users test drive or deploy a new application with one or two commands instead of following pages of installation instructions. Here's how to find your first `Container Image`_::

    podman search docker.io/busybox

Output::

    NAME                                         DESCRIPTION
    docker.io/library/busybox                    Busybox base image.
    docker.io/rancher/busybox
    docker.io/openebs/busybox-client
    docker.io/antrea/busybox
    docker.io/hugegraph/busybox                  test image
    ...

The previous command returned a list of publicly available container images on DockerHub. These container images are easy to consume, but of differing levels of quality and maintenance. Let’s use the first one listed because it seems to be well maintained.

To run the busybox container image, it’s just a single command::

    podman run -it docker.io/library/busybox

Output::

    / #

You can poke around in the busybox container for a while, but you’ll quickly find that running small container with a few Linux utilities in it provides limited value, so exit out::

    exit

There’s an old saying that “nobody runs an operating system just to run an operating system” and the same is true with containers. It’s the workload running on top of an operating system or in a container that’s interesting and valuable.

Sometimes we can find a publicly available container image for the exact workload we’re looking for and it will already be packaged exactly how we want. But, more often than not, there’s something that we want to add, remove, or customize. It can be as simple as a configuration setting for security or performance, or as complex as adding a complex workload. Either way, containers make it fairly easy to make the changes we need.

Container Images aren’t actually images. They are repositories often made up of multiple layers. These layers can easily be added, saved, and shared with others by using a Containerfile_ (Dockerfile). This single file often contains all the instructions needed to build a new container image and can easily be shared with others publicly using tools like GitHub.

Here's an example of how to build a container image from content that resides in a git repository::

    podman build -t hello https://github.com/containers/PodmanHello.git

Once, the image build completes, it’s easy to run the new image from our local cache::

    podman run -it hello

Output::

    !... Hello Podman World ...!

             .--"--.
           / -     - \
          / (O)   (O) \
       ~~~| -=(,Y,)=- |
        .---. /`  \   |~~
     ~/  o  o \~~~~.----. ~~
      | =(X)= |~  / (O (O) \
       ~~~~~~~  ~| =(Y_)=-  |
      ~~~~    ~~~|   U      |~~

    Project:   https://github.com/containers/podman
    Website:   https://podman.io
    Desktop:   https://podman-desktop.io
    Documents: https://docs.podman.io
    YouTube:   https://youtube.com/@Podman
    X/Twitter: @Podman_io
    Mastodon:  @Podman_io@fosstodon.org

Building new images is great, but sharing our work with others lets them review our work, critique how we built them, and offer improved versions. Our newly built `hello` image can be published at quay.io or docker.io to share it with the world. Everything needed to run the `hello` application is provided in the container image. Others can easily pull it down and use it, or make improvements to it.

Standardizing on container images and `Container Registries`_ enable a new level of collaboration through simple consumption. This simple consumption model is possible because every major Container Engine and Registry Server uses the Open Containers Initiative (OCI_) format. This allows users to find, run, build, share and deploy containers anywhere they want. Podman and other `Container Engines`_ like CRI-O, Docker, or containerd can create and consume container images from docker.io, quay.io, an on premise registry or even one provided by a cloud provider. The OCI image format facilitates this ecosystem through a single standard.

For example, if we wanted to share our newly built `hello` container image on quay.io it’s easy. First log in to quay::

    podman login quay.io

Input::

    Username: USERNAME
    Password: ********
    Login Succeeded!

Next, tag the image so that we can push it into our user account::

    podman tag localhost/hello quay.io/USERNAME/hello

Finally, push the image::

    podman push quay.io/USERNAME/hello

Output::

    Getting image source signatures
    Copying blob bf62b9b17289 done   |
    Copying config 17a4bf5a30 done   |
    Writing manifest to image destination

Notice that we pushed one layer to our registry and now it’s available for others to share. Take a quick look::

    podman inspect quay.io/USERNAME/hello

Output::

    [
        {
            "Id": "17a4bf5a301a374771ac66dd09c33d1d765af5265d20d6b4da7ac578381efd87",
            "Digest": "sha256:ee693991b0c8c8c12dfe0e90c25db1b73867e672478fd7a187a2fae31f72531a",
            "RepoTags": [
                "quay.io/USERNAME/hello:latest",
    ...

To summarize, Podman makes it easy to find, run, build and share containers.

* Find: whether finding a container on dockerhub.io or quay.io, an internal registry server, or directly from a vendor, a couple of `podman search`_, and `podman pull`_ commands make it easy
* Run: it's easy to consume pre-built images with everything needed to run an entire application, or start from a Linux distribution base image with the `podman run`_ command
* Build: creating new layers with small tweaks, or major overhauls is easy with `podman build`_
* Share: Podman lets you push your newly built containers anywhere you want with a single `podman push`_ command

For more instructions on use cases, take a look at our :doc:`Tutorials` page.
