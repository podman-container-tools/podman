import json
import unittest
import requests
from dateutil.parser import parse
from .fixtures import APITestCase


class ImageTestCase(APITestCase):
    def test_list(self):
        r = requests.get(self.podman_url + "/v1.40/images/json")
        self.assertEqual(r.status_code, 200, r.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/ImageList
        required_keys = (
            "Id",
            "ParentId",
            "RepoTags",
            "RepoDigests",
            "Created",
            "Size",
            "SharedSize",
            "VirtualSize",
            "Labels",
        )
        images = r.json()
        self.assertIsInstance(images, list)
        for item in images:
            self.assertIsInstance(item, dict)
            for k in required_keys:
                self.assertIn(k, item)

            # Id should be prefixed with sha256: (#11645)
            self.assertIn("sha256:",item['Id'])

    def test_inspect(self):
        r = requests.get(self.podman_url + "/v1.40/images/quay.io/libpod/testimage:20241011/json")
        self.assertEqual(r.status_code, 200, r.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/ImageInspect
        required_keys = (
            "Id",
            "Parent",
            "Comment",
            "Created",
            "DockerVersion",
            "Author",
            "Architecture",
            "Os",
            "Size",
            "VirtualSize",
            "GraphDriver",
            "RootFS",
            "Metadata",
        )

        image = r.json()
        self.assertIsInstance(image, dict)
        for item in required_keys:
            self.assertIn(item, image)
        _ = parse(image["Created"])
        # Id should be prefixed with sha256: (#11645)
        self.assertIn("sha256:",image['Id'])

    def test_delete(self):
        r = requests.delete(self.compat_uri("images/alpine?force=true"))
        self.assertEqual(r.status_code, 409, r.text)

    def test_pull(self):
        def check_response_keys(r, keys_expected):
            text = r.text
            keys_found = set()

            # Read and record stanza's from pull
            for line in str.splitlines(text):
                obj = json.loads(line)
                key_list = list(obj.keys())
                for k in key_list:
                    keys_found.add(k)

            for key, expected in keys_expected.items():
                if expected:
                    negation = ""
                else:
                    negation = "not "
                self.assertEqual(
                    key in keys_found,
                    expected,
                    f'Expected {negation}to find "{key}" stanza in response',
                )

        existing_reference = "alpine"
        non_existing_reference = "quay.io/f4ee35641334/f6fda4bb"
        cases = [
            dict(
                quiet_postfix="&quiet=True",
                reference=existing_reference,
                timeout=15,
                assert_function=self.assertEqual,
                expected_keys={
                    "error": False,
                    "id": True,
                    "images": True,
                    "stream": False,
                },
            ),
            dict(
                quiet_postfix="",
                reference=existing_reference,
                timeout=15,
                assert_function=self.assertEqual,
                expected_keys={
                    "error": False,
                    "id": True,
                    "images": True,
                    "stream": True,
                },
            ),
            dict(
                quiet_postfix="&quiet=True",
                reference=non_existing_reference,
                timeout=None,
                assert_function=self.assertNotEqual,
                expected_keys={
                    "cause": True,
                    "message": True,
                    "response": True,
                },
            ),
            dict(
                quiet_postfix="",
                reference=non_existing_reference,
                timeout=None,
                assert_function=self.assertNotEqual,
                expected_keys={
                    "cause": True,
                    "message": True,
                    "response": True,
                },
            ),
        ]

        for case in cases:
            with self.subTest(case=case):
                r = requests.post(
                    self.uri(f"/images/pull?reference={case['reference']}{case['quiet_postfix']}"),
                    timeout=case["timeout"],
                )
                case["assert_function"](r.status_code, 200, r.status_code)
                check_response_keys(r, case["expected_keys"])

    def test_create(self):
        r = requests.post(
            self.podman_url + "/v1.40/images/create?fromImage=quay.io/libpod/testimage:20241011&platform=linux/amd64/v8",
            timeout=15,
        )
        self.assertEqual(r.status_code, 200, r.text)
        r = requests.post(
            self.podman_url
            + "/v1.40/images/create?fromSrc=-&repo=fedora&message=testing123&platform=linux/amd64",
            timeout=15,
        )
        self.assertEqual(r.status_code, 200, r.text)

    def test_search_compat(self):
        url = self.podman_url + "/v1.44/images/search"

        # Had issues with this test hanging when repositories not happy
        def do_search1():
            required_keys = (
                "description",
                "is_automated",  # Deprecated: always false.
                "is_official",
                "name",
                "star_count",
            )
            payload = {"term": "alpine"}
            r = requests.get(url, params=payload, timeout=30)
            self.assertEqual(r.status_code, 200, f"#1: {r.text}")

            results = r.json()
            self.assertIsInstance(results, list)
            for item in results:
                for k in required_keys:
                    self.assertIn(k, item)

        def do_search2():
            # The containers.conf uses:
            #   compat_api_enforce_docker_hub=false
            # and full name needs to be used here.
            payload = {"term": "docker.io/library/alpine", "limit": 1}
            r = requests.get(url, params=payload, timeout=30)
            self.assertEqual(r.status_code, 200, f"#2: {r.text}")

            results = r.json()
            self.assertIsInstance(results, list)
            self.assertEqual(len(results), 1)

        def do_search3():
            # FIXME: Research if quay.io supports is-official and which image is "official"
            return
            payload = {"term": "thanos", "filters": '{"is-official":["true"]}'}
            r = requests.get(url, params=payload, timeout=30)
            self.assertEqual(r.status_code, 200, f"#3: {r.text}")

            results = r.json()
            self.assertIsInstance(results, list)

            # There should be only one official image
            self.assertEqual(len(results), 1)

        def do_search4():
            headers = {"X-Registry-Auth": "null"}
            payload = {"term": "alpine"}
            r = requests.get(url, params=payload, headers=headers, timeout=30)
            self.assertEqual(r.status_code, 200, f"#4: {r.text}")

        def do_search5():
            headers = {"X-Registry-Auth": "invalid value"}
            payload = {"term": "alpine"}
            r = requests.get(url, params=payload, headers=headers, timeout=30)
            self.assertEqual(r.status_code, 400, f"#5: {r.text}")

        for i, t in enumerate([do_search1, do_search2, do_search3, do_search4, do_search5], start=1):
            with self.subTest(i=i):
                t()

    def test_history(self):
        r = requests.get(self.podman_url + "/v1.40/images/alpine/history")
        self.assertEqual(r.status_code, 200, r.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/ImageHistory
        required_keys = ("Id", "Created", "CreatedBy", "Tags", "Size", "Comment")

        changes = r.json()
        self.assertIsInstance(changes, list)
        for change in changes:
            self.assertIsInstance(change, dict)
            for k in required_keys:
                self.assertIn(k, change)

    def test_tree(self):
        r = requests.get(self.uri("/images/alpine/tree"))
        self.assertEqual(r.status_code, 200, r.text)
        tree = r.json()
        self.assertTrue(tree["Tree"].startswith("Image ID:"), r.text)


if __name__ == "__main__":
    unittest.main()
