import json
import os
import random
import unittest

import requests
from .fixtures import APITestCase


class VolumeTestCase(APITestCase):
    def test_volume_crud(self):
        name = f"Volume_{random.getrandbits(160):x}"

        ls = requests.get(self.podman_url + "/v1.40/volumes")
        self.assertEqual(ls.status_code, 200, ls.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/VolumeList
        required_keys = (
            "Volumes",
            "Warnings",
        )

        volumes = ls.json()
        self.assertIsInstance(volumes, dict)
        for key in required_keys:
            self.assertIn(key, volumes)

        create = requests.post(self.podman_url + "/v1.40/volumes/create", json={"Name": name})
        self.assertEqual(create.status_code, 201, create.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/VolumeCreate
        # and https://docs.docker.com/engine/api/v1.40/#operation/VolumeInspect
        required_keys = (
            "Name",
            "Driver",
            "Mountpoint",
            "Labels",
            "Scope",
            "Options",
        )

        volume = create.json()
        self.assertIsInstance(volume, dict)
        for k in required_keys:
            self.assertIn(k, volume)
        self.assertEqual(volume["Name"], name)

        inspect = requests.get(self.podman_url + f"/v1.40/volumes/{name}")
        self.assertEqual(inspect.status_code, 200, inspect.text)

        volume = inspect.json()
        self.assertIsInstance(volume, dict)
        for k in required_keys:
            self.assertIn(k, volume)

        rm = requests.delete(self.podman_url + f"/v1.40/volumes/{name}")
        self.assertEqual(rm.status_code, 204, rm.text)

        # Recreate volume with data and verify default prune behavior.
        r = requests.post(self.podman_url + "/v1.40/volumes/create", json={"Name": name})
        self.assertEqual(r.status_code, 201, r.text)

        create = r.json()
        with open(os.path.join(create["Mountpoint"], "test_prune"), "w") as file:
            file.writelines(["This is a test\n", "This is a good test\n"])

        prune = requests.post(self.podman_url + "/v1.40/volumes/prune")
        self.assertEqual(prune.status_code, 200, prune.text)

        payload = prune.json()
        self.assertNotIn(name, payload["VolumesDeleted"])

        # Current Docker/Podman behavior: named volumes are pruned when
        # the all=true filter is set.
        prune_all = requests.post(
            self.podman_url + "/v1.40/volumes/prune",
            params={"filters": json.dumps({"all": {"true": True}})},
        )
        self.assertEqual(prune_all.status_code, 200, prune_all.text)

        payload_all = prune_all.json()
        self.assertIn(name, payload_all["VolumesDeleted"])
        self.assertGreater(payload_all["SpaceReclaimed"], 0)

    def test_volume_label(self):
        name = f"Volume_{random.getrandbits(160):x}"
        expected = {
            "Production": "False",
            "Database": "Foxbase",
        }

        create = requests.post(
            self.podman_url + "/v4.0.0/libpod/volumes/create",
            json={"name": name, "label": expected},
        )
        self.assertEqual(create.status_code, 201, create.text)

        inspect = requests.get(self.podman_url + f"/v4.0.0/libpod/volumes/{name}/json")
        self.assertEqual(inspect.status_code, 200, inspect.text)

        volume = inspect.json()
        self.assertIn("Labels", volume)
        self.assertNotIn("Label", volume)
        self.assertDictEqual(expected, volume["Labels"])

    def test_volume_labels(self):
        name = f"Volume_{random.getrandbits(160):x}"
        expected = {
            "Production": "False",
            "Database": "Foxbase",
        }

        create = requests.post(
            self.podman_url + "/v4.0.0/libpod/volumes/create",
            json={"name": name, "labels": expected},
        )
        self.assertEqual(create.status_code, 201, create.text)

        inspect = requests.get(self.podman_url + f"/v4.0.0/libpod/volumes/{name}/json")
        self.assertEqual(inspect.status_code, 200, inspect.text)

        volume = inspect.json()
        self.assertIn("Labels", volume)
        self.assertDictEqual(expected, volume["Labels"])

    def test_volume_label_override(self):
        name = f"Volume_{random.getrandbits(160):x}"
        create = requests.post(
            self.podman_url + "/v4.0.0/libpod/volumes/create",
            json={
                "Name": name,
                "Label": {
                    "Database": "dbase",
                },
                "Labels": {
                    "Database": "sqlserver",
                },
            },
        )
        self.assertEqual(create.status_code, 201, create.text)

        inspect = requests.get(self.podman_url + f"/v4.0.0/libpod/volumes/{name}/json")
        self.assertEqual(inspect.status_code, 200, inspect.text)

        volume = inspect.json()
        self.assertIn("Labels", volume)
        self.assertNotIn("Label", volume)
        self.assertDictEqual({"Database": "sqlserver"}, volume["Labels"])


if __name__ == "__main__":
    unittest.main()
