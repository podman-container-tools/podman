"""
Integration tests for exercising docker-py against Podman Service.
"""
import io
import json
import tarfile
import threading
import time
from typing import IO, List, Optional

import yaml
from docker import errors
from docker.models.containers import Container
from docker.models.images import Image
from docker.models.volumes import Volume
from docker.types import Mount
from jsonschema.exceptions import best_match, ValidationError

# pylint: disable=no-name-in-module,import-error,wrong-import-order
from test.python.docker.compat import common, constant
from openapi_schema_validator import OAS31Validator

from test.python.docker.compat.constant import DOCKER_API_COMPATIBILITY_VERSION


# pylint: disable=missing-function-docstring
class TestContainers(common.DockerTestCase):
    """TestCase for exercising containers."""

    def test_create_container(self):
        """Run a container with detach mode."""
        self.docker.containers.create(image="alpine", detach=True)
        self.assertEqual(len(self.docker.containers.list(all=True)), 2)

    def test_create_network(self):
        """Add network to a container."""
        self.docker.networks.create("testNetwork", driver="bridge")
        self.docker.containers.create(image="alpine", detach=True)

    def test_start_container(self):
        # Podman docs says it should give a 304 but returns with no response
        # # Start an already started container should return 304
        # response = self.docker.api.start(container=self.top_container_id)
        # self.assertEqual(error.exception.response.status_code, 304)

        # Create a new container and validate the count
        self.docker.containers.create(image=constant.ALPINE, name="container2")
        containers = self.docker.containers.list(all=True)
        self.assertEqual(len(containers), 2)

    def test_start_container_with_random_port_bind(self):
        container = self.docker.containers.create(
            image=constant.ALPINE,
            name="containerWithRandomBind",
            ports={"1234/tcp": None},
        )
        containers = self.docker.containers.list(all=True)
        self.assertTrue(container in containers)

    def test_stop_container(self):
        top = self.docker.containers.get(self.top_container_id)
        self.assertEqual(top.status, "running")

        # Stop a running container and validate the state
        top.stop()
        top.reload()
        self.assertIn(top.status, ("stopped", "exited"))

    def test_kill_container(self):
        top = self.docker.containers.get(self.top_container_id)
        self.assertEqual(top.status, "running")

        # Kill a running container and validate the state
        top.kill()
        top.reload()
        self.assertIn(top.status, ("stopped", "exited"))

    def test_restart_container(self):
        # Validate the container state
        top = self.docker.containers.get(self.top_container_id)
        top.stop()
        top.reload()
        self.assertIn(top.status, ("stopped", "exited"))

        # restart a running container and validate the state
        top.restart()
        top.reload()
        self.assertEqual(top.status, "running")

    def test_remove_container(self):
        # Remove container by ID with force
        top = self.docker.containers.get(self.top_container_id)
        top.remove(force=True)
        self.assertEqual(len(self.docker.containers.list()), 0)

    def test_remove_container_without_force(self):
        # Validate current container count
        self.assertEqual(len(self.docker.containers.list()), 1)

        # Remove running container should throw error
        top = self.docker.containers.get(self.top_container_id)
        with self.assertRaises(errors.APIError) as error:
            top.remove()
        self.assertEqual(error.exception.response.status_code, 500)

        # Remove container by ID without force
        top.stop()
        top.remove()
        self.assertEqual(len(self.docker.containers.list()), 0)

    def test_pause_container(self):
        # Validate the container state
        top = self.docker.containers.get(self.top_container_id)
        self.assertEqual(top.status, "running")

        # Pause a running container and validate the state
        top.pause()
        top.reload()
        self.assertEqual(top.status, "paused")

    def test_pause_stopped_container(self):
        # Stop the container
        top = self.docker.containers.get(self.top_container_id)
        top.stop()

        # Pause exited container should throw error
        with self.assertRaises(errors.APIError) as error:
            top.pause()
        self.assertEqual(error.exception.response.status_code, 500)

    def test_unpause_container(self):
        top = self.docker.containers.get(self.top_container_id)

        # Validate the container state
        top.pause()
        top.reload()
        self.assertEqual(top.status, "paused")

        # Pause a running container and validate the state
        top.unpause()
        top.reload()
        self.assertEqual(top.status, "running")

    def test_list_container(self):
        # Add container and validate the count
        self.docker.containers.create(image="alpine", detach=True)
        containers = self.docker.containers.list(all=True)
        self.assertEqual(len(containers), 2)

    def test_filters(self):
        self.skipTest("TODO Endpoint does not yet support filters")

        # List container with filter by id
        filters = {"id": self.top_container_id}
        ctnrs = self.docker.containers.list(all=True, filters=filters)
        self.assertEqual(len(ctnrs), 1)

        # List container with filter by name
        filters = {"name": "top"}
        ctnrs = self.docker.containers.list(all=True, filters=filters)
        self.assertEqual(len(ctnrs), 1)

    def test_copy_to_container(self):
        ctr: Optional[Container] = None
        vol: Optional[Volume] = None
        try:
            test_file_content = b"Hello World!"
            vol = self.docker.volumes.create("test-volume")
            ctr = self.docker.containers.create(
                image="alpine",
                detach=True,
                command="top",
                volumes=["test-volume:/test-volume-read-only:ro"],
            )
            ctr.start()

            buff: IO[bytes] = io.BytesIO()
            with tarfile.open(fileobj=buff, mode="w:") as file:
                info: tarfile.TarInfo = tarfile.TarInfo()
                info.uid = 1042
                info.gid = 1043
                info.name = "a.txt"
                info.path = "a.txt"
                info.mode = 0o644
                info.type = tarfile.REGTYPE
                info.size = len(test_file_content)
                file.addfile(info, fileobj=io.BytesIO(test_file_content))

            buff.seek(0)
            ctr.put_archive("/tmp/", buff)
            ret, out = ctr.exec_run(["stat", "-c", "%u:%g", "/tmp/a.txt"])

            self.assertEqual(ret, 0)
            self.assertEqual(out.rstrip(), b"1042:1043", "UID/GID of copied file")

            ret, out = ctr.exec_run(["cat", "/tmp/a.txt"])
            self.assertEqual(ret, 0)
            self.assertEqual(out.rstrip(), test_file_content, "Content of copied file")

            buff.seek(0)
            with self.assertRaises(errors.APIError):
                ctr.put_archive("/test-volume-read-only/", buff)
        finally:
            if ctr is not None:
                ctr.stop()
                ctr.remove()
            if vol is not None:
                vol.remove(force=True)

    def test_mount_preexisting_dir(self):
        dockerfile = (
            b"FROM quay.io/libpod/alpine:latest\n"
            b"USER root\n"
            b"RUN mkdir -p /workspace\n"
            b"RUN chown 1042:1043 /workspace"
        )
        img: Image
        img, out = self.docker.images.build(fileobj=io.BytesIO(dockerfile))
        ctr: Container = self.docker.containers.create(
            image=img.id,
            detach=True,
            command="top",
            volumes=["test_mount_preexisting_dir_vol:/workspace"],
        )
        ctr.start()
        _, out = ctr.exec_run(["stat", "-c", "%u:%g", "/workspace"])
        self.assertEqual(out.rstrip(), b"1042:1043", "UID/GID set in dockerfile")

    def test_non_existent_workdir(self):
        dockerfile = (
            b"FROM quay.io/libpod/alpine:latest\n"
            b"USER root\n"
            b"WORKDIR /workspace/scratch\n"
            b"RUN touch test"
        )
        img: Image
        img, _ = self.docker.images.build(fileobj=io.BytesIO(dockerfile))
        ctr: Container = self.docker.containers.create(
            image=img.id,
            detach=True,
            command="top",
            volumes=["test_non_existent_workdir:/workspace"],
        )
        ctr.start()
        ret, _ = ctr.exec_run(["stat", "/workspace/scratch/test"])
        self.assertEqual(ret, 0, "Working directory created if it doesn't exist")

    def test_build_pull(self):
        dockerfile = (
            b"FROM quay.io/libpod/alpine:latest\n"
            b"USER 1000:1000\n"
        )
        img: Image
        img, logs = self.docker.images.build(fileobj=io.BytesIO(dockerfile), quiet=False, pull=True)
        has_tried_pull = False
        for e in logs:
            if "stream" in e and "trying to pull" in e["stream"].lower():
                has_tried_pull = True
        self.assertTrue(has_tried_pull, "the build process has not tried to pull the base image")

        img, logs = self.docker.images.build(fileobj=io.BytesIO(dockerfile), quiet=False, pull=False)
        has_tried_pull = False
        for e in logs:
            if "stream" in e and "trying to pull" in e["stream"].lower():
                has_tried_pull = True
        self.assertFalse(has_tried_pull, "the build process has tried tried to pull the base image")

    def test_mount_options_by_default(self):
        ctr: Optional[Container] = None
        vol: Optional[Volume] = None

        try:
            vol = self.docker.volumes.create("test-volume")
            ctr = self.docker.containers.create(
                image="alpine",
                detach=True,
                command="top",
                mounts=[
                    Mount(target="/vol-mnt", source="test-volume", type="volume", read_only=False)
                ],
            )
            ctr_inspect = self.docker.api.inspect_container(ctr.id)
            binds: List[str] = ctr_inspect["HostConfig"]["Binds"]
            self.assertEqual(len(binds), 1)
            self.assertEqual(binds[0], "test-volume:/vol-mnt:rprivate,nosuid,nodev,rbind")
        finally:
            if ctr is not None:
                ctr.remove()
            if vol is not None:
                vol.remove(force=True)

    def test_wait_next_exit(self):
        ctr: Container = self.docker.containers.create(
            image=constant.ALPINE,
            name="test-exit",
            command=["true"],
            labels={"my-label": "0" * 250_000})

        try:
            def wait_and_start():
                time.sleep(5)
                ctr.start()

            t = threading.Thread(target=wait_and_start)
            t.start()
            ctr.wait(condition="next-exit", timeout=300)
            t.join()
        finally:
            ctr.stop()
            ctr.remove(force=True)

    def test_container_inspect_compatibility(self):
        """Test container inspect result compatibility with DOCKER_API.
        When upgrading module "github.com/moby/moby" this test might fail, if so please correct podman inspect
        command result to stay compatible with docker.
        """
        ctr = self.docker.containers.create(image="alpine", detach=True)
        try:
            spec = yaml.load(open("vendor/github.com/moby/moby/api/swagger.yaml").read(), Loader=yaml.Loader)
            ctr_inspect = json.loads(self.podman.run("inspect", ctr.id).stdout)[0]
            schema = spec['paths']["/containers/{id}/json"]["get"]['responses'][200]['schema']
            schema["definitions"] = spec["definitions"]

            OAS31Validator.check_schema(schema)
            validator = OAS31Validator(schema)
            important_error = []
            for error in validator.iter_errors(ctr_inspect):
                if isinstance(error, ValidationError):
                    # ignore None instead of object/array/string errors
                    if error.message.startswith("None is not of type"):
                        continue
                    # ignore Windows specific option error
                    if error.json_path == '$.HostConfig.Isolation':
                        continue
                important_error.append(error)
            if important_error:
                if newversion := spec["info"]["version"] != DOCKER_API_COMPATIBILITY_VERSION:
                    ex = Exception(f"There may be a breaking change in Docker API between "
                                   f"{DOCKER_API_COMPATIBILITY_VERSION} and {newversion}")
                    raise best_match(important_error) from ex
                else:
                    raise best_match(important_error)
        finally:
            ctr.stop()
            ctr.remove(force=True)
