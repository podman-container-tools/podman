import unittest

import requests
from .fixtures import APITestCase


class ManifestTestCase(APITestCase):
    def test_manifest_409(self):
        r = requests.post(self.uri("/manifests/create"), params={"name": "ThisIsAnInvalidImage"})
        self.assertEqual(r.status_code, 400, r.text)


if __name__ == "__main__":
    unittest.main()
