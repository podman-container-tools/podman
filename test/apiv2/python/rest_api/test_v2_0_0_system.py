import json
import unittest
import uuid

import requests
import yaml
from .fixtures import APITestCase


class SystemTestCase(APITestCase):
    def test_info(self):
        r = requests.get(self.uri("/info"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsNotNone(r.content)
        _ = r.json()

        r = requests.get(self.podman_url + "/v1.40/info")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsNotNone(r.content)
        response = r.json()

        info_status = yaml.load(self.podman.run("info").stdout, Loader=yaml.FullLoader)
        if info_status["host"]["security"]["rootless"]:
            self.assertIn("name=rootless", response["SecurityOptions"])
        else:
            self.assertNotIn("name=rootless", response["SecurityOptions"])

        if info_status["host"]["security"]["selinuxEnabled"]:
            self.assertIn("name=selinux", response["SecurityOptions"])
        else:
            self.assertNotIn("name=selinux", response["SecurityOptions"])

    def test_events(self):
        r = requests.get(self.uri("/events?stream=false"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsNotNone(r.content)

        report = r.text.splitlines()
        self.assertGreater(len(report), 0, "No events found!")
        for line in report:
            obj = json.loads(line)
            # Actor.ID is uppercase for compatibility
            self.assertIn("ID", obj["Actor"])
            # Verify 1.22+ deprecated variants are present if current originals are
            if obj["Actor"]["ID"]:
                self.assertEqual(obj["Actor"]["ID"], obj["id"])
            if obj["Action"]:
                self.assertEqual(obj["Action"], obj["status"])
            if obj["Actor"].get("Attributes") and obj["Actor"]["Attributes"].get("image"):
                self.assertEqual(obj["Actor"]["Attributes"]["image"], obj["from"])

    def test_ping(self):
        required_headers = (
            "API-Version",
            "Builder-Version",
            "Docker-Experimental",
            "Cache-Control",
            "Pragma",
            "Pragma",
        )

        def check_headers(req):
            for k in required_headers:
                self.assertIn(k, req.headers)

        r = requests.get(self.podman_url + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "OK")
        check_headers(r)

        r = requests.head(self.podman_url + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "")
        check_headers(r)

        r = requests.get(self.uri("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "OK")
        check_headers(r)

        r = requests.head(self.uri("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "")
        check_headers(r)

    def test_version(self):
        r = requests.get(self.podman_url + "/v1.40/version")
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.get(self.uri("/version"))
        self.assertEqual(r.status_code, 200, r.text)

        body = r.json()
        names = [d.get("Name", "") for d in body["Components"]]

        self.assertIn("Conmon", names)
        for n in names:
            if n.startswith("OCI Runtime"):
                oci_name = n
        self.assertIsNotNone(oci_name, "OCI Runtime not found in version components.")

    def test_df(self):
        r = requests.get(self.podman_url + "/v1.40/system/df")
        self.assertEqual(r.status_code, 200, r.text)

        obj = r.json()
        self.assertIn("Images", obj)
        self.assertIn("Containers", obj)
        self.assertIn("Volumes", obj)
        self.assertIn("BuildCache", obj)

        r = requests.get(self.uri("/system/df"))
        self.assertEqual(r.status_code, 200, r.text)

    def test_reference_id(self):
        rid = str(uuid.uuid4())
        r = requests.get(self.uri("/info"), headers={"X-Reference-Id": rid})
        self.assertEqual(r.status_code, 200, r.text)

        self.assertIn("X-Reference-Id", r.headers)
        self.assertEqual(r.headers["X-Reference-Id"], rid)

        r = requests.get(self.uri("/info"))
        self.assertIn("X-Reference-Id", r.headers)
        self.assertNotEqual(r.headers["X-Reference-Id"], rid)


if __name__ == "__main__":
    unittest.main()
