![PODMAN logo](https://raw.githubusercontent.com/containers/common/main/logos/podman-logo-full-vert.png)

# Basic Setup and Use of Podman
Podman is a utility provided as part of the libpod library.  It can be used to create and maintain
containers. The following tutorial will teach you how to set up Podman and perform some basic
commands with Podman.

If you are running on a Mac or Windows PC, you should instead follow the [Mac and Windows tutorial](https://github.com/containers/podman/blob/main/docs/tutorials/mac_win_client.md)
to set up the remote Podman client.

**NOTE**: the code samples are intended to be run as a non-root user, and use `sudo` where
root escalation is required.

## Installing Podman

For installing or building Podman, see the [installation instructions](https://podman.io/getting-started/installation).

## Familiarizing yourself with Podman

### Running a sample container
This sample container will run a very basic httpd server (named basic_httpd) that serves only its index
page.
```console
podman run --name basic_httpd -d -p 8080:80/tcp docker.io/nginx
```
Because the container is being run in detached mode, represented by the *-d* in the `podman run` command, Podman
will print the container ID after it has run. Note that we use port forwarding to be able to
access the HTTP server.

### Listing running containers
The Podman *ps* command is used to list creating and running containers.
```console
podman ps
```

Note: If you add *-a* to the *ps* command, Podman will show all containers.
### Inspecting a running container
You can "inspect" a running container for metadata and details about itself.  We can even use
the inspect subcommand to see what IP address was assigned to the container. As the container is running in rootless mode, an IP address is often not assigned and the field may be empty.
```console
podman inspect basic_httpd --format 'IP address: {{.NetworkSettings.IPAddress}}'
```

Example output (rootless):

```text
IP address:
```

### Testing the httpd server
As we do not have the IP address of the container, we can test the network communication between the host
operating system and the container using curl. The following command should display the index page of our
containerized httpd server.
```console
curl http://localhost:8080
```

### Viewing the container's logs
You can view the container's logs with Podman as well:
```console
podman logs <container_id>
```

Example output:

```text
10.88.0.1 - - [07/Feb/2018:15:22:11 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:30 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:30 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:31 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
10.88.0.1 - - [07/Feb/2018:15:22:31 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.55.1" "-"
```

### Viewing the container's pids
And you can observe the httpd pid in the container with *top*.
```console
podman top <container_id>
```

Example output:

```text
  UID   PID  PPID  C STIME TTY          TIME CMD
    0 31873 31863  0 09:21 ?        00:00:00 nginx: master process nginx -g daemon off;
  101 31889 31873  0 09:21 ?        00:00:00 nginx: worker process
```

### Checkpointing the container
Checkpointing a container stops the container while writing the state of all processes in the container to disk.
With this a container can later be restored and continue running at exactly the same point in time as the
checkpoint. This capability requires CRIU 3.11 or later installed on the system.
This feature is not supported as rootless; as such, if you wish to try it, you'll need to re-create your container as root, using the same command but with sudo.

To checkpoint the container use:
```console
sudo podman container checkpoint <container_id>
```

### Restoring the container
Restoring a container is only possible for a previously checkpointed container. The restored container will
continue to run at exactly the same point in time it was checkpointed.
To restore the container use:
```console
sudo podman container restore <container_id>
```

After being restored, the container will answer requests again as it did before checkpointing.
```console
curl http://<IP_address>:8080
```

### Migrate the container
To live migrate a container from one host to another the container is checkpointed on the source
system of the migration, transferred to the destination system and then restored on the destination
system. When transferring the checkpoint, it is possible to specify an output-file.

On the source system:
```console
sudo podman container checkpoint <container_id> -e /tmp/checkpoint.tar.gz
scp /tmp/checkpoint.tar.gz <destination_system>:/tmp
```

On the destination system:
```console
sudo podman container restore -i /tmp/checkpoint.tar.gz
```

After being restored, the container will answer requests again as it did before checkpointing. This
time the container will continue to run on the destination system.
```console
curl http://<IP_address>:8080
```

### Stopping the container
To stop the httpd container:
```console
podman stop <container_id>
```
You can also check the status of one or more containers using the *ps* subcommand. In this case, we should
use the *-a* argument to list all containers.
```console
podman ps -a
```

### Removing the container
To remove the httpd container:
```console
podman rm <container_id>
```
You can verify the deletion of the container by running *podman ps -a*.

## Integration Tests
For more information on how to set up and run the integration tests in your environment, checkout the Integration Tests [README.md](../../test/README.md)

## More information

For more information on Podman and its subcommands, checkout the asciiart demos on the [README.md](../../README.md#commands)
page.
