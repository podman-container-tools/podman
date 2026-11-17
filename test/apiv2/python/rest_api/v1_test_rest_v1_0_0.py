import json
import os
import shlex
import signal
import string
import subprocess
import sys
import time
import unittest
from collections.abc import Iterable
from multiprocessing import Process

import requests
from dateutil.parser import parse

PODMAN_URL = "http://localhost:8080"


def _url(path):
    return PODMAN_URL + "/v1.0.0/libpod" + path


def podman():
    binary = os.getenv("PODMAN_BINARY")
    if binary is None:
        binary = "bin/podman"
    return binary


def ctnr(path):
    r = requests.get(_url("/containers/json?all=true"))
    try:
        ctnrs = json.loads(r.text)
    except Exception as e:
        sys.stderr.write("Bad container response: {}/{}".format(r.text, e))
        raise e
    return path.format(ctnrs[0]["Id"])


class TestApi(unittest.TestCase):
    podman = None

    def setUp(self):
        super().setUp()
        if TestApi.podman.poll() is not None:
            sys.stderr.write("podman service returned {}", TestApi.podman.returncode)
            sys.exit(2)
        requests.get(_url("/images/create?fromSrc=quay.io%2Flibpod%2Falpine%3Alatest"))
        # calling out to podman is easier than the API for running a container
        subprocess.run(
            [podman(), "run", "alpine", "/bin/ls"],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

    @classmethod
    def setUpClass(cls):
        super().setUpClass()

        TestApi.podman = subprocess.Popen(
            [
                podman(),
                "system",
                "service",
                "tcp://localhost:8080",
                "--log-level=debug",
                "--time=0",
            ],
            shell=False,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        time.sleep(2)

    @classmethod
    def tearDownClass(cls):
        TestApi.podman.terminate()
        stdout, stderr = TestApi.podman.communicate(timeout=0.5)
        if stdout:
            print("\nService Stdout:\n" + stdout.decode("utf-8"))
        if stderr:
            print("\nService Stderr:\n" + stderr.decode("utf-8"))

        if TestApi.podman.returncode > 0:
            sys.stderr.write("podman exited with error code {}\n".format(TestApi.podman.returncode))
            sys.exit(2)

        return super().tearDownClass()

    def test_info(self):
        r = requests.get(_url("/info"))
        self.assertEqual(r.status_code, 200)
        self.assertIsNotNone(r.content)
        _ = json.loads(r.text)

    def test_events(self):
        r = requests.get(_url("/events?stream=false"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsNotNone(r.content)
        for line in r.text.splitlines():
            obj = json.loads(line)
            # Actor.ID is uppercase for compatibility
            _ = obj["Actor"]["ID"]

    def test_containers(self):
        r = requests.get(_url("/containers/json"), timeout=5)
        self.assertEqual(r.status_code, 200, r.text)
        obj = json.loads(r.text)
        self.assertEqual(len(obj), 0)

    def test_containers_all(self):
        r = requests.get(_url("/containers/json?all=true"))
        self.assertEqual(r.status_code, 200, r.text)
        self.validateObjectFields(r.text)

    def test_inspect_container(self):
        r = requests.get(_url(ctnr("/containers/{}/json")))
        self.assertEqual(r.status_code, 200, r.text)
        obj = self.validateObjectFields(r.content)
        _ = parse(obj["Created"])

    def test_stats(self):
        r = requests.get(_url(ctnr("/containers/{}/stats?stream=false")))
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            self.validateObjectFields(r.text)

    def test_delete_containers(self):
        r = requests.delete(_url(ctnr("/containers/{}")))
        self.assertEqual(r.status_code, 204, r.text)

    def test_stop_containers(self):
        r = requests.post(_url(ctnr("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(_url(ctnr("/containers/{}/stop")))
        self.assertIn(r.status_code, (204, 304), r.text)

    def test_start_containers(self):
        r = requests.post(_url(ctnr("/containers/{}/stop")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(_url(ctnr("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

    def test_restart_containers(self):
        r = requests.post(_url(ctnr("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(_url(ctnr("/containers/{}/restart")), timeout=5)
        self.assertEqual(r.status_code, 204, r.text)

    def test_resize(self):
        r = requests.post(_url(ctnr("/containers/{}/resize?h=43&w=80")))
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            self.assertIsNone(r.text)

    def test_attach_containers(self):
        r = requests.post(_url(ctnr("/containers/{}/attach")))
        self.assertIn(r.status_code, (101, 409), r.text)

    def test_logs_containers(self):
        r = requests.get(_url(ctnr("/containers/{}/logs?stdout=true")))
        self.assertEqual(r.status_code, 200, r.text)

    def test_post_create(self):
        self.skipTest("TODO: create request body")
        r = requests.post(_url("/containers/create?args=True"))
        self.assertEqual(r.status_code, 200, r.text)
        json.loads(r.text)

    def test_commit(self):
        r = requests.post(_url(ctnr("/commit?container={}")))
        self.assertEqual(r.status_code, 200, r.text)
        self.validateObjectFields(r.text)

    def test_images(self):
        r = requests.get(_url("/images/json"))
        self.assertEqual(r.status_code, 200, r.text)
        self.validateObjectFields(r.content)

    def test_inspect_image(self):
        r = requests.get(_url("/images/alpine/json"))
        self.assertEqual(r.status_code, 200, r.text)
        obj = self.validateObjectFields(r.content)
        _ = parse(obj["Created"])

    def test_delete_image(self):
        r = requests.delete(_url("/images/alpine?force=true"))
        self.assertEqual(r.status_code, 200, r.text)
        json.loads(r.text)

    def test_pull(self):
        r = requests.post(_url("/images/pull?reference=alpine"), timeout=5)
        self.assertEqual(r.status_code, 200, r.text)
        json.loads(r.text)

    def test_search(self):
        # Had issues with this test hanging when repositories not happy
        def do_search():
            r = requests.get(_url("/images/search?term=alpine"), timeout=5)
            self.assertEqual(r.status_code, 200, r.text)
            json.loads(r.text)

        search = Process(target=do_search)
        search.start()
        search.join(timeout=10)
        self.assertFalse(search.is_alive(), "/images/search took too long")

    def test_ping(self):
        r = requests.get(PODMAN_URL + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.head(PODMAN_URL + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.get(_url("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.get(_url("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)


def validateObjectFields(self, buffer):
    objs = json.loads(buffer)
    if not isinstance(objs, dict):
        for o in objs:
            _ = o["Id"]
    else:
        _ = objs["Id"]
    return objs


if __name__ == "__main__":
    unittest.main()
