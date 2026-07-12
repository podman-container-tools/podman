import random
import unittest

import requests
from .fixtures import APITestCase


class TestApi(APITestCase):
    def test_pod_start_conflict(self):
        """Verify issue #8865"""

        pod_name = list()
        pod_name.append(f"Pod_{random.getrandbits(160):x}")
        pod_name.append(f"Pod_{random.getrandbits(160):x}")

        r = requests.post(
            self.uri("/pods/create"),
            json={
                "name": pod_name[0],
                "no_infra": False,
                "portmappings": [{"host_ip": "127.0.0.1", "host_port": 8889, "container_port": 89}],
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        r = requests.post(
            self.uri("/containers/create"),
            json={
                "pod": pod_name[0],
                "image": "quay.io/libpod/alpine:latest",
                "command": ["top"],
            },
        )
        self.assertEqual(r.status_code, 201, r.text)

        r = requests.post(
            self.uri("/pods/create"),
            json={
                "name": pod_name[1],
                "no_infra": False,
                "portmappings": [{"host_ip": "127.0.0.1", "host_port": 8889, "container_port": 89}],
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        r = requests.post(
            self.uri("/containers/create"),
            json={
                "pod": pod_name[1],
                "image": "quay.io/libpod/alpine:latest",
                "command": ["top"],
            },
        )
        self.assertEqual(r.status_code, 201, r.text)

        r = requests.post(self.uri(f"/pods/{pod_name[0]}/start"))
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.post(self.uri(f"/pods/{pod_name[1]}/start"))
        self.assertEqual(r.status_code, 409, r.text)

        start = r.json()
        self.assertGreater(len(start["Errs"]), 0, r.text)


if __name__ == "__main__":
    unittest.main()
