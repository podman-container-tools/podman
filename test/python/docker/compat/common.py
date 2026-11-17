"""
Fixtures and Helpers for unittests.
"""
import subprocess
import sys
import time
import unittest

# pylint: disable=no-name-in-module,import-error,wrong-import-order
from docker import DockerClient

from test.python.docker import PodmanAPI
from test.python.docker.compat import constant


def run_top_container(client: DockerClient):
    """Run top command in an alpine container."""
    ctnr = client.containers.create(
        constant.ALPINE,
        command="top",
        detach=True,
        tty=True,
        name="top",
    )
    ctnr.start()
    return ctnr.id


def remove_all_containers(client: DockerClient):
    """Delete all containers from the Podman service."""
    for ctnr in client.containers.list(all=True):
        ctnr.remove(force=True)


def remove_all_images(client: DockerClient):
    """Delete all images from the Podman service."""
    for img in client.images.list():
        # FIXME should DELETE /images accept the sha256: prefix?
        id_ = img.id.removeprefix("sha256:")
        client.images.remove(id_, force=True)


class DockerTestCase(unittest.TestCase):
    """Specialized TestCase class for testing against Podman service."""

    podman: PodmanAPI = None  # initialized podman configuration for tests
    service: subprocess.Popen = None  # podman service instance

    top_container_id: str = None
    docker: DockerClient = None

    @classmethod
    def setUpClass(cls) -> None:
        super().setUpClass()

        cls.podman = PodmanAPI()
        super().addClassCleanup(cls.podman.tear_down)

        cls.service = cls.podman.open("system", "service", "tcp:127.0.0.1:8080", "--time=0")
        # give the service some time to be ready...
        time.sleep(2)

        return_code = cls.service.poll()
        if return_code is not None:
            raise subprocess.CalledProcessError(return_code, "podman system service")

    @classmethod
    def tearDownClass(cls) -> None:
        cls.service.terminate()
        stdout, stderr = cls.service.communicate(timeout=0.5)
        if stdout:
            sys.stdout.write("\ndocker-py -- Service Stdout:\n" + stdout.decode("utf-8"))
        if stderr:
            sys.stderr.write("\ndocker-py -- Service Stderr:\n" + stderr.decode("utf-8"))

        return super().tearDownClass()

    def setUp(self) -> None:
        super().setUp()

        self.docker = DockerClient(base_url="tcp://127.0.0.1:8080", timeout=15)
        self.addCleanup(self.docker.close)

        self.podman.restore_image_from_cache(self.docker)
        self.top_container_id = run_top_container(self.docker)
        self.assertIsNotNone(self.top_container_id, "Failed to create 'top' container")

    def tearDown(self) -> None:
        remove_all_containers(self.docker)
        remove_all_images(self.docker)

        super().tearDown()
