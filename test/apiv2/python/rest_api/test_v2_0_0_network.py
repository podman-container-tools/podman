import random
import unittest

import requests

from .fixtures import APITestCase


class NetworkTestCase(APITestCase):
    # TODO Need to support Docker-py order of network/container creates
    def test_connect(self):
        """Create network and container then connect to network"""
        net_default = requests.post(
            self.podman_url + "/v1.40/networks/create", json={"Name": "TestDefaultNetwork"}
        )
        self.assertEqual(net_default.status_code, 201, net_default.text)
        net_id = net_default.json()["Id"]

        create = requests.post(
            self.podman_url + "/v1.40/containers/create?name=postCreateConnect",
            json={
                "Cmd": ["top"],
                "Image": "alpine:latest",
                "NetworkDisabled": False,
            },
        )
        self.assertEqual(create.status_code, 201, create.text)
        self.assertId(create.content)

        payload = create.json()
        start = requests.post(self.podman_url + f"/v1.40/containers/{payload['Id']}/start")
        self.assertEqual(start.status_code, 204, start.text)

        connect = requests.post(
            self.podman_url + "/v1.40/networks/TestDefaultNetwork/connect",
            json={"Container": payload["Id"]},
        )
        self.assertEqual(connect.status_code, 200, connect.text)
        self.assertEqual(connect.text, "OK\n")

        inspect = requests.get(f"{self.podman_url}/v1.40/containers/{payload['Id']}/json")
        self.assertEqual(inspect.status_code, 200, inspect.text)

        payload = inspect.json()
        self.assertFalse(payload["Config"].get("NetworkDisabled", False))

        self.assertEqual(
            net_id,
            payload["NetworkSettings"]["Networks"]["TestDefaultNetwork"]["NetworkID"],
        )
        # TODO restore this to test, when joining multiple networks possible
        # self.assertEqual(
        #     "TestNetwork",
        #     payload["NetworkSettings"]["Networks"]["TestNetwork"]["NetworkID"],
        # )
        # TODO Need to support network aliases
        # self.assertIn(
        #     "test_post_create",
        #     payload["NetworkSettings"]["Networks"]["TestNetwork"]["Aliases"],
        # )

    def test_create(self):
        """Create network and connect container during create"""
        net = requests.post(
            self.podman_url + "/v1.40/networks/create", json={"Name": "TestNetwork"}
        )
        self.assertEqual(net.status_code, 201, net.text)
        net_id = net.json()["Id"]

        create = requests.post(
            self.podman_url + "/v1.40/containers/create?name=postCreate",
            json={
                "Cmd": ["date"],
                "Image": "alpine:latest",
                "NetworkDisabled": False,
                "HostConfig": {"NetworkMode": "TestNetwork"},
            },
        )
        self.assertEqual(create.status_code, 201, create.text)
        self.assertId(create.content)

        payload = create.json()
        inspect = requests.get(f"{self.podman_url}/v1.40/containers/{payload['Id']}/json")
        self.assertEqual(inspect.status_code, 200, inspect.text)

        payload = inspect.json()
        self.assertFalse(payload["Config"].get("NetworkDisabled", False))
        self.assertEqual(
            net_id,
            payload["NetworkSettings"]["Networks"]["TestNetwork"]["NetworkID"],
        )
    def test_inspect(self):
        name = f"Network_{random.getrandbits(160):x}"
        create = requests.post(self.podman_url + "/v1.40/networks/create", json={"Name": name})
        self.assertEqual(create.status_code, 201, create.text)
        self.assertId(create.content)

        net = create.json()
        self.assertIsInstance(net, dict)
        self.assertNotEqual(net["Id"], name)
        ident = net["Id"]

        ls = requests.get(self.podman_url + "/v1.40/networks")
        self.assertEqual(ls.status_code, 200, ls.text)

        networks = ls.json()
        self.assertIsInstance(networks, list)

        found = False
        for net in networks:
            if net["Name"] == name:
                found = True
                break
        self.assertTrue(found, f"Network '{name}' not found")

        inspect = requests.get(self.podman_url + f"/v1.40/networks/{ident}?verbose=false&scope=local")
        self.assertEqual(inspect.status_code, 200, inspect.text)


    def test_crud(self):
        name = f"Network_{random.getrandbits(160):x}"

        # Cannot test for 0 existing networks because default "podman" network always exists

        create = requests.post(self.podman_url + "/v1.40/networks/create", json={"Name": name})
        self.assertEqual(create.status_code, 201, create.text)
        self.assertId(create.content)

        net = create.json()
        self.assertIsInstance(net, dict)
        self.assertNotEqual(net["Id"], name)
        ident = net["Id"]

        ls = requests.get(self.podman_url + "/v1.40/networks")
        self.assertEqual(ls.status_code, 200, ls.text)

        networks = ls.json()
        self.assertIsInstance(networks, list)

        found = False
        for net in networks:
            if net["Name"] == name:
                found = True
                break
        self.assertTrue(found, f"Network '{name}' not found")

        inspect = requests.get(self.podman_url + f"/v1.40/networks/{ident}")
        self.assertEqual(inspect.status_code, 200, inspect.text)
        self.assertIsInstance(inspect.json(), dict)

        inspect = requests.delete(self.podman_url + f"/v1.40/networks/{ident}")
        self.assertEqual(inspect.status_code, 204, inspect.text)
        inspect = requests.get(self.podman_url + f"/v1.40/networks/{ident}")
        self.assertEqual(inspect.status_code, 404, inspect.text)

        # network prune
        prune_name = f"Network_{random.getrandbits(160):x}"
        prune_create = requests.post(
            self.podman_url + "/v1.40/networks/create", json={"Name": prune_name}
        )
        self.assertEqual(create.status_code, 201, prune_create.text)

        prune = requests.post(self.podman_url + "/v1.40/networks/prune")
        self.assertEqual(prune.status_code, 200, prune.text)
        self.assertTrue(prune_name in prune.json()["NetworksDeleted"])


if __name__ == "__main__":
    unittest.main()
