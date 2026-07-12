"""
Integration tests for exercising docker-py against Podman Service.
"""
import io
import os
import unittest
import json
from typing import Iterator

from docker import errors
from docker.errors import APIError

# pylint: disable=no-name-in-module,import-error,wrong-import-order
from test.python.docker.compat import common, constant


class TestImages(common.DockerTestCase):
    """TestCase for exercising images."""

    def test_tag_valid_image(self):
        """Validates if the image is tagged successfully"""
        alpine = self.docker.images.get(constant.ALPINE)
        self.assertTrue(alpine.tag("demo", constant.ALPINE_SHORTNAME))

        alpine = self.docker.images.get(constant.ALPINE)
        for tag in alpine.tags:
            self.assertIn("alpine", tag)

    def test_retag_valid_image(self):
        """Validates if name updates when the image is re-tagged."""
        alpine = self.docker.images.get(constant.ALPINE)
        self.assertTrue(alpine.tag("demo", "rename"))

        alpine = self.docker.images.get(constant.ALPINE)
        self.assertNotIn("demo:test", alpine.tags)

    def test_list_images(self):
        """List images"""
        self.assertEqual(len(self.docker.images.list()), 1)

        # Add more images
        self.docker.images.pull(constant.BB)
        self.assertEqual(len(self.docker.images.list()), 2)
        self.assertEqual(len(self.docker.images.list(all=True)), 2)

        # List images with filter
        self.assertEqual(len(self.docker.images.list(filters={"reference": "alpine"})), 1)

    def test_search_image(self):
        """Search for image"""
        self.skipTest("FIXME: search seems broken with our new CI and local registry mirror setup")
        for registry in self.docker.images.search("alpine"):
            # registry matches if string is in either one
            self.assertIn("alpine", registry["Name"] + " " + registry["Description"].lower())

    def test_search_bogus_image(self):
        """Search for bogus image should throw exception"""
        with self.assertRaises(errors.APIError):
            self.docker.images.search("bogus/bogus")

    def test_remove_image(self):
        """Remove image"""
        # Check for error with wrong image name
        with self.assertRaises(errors.NotFound):
            self.docker.images.remove("dummy")

        common.remove_all_containers(self.docker)
        self.assertEqual(len(self.docker.images.list()), 1)
        self.docker.images.remove(constant.ALPINE)
        self.assertEqual(len(self.docker.images.list()), 0)

    def test_image_history(self):
        """Image history"""
        img = self.docker.images.get(constant.ALPINE)
        history = img.history()

        found = False
        for change in history:
            found |= img.id in change.values()
        self.assertTrue(found, f"image id {img.id} not found in history")

    def test_get_image_exists_not(self):
        """Negative test for get image"""
        with self.assertRaises(errors.NotFound):
            self.docker.images.get("image_does_not_exists")

    def test_save_image(self):
        """Export Image"""
        image = self.docker.images.pull(constant.BB)

        file = os.path.join(TestImages.podman.image_cache, "busybox.tar")
        with open(file, mode="wb") as tarball:
            for frame in image.save(named=True):
                tarball.write(frame)
        self.assertGreater(os.path.getsize(file), 0)

    def test_load_image(self):
        """Import|Load Image"""
        self.assertEqual(len(self.docker.images.list()), 1)

        image = self.docker.images.pull(constant.BB)
        file = os.path.join(TestImages.podman.image_cache, "busybox.tar")
        with open(file, mode="wb") as tarball:
            for frame in image.save():
                tarball.write(frame)

        with open(file, mode="rb") as saved:
            self.docker.images.load(saved)

        self.assertEqual(len(self.docker.images.list()), 2)

    def test_load_corrupt_image(self):
        """Import|Load Image failure"""
        tarball = io.BytesIO("This is a corrupt tarball".encode("utf-8"))
        with self.assertRaises(errors.APIError):
            self.docker.images.load(tarball)

    def test_build_image(self):
        """Build Image with custom labels."""
        labels = {"apple": "red", "grape": "green"}
        self.docker.images.build(
            path="test/python/docker/build_labels",
            labels=labels,
            tag="labels",
            isolation="default",
        )
        image = self.docker.images.get("labels")
        self.assertEqual(image.labels["apple"], labels["apple"])
        self.assertEqual(image.labels["grape"], labels["grape"])

    def test_build_image_via_api_client(self):
        api_client = self.docker.api
        for line in api_client.build(path="test/python/docker/build_labels"):
            try:
                parsed = json.loads(line.decode("utf-8"))
            except json.JSONDecodeError as e:
                raise IOError(f"Line '{line}' was not JSON parsable")
            assert "errorDetail" not in parsed

    def test_push_error(self):
        """Validates that push error is correctly propagated"""
        alpine = self.docker.images.get(constant.ALPINE)
        if not alpine.tag("non-existent.lan:5000/alpine", "latest"):
            self.fail("not tagged")
        try:
            resp: str = self.docker.images.push("non-existent.lan:5000/alpine", "latest")
            for obj in json_stream(resp):
                if 'no such host' in obj.get('errorDetail', {}).get('message', ''):
                    return
            self.fail('expected error not found in the response')
        except APIError as e:
            if 'no such host' not in e.explanation:
                self.fail('expected error not found in the response')

def json_stream(data: str) -> Iterator[dict]:
    decoder = json.JSONDecoder()
    pos = 0
    length = len(data)
    while pos < length:
        if data[pos].isspace():
            pos += 1
            continue
        obj, pos_end = decoder.raw_decode(data, pos)
        yield obj
        pos = pos_end

if __name__ == "__main__":
    # Setup temporary space
    unittest.main()
