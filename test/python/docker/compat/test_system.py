"""
Integration tests for exercising docker-py against Podman Service.
"""

# pylint: disable=no-name-in-module,import-error,wrong-import-order
from test.python.docker import constant
from test.python.docker.compat import common


# pylint: disable=missing-function-docstring
class TestSystem(common.DockerTestCase):
    """TestCase for exercising Podman system services."""

    def test_info(self):
        info = self.docker.info()
        self.assertIsNotNone(info)
        self.assertEqual(info["RegistryConfig"]["IndexConfigs"]["localhost:5000"]["Secure"], False)
        self.assertEqual(
            info["RegistryConfig"]["IndexConfigs"]["localhost:5000"]["Mirrors"],
            ["mirror.localhost:5000"],
        )

    def test_info_container_details(self):
        info = self.docker.info()
        self.assertEqual(info["Containers"], 1)
        self.docker.containers.create(image=constant.ALPINE)
        info = self.docker.info()
        self.assertEqual(info["Containers"], 2)

    def test_version(self):
        version = self.docker.version()
        self.assertIsNotNone(version["Platform"]["Name"])
