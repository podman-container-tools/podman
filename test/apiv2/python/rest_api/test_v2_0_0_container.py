import io
import multiprocessing
import queue
import random
import subprocess
import tarfile
import threading
import unittest

import requests
import os
import time
from dateutil.parser import parse

from .fixtures import APITestCase


class ContainerTestCase(APITestCase):
    def test_list(self):
        r = requests.get(self.uri("/containers/json"), timeout=5)
        self.assertEqual(r.status_code, 200, r.text)
        obj = r.json()
        self.assertEqual(len(obj), 1)

    def test_list_filters_status(self):
        r = requests.get(
            self.podman_url
            + "/v1.40/containers/json?filters=%7B%22status%22%3A%5B%22running%22%5D%7D"
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        containerAmnt = len(payload)
        self.assertGreater(containerAmnt, 0)

    def test_list_filters_label(self):
        r = requests.get(
            self.podman_url
            + "/v1.40/containers/json?filters=%7B%22label%22%3A%5B%22nonexistlabel%22%5D%7D"
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        containerAmnt = len(payload)
        self.assertEqual(containerAmnt, 0)

    def test_list_filters_label_not(self):
        r = requests.get(
            self.podman_url
            + "/v1.40/containers/json?filters=%7B%22label%21%22%3A%5B%22nonexistlabel%22%5D%7D"
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        containerAmnt = len(payload)
        self.assertGreater(containerAmnt, 0)

    def test_list_all(self):
        r = requests.get(self.uri("/containers/json?all=true"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertId(r.content)

    def test_inspect(self):
        r = requests.get(self.uri(self.resolve_container("/containers/{}/json")))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertId(r.content)
        _ = parse(r.json()["Created"])

        r = requests.post(
            self.podman_url + "/v1.40/containers/create?name=topcontainer",
            json={
                "Cmd": ["top"],
                "Image": "alpine:latest",
                "Healthcheck": {
                    "Test": ["CMD", "pidof", "top"],
                    "Interval": 5000000000,
                    "Timeout": 2000000000,
                    "Retries": 3,
                    "StartPeriod": 5000000000,
                },
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        payload = r.json()
        container_id = payload["Id"]
        self.assertIsNotNone(container_id)

        r = requests.get(self.podman_url + f"/v1.40/containers/{container_id}/json")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertId(r.content)
        out = r.json()
        self.assertIsNotNone(out["State"].get("Health"))
        self.assertListEqual(["CMD", "pidof", "top"], out["Config"]["Healthcheck"]["Test"])
        self.assertEqual(5000000000, out["Config"]["Healthcheck"]["Interval"])
        self.assertEqual(2000000000, out["Config"]["Healthcheck"]["Timeout"])
        self.assertEqual(3, out["Config"]["Healthcheck"]["Retries"])
        self.assertEqual(5000000000, out["Config"]["Healthcheck"]["StartPeriod"])

        r = requests.get(self.uri(f"/containers/{container_id}/json"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertId(r.content)
        out = r.json()
        hc = out["Config"]["Healthcheck"]["Test"]
        self.assertListEqual(["CMD", "pidof", "top"], hc)

        r = requests.post(self.podman_url + f"/v1.40/containers/{container_id}/start")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(self.podman_url + f"/v1.40/containers/{container_id}/json")
        self.assertEqual(r.status_code, 200, r.text)
        out = r.json()
        state = out["State"]["Health"]
        self.assertIsInstance(state, dict)

    def test_stats(self):
        r = requests.get(self.uri(self.resolve_container("/containers/{}/stats?stream=false")))
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            self.assertId(r.content)
        r = requests.get(
            self.uri(self.resolve_container("/containers/{}/stats?stream=false&one-shot=true"))
        )
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            self.assertId(r.content)

    def test_delete(self):
        r = requests.delete(self.uri(self.resolve_container("/containers/{}?force=true")))
        self.assertEqual(r.status_code, 200, r.text)

    def test_stop(self):
        r = requests.post(self.uri(self.resolve_container("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(self.uri(self.resolve_container("/containers/{}/stop")))
        self.assertIn(r.status_code, (204, 304), r.text)

    def test_start(self):
        r = requests.post(self.uri(self.resolve_container("/containers/{}/stop")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(self.uri(self.resolve_container("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

    def test_restart(self):
        r = requests.post(self.uri(self.resolve_container("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(self.uri(self.resolve_container("/containers/{}/restart")), timeout=5)
        self.assertEqual(r.status_code, 204, r.text)

    def test_resize(self):
        r = requests.post(self.uri(self.resolve_container("/containers/{}/resize?h=43&w=80")))
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            self.assertEqual(r.text, "", r.text)

    def test_attach(self):
        r = requests.post(
            self.podman_url + "/v1.40/containers/create?name=topcontainer",
            json={"Cmd": ["sh", "-c", "echo podman; sleep 100"], "Image": "alpine:latest"},
        )
        self.assertEqual(r.status_code, 201, r.text)
        payload = r.json()

        r = requests.post(
            self.podman_url
            + f"/v1.40/containers/{payload['Id']}/start"
        )
        self.assertEqual(r.status_code, 204, r.text)

        # wait for the log message to appear to avoid flakes on slow systems
        # with the /attach?logs=true test below
        for _ in range(5):
            r = requests.get(
                self.podman_url
                + f"/v1.40/containers/{payload['Id']}/logs?stdout=true"
            )
            self.assertIn(r.status_code, (101, 200), r.text)
            if r.content == b"\x01\x00\x00\x00\x00\x00\x00\x07podman\n":
                break

            time.sleep(1)

        r = requests.post(
            self.podman_url
            + f"/v1.40/containers/{payload['Id']}/attach?logs=true&stream=false"
        )
        self.assertIn(r.status_code, (101, 200), r.text)
        # see the attach format docs, stdout = 1, length = 7, message = podman\n
        self.assertEqual(r.content, b"\x01\x00\x00\x00\x00\x00\x00\x07podman\n", r.text)

        r = requests.post(
            self.podman_url
            + f"/v1.40/containers/{payload['Id']}/stop?t=0"
        )
        self.assertEqual(r.status_code, 204, r.text)

        requests.delete(
            self.podman_url + f"/v1.40/containers/{payload['Id']}?force=true"
        )

    def test_logs(self):
        r = requests.get(self.uri(self.resolve_container("/containers/{}/logs?stdout=true")))
        self.assertEqual(r.status_code, 200, r.text)
        r = requests.post(
            self.podman_url + "/v1.40/containers/create?name=topcontainer",
            json={"Cmd": ["top", "ls"], "Image": "alpine:latest"},
        )
        self.assertEqual(r.status_code, 201, r.text)
        payload = r.json()
        container_id = payload["Id"]
        self.assertIsNotNone(container_id)
        r = requests.get(
            self.podman_url
            + f"/v1.40/containers/{payload['Id']}/logs?follow=false&stdout=true&until=0"
        )
        self.assertEqual(r.status_code, 200, r.text)
        r = requests.get(
            self.podman_url
            + f"/v1.40/containers/{payload['Id']}/logs?follow=false&stdout=true&until=1"
        )
        self.assertEqual(r.status_code, 200, r.text)

    def test_commit(self):
        r = requests.post(self.uri(self.resolve_container("/commit?container={}")))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertId(r.content)

        obj = r.json()
        self.assertIsInstance(obj, dict)

    def test_prune(self):
        name = f"Container_{random.getrandbits(160):x}"

        r = requests.post(
            self.podman_url + f"/v1.40/containers/create?name={name}",
            json={
                "Cmd": ["cp", "/etc/motd", "/motd.size_test"],
                "Image": "alpine:latest",
                "NetworkDisabled": True,
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        create = r.json()

        r = requests.post(self.podman_url + f"/v1.40/containers/{create['Id']}/start")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.post(self.podman_url + f"/v1.40/containers/{create['Id']}/wait")
        self.assertEqual(r.status_code, 200, r.text)
        wait = r.json()
        self.assertEqual(wait["StatusCode"], 0, wait["Error"])

        prune = requests.post(self.podman_url + "/v1.40/containers/prune")
        self.assertEqual(prune.status_code, 200, prune.status_code)
        prune_payload = prune.json()
        self.assertGreater(prune_payload["SpaceReclaimed"], 0)
        self.assertIn(create["Id"], prune_payload["ContainersDeleted"])

        # Delete any orphaned containers
        r = requests.get(self.podman_url + "/v1.40/containers/json?all=true")
        self.assertEqual(r.status_code, 200, r.text)
        for self.resolve_container in r.json():
            requests.delete(
                self.podman_url + f"/v1.40/containers/{self.resolve_container['Id']}?force=true"
            )

        # Image prune here tied to containers freeing up
        prune = requests.post(self.podman_url + "/v1.40/images/prune")
        self.assertEqual(prune.status_code, 200, prune.text)
        prune_payload = prune.json()
        self.assertGreater(prune_payload["SpaceReclaimed"], 0)

        # FIXME need method to determine which image is going to be "pruned" to fix test
        # TODO should handler be recursive when deleting images?
        # self.assertIn(img["Id"], prune_payload["ImagesDeleted"][1]["Deleted"])

        # FIXME (@vrothberg): I commented this line out during the `libimage` migration.
        # It doesn't make sense to report anything to be deleted if the reclaimed space
        # is zero.  I think the test needs some rewrite.
        # self.assertIsNotNone(prune_payload["ImagesDeleted"][1]["Deleted"])

    def test_create_duplicate_name(self):
        name = f"Container_{random.getrandbits(160):x}"
        payload = {"Cmd": ["top"], "Image": "alpine:latest"}

        r = requests.post(
            self.podman_url + f"/v1.40/containers/create?name={name}", json=payload
        )
        self.assertEqual(r.status_code, 201, r.text)
        container_id = r.json()["Id"]

        r = requests.post(
            self.podman_url + f"/v1.40/containers/create?name={name}", json=payload
        )
        self.assertEqual(r.status_code, 409, r.text)

        requests.delete(self.podman_url + f"/v1.40/containers/{container_id}?force=true")

    def test_status(self):
        r = requests.post(
            self.podman_url + "/v1.40/containers/create?name=topcontainer",
            json={"Cmd": ["top"], "Image": "alpine:latest"},
        )
        self.assertEqual(r.status_code, 201, r.text)
        payload = r.json()
        container_id = payload["Id"]
        self.assertIsNotNone(container_id)

        r = requests.get(
            self.podman_url + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        self.assertEqual(payload[0]["Status"], "Created")

        r = requests.post(self.podman_url + f"/v1.40/containers/{container_id}/start")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(
            self.podman_url + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        self.assertTrue(str(payload[0]["Status"]).startswith("Up"))

        r = requests.post(self.podman_url + f"/v1.40/containers/{container_id}/pause")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(
            self.podman_url + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        self.assertTrue(str(payload[0]["Status"]).startswith("Up"))
        self.assertTrue(str(payload[0]["Status"]).endswith("(Paused)"))

        r = requests.post(self.podman_url + f"/v1.40/containers/{container_id}/unpause")
        self.assertEqual(r.status_code, 204, r.text)
        r = requests.post(self.podman_url + f"/v1.40/containers/{container_id}/stop")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(
            self.podman_url + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        self.assertTrue(str(payload[0]["Status"]).startswith("Exited"))

        r = requests.delete(self.podman_url + f"/v1.40/containers/{container_id}")
        self.assertEqual(r.status_code, 204, r.text)

    def test_top_no_stream(self):
        uri = self.uri(self.resolve_container("/containers/{}/top"))
        q = queue.Queue()

        def _impl(fifo):
            fifo.put(requests.get(uri, params={"stream": False}, timeout=2))

        top = threading.Thread(target=_impl, args=(q,))
        top.start()
        time.sleep(2)
        self.assertFalse(top.is_alive(), f"GET {uri} failed to return in 2s")

        qr = q.get(False)
        self.assertEqual(qr.status_code, 200, qr.text)

        qr.close()
        top.join()

    def test_top_stream(self):
        uri = self.uri(self.resolve_container("/containers/{}/top"))
        q = queue.Queue()

        stop_thread = False

        def _impl(fifo, stop):
            try:
                with requests.get(uri, params={"stream": True, "delay": 1}, stream=True) as r:
                    r.raise_for_status()
                    fifo.put(r)
                    for buf in r.iter_lines(chunk_size=None):
                        if stop():
                            break
                        fifo.put(buf)
            except Exception:
                pass

        top = threading.Thread(target=_impl, args=(q, (lambda: stop_thread)))
        top.start()
        time.sleep(4)
        self.assertTrue(top.is_alive(), f"GET {uri} exited too soon")
        stop_thread = True

        for _ in range(10):
            try:
                qr = q.get_nowait()
                if qr is not None:
                    self.assertEqual(qr.status_code, 200)
                    qr.close()
                    break
            except queue.Empty:
                pass
            finally:
                time.sleep(1)
        else:
            self.fail("Server failed to respond in 10s")
        top.join()

    def test_memory(self):
        r = requests.post(
            self.podman_url + "/v1.4.0/libpod/containers/create",
            json={
                "Name": "memory",
                "Cmd": ["top"],
                "Image": "alpine:latest",
                "Resource_Limits": {
                    "Memory":{
                        "Limit": 1000,
                    },
                    "CPU":{
                        "Shares": 200,
                    },
                },
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        payload = r.json()
        container_id = payload["Id"]
        self.assertIsNotNone(container_id)

        r = requests.get(self.podman_url + f"/v1.40/containers/{container_id}/json")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertId(r.content)
        out = r.json()
        self.assertEqual(2000, out["HostConfig"]["MemorySwap"])
        self.assertEqual(1000, out["HostConfig"]["Memory"])

    def test_host_config_port_bindings(self):
        # create a container with two ports exposed, but only one of the ports bound
        r = requests.post(
            self.podman_url + "/v1.40/containers/create",
            json={
                "Name": "memory",
                "Cmd": ["top"],
                "Image": "alpine:latest",
                "HostConfig": {
                    "PortBindings": {
                        "8080": [{"HostPort": "87634"}]
                    }
                },
                "ExposedPorts": {
                    "8080": {},
                    "8081": {}
                }
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        payload = r.json()
        container_id = payload["Id"]
        self.assertIsNotNone(container_id)

        r = requests.get(self.podman_url +
                         f"/v1.40/containers/{container_id}/json")
        self.assertEqual(r.status_code, 200, r.text)
        inspect_response = r.json()
        # both ports are in the config
        self.assertEqual(2, len(inspect_response["Config"]["ExposedPorts"]))
        self.assertTrue("8080/tcp" in inspect_response["Config"]["ExposedPorts"])
        self.assertTrue("8081/tcp" in inspect_response["Config"]["ExposedPorts"])
        # only 8080 one port is bound
        self.assertEqual(1, len(inspect_response["HostConfig"]["PortBindings"]))
        self.assertTrue("8080/tcp" in inspect_response["HostConfig"]["PortBindings"])
        self.assertFalse("8081/tcp" in inspect_response["HostConfig"]["PortBindings"])

    def test_host_config_cgroupns_mode(self):
        for mode in ["private", "host"]:
            r = requests.post(
                self.podman_url + "/v1.40/containers/create",
                json={
                    "Name": "cgroupns_" + mode,
                    "Cmd": ["top"],
                    "Image": "alpine:latest",
                    "HostConfig": {
                        "CgroupnsMode": mode
                    }
                },
            )
            self.assertEqual(r.status_code, 201, r.text)
            payload = r.json()
            container_id = payload["Id"]
            self.assertIsNotNone(container_id)

            r = requests.get(self.podman_url + f"/v1.40/containers/{container_id}/json")
            self.assertEqual(r.status_code, 200, r.text)
            inspect_response = r.json()
            self.assertEqual(mode, inspect_response["HostConfig"]["CgroupnsMode"])

            r = requests.delete(self.podman_url + f"/v1.40/containers/{container_id}")
            self.assertEqual(r.status_code, 204, r.text)

def execute_process(cmd):
    return subprocess.run(
                cmd,
                shell=True,
                check=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )

def create_named_network_ns(network_ns_name):
    execute_process(f"ip netns add {network_ns_name}")
    execute_process(f"ip netns exec {network_ns_name} ip link add enp2s0 type veth peer name eth0")
    execute_process(f"ip netns exec {network_ns_name} ip addr add 10.0.1.0/24 dev eth0")
    execute_process(f"ip netns exec {network_ns_name} ip link set eth0 up")
    execute_process(f"ip netns exec {network_ns_name} ip link add enp2s1 type veth peer name eth1")
    execute_process(f"ip netns exec {network_ns_name} ip addr add 10.0.2.0/24 dev eth1")
    execute_process(f"ip netns exec {network_ns_name} ip link set eth1 up")

def delete_named_network_ns(network_ns_name):
    execute_process(f"ip netns delete {network_ns_name}")

class ContainerCompatibleAPITestCase(APITestCase):
    def test_inspect_network(self):
        if os.getuid() != 0:
            self.skipTest("test needs to be executed as root!")
        try:
            network_ns_name = "test-compat-api"
            create_named_network_ns(network_ns_name)
            self.podman.run("rm", "--all", "--force", check=True)
            self.podman.run("run", "--net", f"ns:/run/netns/{network_ns_name}", "-d", "alpine", "top", check=True)

            r = requests.post(self.uri(self.resolve_container("/containers/{}/start")))
            self.assertIn(r.status_code, (204, 304), r.text)

            container_uri = self.resolve_container("/containers/{}/json")

            # SecondaryIPAddresses present for API < v1.52
            r = requests.get(self.podman_url + "/v1.44/" + container_uri)
            self.assertEqual(r.status_code, 200, r.text)
            self.assertId(r.content)
            out = r.json()
            self.assertEqual("10.0.2.0", out["NetworkSettings"]["SecondaryIPAddresses"][0]["Addr"])
            self.assertEqual(24, out["NetworkSettings"]["SecondaryIPAddresses"][0]["PrefixLen"])

            # SecondaryIPAddresses removed for API >= v1.52
            r = requests.get(self.podman_url + "/v1.52/" + container_uri)
            self.assertEqual(r.status_code, 200, r.text)
            self.assertId(r.content)
            out = r.json()
            self.assertIsNone(out["NetworkSettings"].get("SecondaryIPAddresses"))
        finally:
            delete_named_network_ns(network_ns_name)

if __name__ == "__main__":
    unittest.main()
