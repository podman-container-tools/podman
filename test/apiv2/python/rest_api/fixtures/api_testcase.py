import json
import os
import random
import subprocess
import sys
import time
import unittest

import requests

from .podman import Podman


class ArtifactFile:
    __test__: bool = False

    name: str | None
    size: int | None
    sig: bytes | None

    def __init__(
        self, name: str | None = None, size: int | None = None, sig: bytes | None = None
    ) -> None:
        self.name = name
        self.size = size
        self.sig = sig
        self.render_test_file()

    def render_test_file(self) -> None:
        if self.name is None:
            self.name = "test_file_1"
        if self.size is None:
            self.size = 1048576

        file_data = None
        if self.sig is not None:
            random_bytes = random.randbytes(self.size - len(self.sig))

            file_data = bytearray(self.sig)
            file_data.extend(random_bytes)
        else:
            file_data = os.urandom(self.size)

        try:
            with open(self.name, "wb") as f:
                _ = f.write(file_data)
        except Exception as e:
            print(f"File write error for {self.name}: {e}")
            raise


class Artifact:
    __test__: bool = False

    uri: str
    name: str
    parameters: dict[str, str | list[str]]
    file: ArtifactFile

    def __init__(
        self,
        uri: str,
        name: str,
        parameters: dict[str, str | list[str]],
        file: ArtifactFile,
    ) -> None:
        self.uri = uri
        self.name = name
        self.parameters = parameters
        self.file = file

    def add(self) -> requests.Response:
        try:
            with open(self.file.name, "rb") as file_to_upload:
                file_content = file_to_upload.read()
                r = requests.post(
                    self.uri + "/artifacts/add",
                    data=file_content,
                    params=self.parameters,
                )
        except Exception:
            pass

        os.remove(self.file.name)
        return r

    def add_local(self) -> requests.Response:
        try:
            r = requests.post(
                self.uri + "/artifacts/local/add",
                params=self.parameters,
            )
        except Exception:
            pass

        if self.file is not None and os.path.exists(self.file.name):
            os.remove(self.file.name)
        return r

    def do_artifact_inspect_request(self) -> requests.Response:
        r = requests.get(
            self.uri + "/artifacts/" + self.name + "/json",
        )

        return r


class APITestCase(unittest.TestCase):
    PODMAN_URL = "http://localhost:8080"
    podman = None  # initialized podman configuration for tests
    service = None  # podman service instance

    @classmethod
    def setUpClass(cls):
        super().setUpClass()

        APITestCase.podman = Podman()
        APITestCase.service = APITestCase.podman.open(
            "system", "service", "tcp://localhost:8080", "--time=0"
        )
        # give the service some time to be ready...
        time.sleep(2)

        returncode = APITestCase.service.poll()
        if returncode is not None:
            raise subprocess.CalledProcessError(returncode, "podman system service")

        r = requests.post(
            APITestCase.uri("/images/pull?reference=quay.io%2Flibpod%2Falpine%3Alatest")
        )
        if r.status_code != 200:
            raise subprocess.CalledProcessError(
                r.status_code, f"podman images pull quay.io/libpod/alpine:latest {r.text}"
            )

    @classmethod
    def tearDownClass(cls):
        APITestCase.service.terminate()
        stdout, stderr = APITestCase.service.communicate(timeout=1)
        if stdout:
            sys.stdout.write("\nService Stdout:\n" + stdout.decode("utf-8"))
        if stderr:
            sys.stderr.write("\nService Stderr:\n" + stderr.decode("utf-8"))
        return super().tearDownClass()

    def setUp(self):
        super().setUp()
        APITestCase.podman.run("run", "-d", "alpine", "top", check=True)

    def tearDown(self) -> None:
        APITestCase.podman.run("pod", "rm", "--all", "--force", check=True)
        APITestCase.podman.run("rm", "--all", "--force", check=True)
        super().tearDown()

    @property
    def podman_url(self):
        return "http://localhost:8080"

    @staticmethod
    def uri(path: str) -> str:
        return APITestCase.PODMAN_URL + "/v2.0.0/libpod" + path

    @staticmethod
    def compat_uri(path):
        return APITestCase.PODMAN_URL + "/v3.0.0/" + path

    def resolve_container(self, path):
        """Find 'first' container and return 'Id' formatted into given URI path."""

        try:
            r = requests.get(self.uri("/containers/json?all=true"))
            containers = r.json()
        except Exception as e:
            msg = f"Bad container response: {e}"
            if r is not None:
                msg += ": " + r.text
            raise self.failureException(msg)
        return path.format(containers[0]["Id"])

    def assertContainerExists(self, member, msg=None):  # pylint: disable=invalid-name
        r = requests.get(self.uri(f"/containers/{member}/exists"))
        if r.status_code == 404:
            if msg is None:
                msg = f"Container '{member}' does not exist."
            self.failureException(msg)

    def assertContainerNotExists(self, member, msg=None):  # pylint: disable=invalid-name
        r = requests.get(self.uri(f"/containers/{member}/exists"))
        if r.status_code == 204:
            if msg is None:
                msg = f"Container '{member}' exists."
            self.failureException(msg)

    def assertId(self, content):  # pylint: disable=invalid-name
        objects = json.loads(content)
        try:
            if isinstance(objects, dict):
                _ = objects["Id"]
            else:
                for item in objects:
                    _ = item["Id"]
        except KeyError:
            self.failureException("Failed in find 'Id' in return value.")
