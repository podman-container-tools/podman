"""
Helpers for integration tests using DockerClient
"""
import json
import os
import pathlib
import shutil
import subprocess
import tempfile

from docker import DockerClient

from .compat import constant


class PodmanAPI:
    """
    Instances hold the configuration and setup for running podman commands
    """

    def __init__(self):
        """Initialize a Podman instance with global options"""
        binary = os.getenv("PODMAN", "bin/podman")
        self.cmd = [binary, "--storage-driver=vfs"]

        cgroupfs = os.getenv("CGROUP_MANAGER", "systemd")
        self.cmd.append(f"--cgroup-manager={cgroupfs}")

        # No support for tmpfs (/tmp) or extfs (/var/tmp)
        # self.cmd.append("--storage-driver=overlay")

        if os.getenv("PODMAN_PYTHON_TEST_DEBUG"):
            self.cmd.append("--log-level=debug")
            self.cmd.append("--syslog=true")

        self.anchor_directory = tempfile.mkdtemp(prefix="podman_docker_")

        self.image_cache = os.path.join(self.anchor_directory, "cache")
        os.makedirs(self.image_cache, exist_ok=True)

        self.cmd.append("--root=" + os.path.join(self.anchor_directory, "crio"))
        self.cmd.append("--runroot=" + os.path.join(self.anchor_directory, "crio-run"))

        os.environ["CONTAINERS_REGISTRIES_CONF"] = os.path.join(
            self.anchor_directory, "registries.conf"
        )

        # Entry verified by compat/test_system.py
        reg_conf_sfx = """

[[registry.mirror]]
location = "mirror.localhost:5000"

"""

        # Assume developer-mode testing by default
        reg_conf_source_path="./test/registries.conf"

        # When operating in a CI environment, use the local registry server.
        # Ref: https://github.com/containers/automation_images/pull/357
        #      https://github.com/containers/podman/pull/22726
        if os.getenv("CI_USE_REGISTRY_CACHE"):
            reg_conf_source_path = "./test/registries-cached.conf"

        with open(os.path.join(reg_conf_source_path)) as file:
            conf = file.read() + reg_conf_sfx

        with open(os.environ["CONTAINERS_REGISTRIES_CONF"], "w") as file:
            file.write(conf)

    def open(self, command, *args, **kwargs):
        """Podman initialized instance to run a given command

        :param self: Podman instance
        :param command: podman sub-command to run
        :param args: arguments and options for command
        :param kwargs: See subprocess.Popen() for shell keyword
        :return: subprocess.Popen() instance configured to run podman instance
        """
        cmd = self.cmd.copy()
        cmd.append(command)
        cmd.extend(args)

        shell = kwargs.get("shell", False)

        # pylint: disable=consider-using-with
        return subprocess.Popen(
            cmd,
            shell=shell,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

    def run(self, command, *args, **kwargs):
        """Podman initialized instance to run a given command

        :param self: Podman instance
        :param command: podman sub-command to run
        :param args: arguments and options for command
        :param kwargs: See subprocess.Popen() for shell and check keywords
        :return: subprocess.Popen() instance configured to run podman instance
        """
        cmd = self.cmd.copy()
        cmd.append(command)
        cmd.extend(args)

        check = kwargs.get("check", False)
        shell = kwargs.get("shell", False)

        return subprocess.run(
            cmd,
            shell=shell,
            check=check,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    def tear_down(self):
        """Delete test environment."""
        shutil.rmtree(self.anchor_directory, ignore_errors=True)

    def restore_image_from_cache(self, client: DockerClient):
        """Populate images from cache."""
        path = os.path.join(self.image_cache, constant.ALPINE_TARBALL)
        if not os.path.exists(path):
            img = client.images.pull(constant.ALPINE)
            with open(path, mode="wb") as tarball:
                for frame in img.save(named=True):
                    tarball.write(frame)
        else:
            self.run("load", "-i", path, check=True)

    def flush_image_cache(self):
        """Delete image cache."""
        for file in pathlib.Path(self.image_cache).glob("*.tar"):
            file.unlink(missing_ok=True)
