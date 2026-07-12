# Change Request

## **Short Summary**

Implement the functionality to mount host certificates from the host system to
the Podman Machine OS.

## **Objective**

Users want the workloads running in containers to trust the Certificate
Authorities (CA) certificates that are installed in the host OS keystore.

The procedure to add the locally trusted certificates in the Podman machine is
currently documented as a manual procedure in both:

- [Podman tutorials](https://github.com/containers/podman/blob/main/docs/tutorials/podman-install-certificate-authority.md)
- [Podman Desktop Guide](https://podman-desktop.io/docs/podman/adding-certificates-to-a-podman-machine)

The objective is to automate this procedure and import the native trusted CA
certificates into the Podman machine's guest OS.

The certificates are imported during the start of a Podman machine.

If a new trusted CA certificate is added on the host, it will be imported only
after the Podman machine is restarted.

If a trusted certificate is removed from the host (i.e. it isn't considered
trusted anymore), it will be removed only after the Podman machine is
restarted.

## **Detailed Description:**

### The new `import-native-ca` machine configuration property

To implement this feature we will add a new flag, `--import-native-ca`, to the
`podman podman init` command. And a corresponding `import-native-ca` machine
configuration.

When `import-native-ca` is set to `true`, the certificates are imported when
`podman machine start` is executed.

The default value for `import-native-ca` is `false`. This is because, without
proper testing on customers environment (i.e. laptops connected to enterprises
internal networks with thousands of CA certificates) we can't predict the
impact of the certificates import on the time to startup the Podman machines
and we prefer to be conservative.

In our tests we have observed that the impact of the import of certificates
at the startup is negligible (less then a second to import 1K+ certificates).
If this is confirmed on a larger test base, the plan is to change the default
value to `true`.

### Fetching the locally trusted certificates

To import the locally trusted certificates, the Podman client retrieves the
trusted certificates from the local certificates stores.

The location of these stores may vary depending on the host OS, but Go provides
some abstractions to access the stores without worrying about the underlying
details.

For instance package `crypto/x509` provides an API to retrieve the bundle of
locally trusted certificates (see [`SystemCertPool()`](https://pkg.go.dev/crypto/x509#SystemCertPool)).

On Windows, where certificates are stored in the registry
[system stores](https://learn.microsoft.com/en-us/windows/win32/seccrypto/system-store-locations),
it's possible to get more fine grained access through the native APIs exposed
in package `golang.org/x/sys/windows` such as [`CertEnumCertificatesInStore`](https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-certenumcertificatesinstore)).

The certificates are extracted in the [PEM](https://www.rfc-editor.org/rfc/rfc1422)
format and saved in the machine data folder:

```shell
~/.local/share/containers/podman/machine/<provider>/<machine>/host-ca-certs.pem
```

### Installing the certificates in the guest OS

To install the certificates in the Fedora machine, the clients copies in the
`PEM` file containing all the host certificates in the guest `anchors` folder
(`/etc/pki/ca-trust/source/anchors`) and runs the command `update-ca-trust`. See
the [Fedora documentation for adding new certificates](https://docs.fedoraproject.org/en-US/quick-docs/using-shared-system-certificates/#_adding_new_certificates).

::NOTE:: The folder containing the `PEM` file is automatically mounted in the
guest with the default machine configuration. When this is not the case, because
the machine volumes have been customized for example, the file is transferred
from the host to the guest, with the `SCP` protocol.

### Removing old certificates from the guest OS

The certificates that are removed from the host will be automatically purged
when the machine is restarted.

This behaviour is a consequence of `update-ca-trust` removing the previously
installed as part of the update.

### If an error occurs during the import

The retrieval of the certificates, the creation of the certificates file, the
copy of the file from the host into the guest and the execution of the
`update-ca-trust` are all operations that can fail.

In the event of a failure, we warn the user but continue the machine startup
process.

### Impact on the Podman machine startup time

To evaluate the impact of certificates import to the machines startup times,
I have [patched the Podman client](https://github.com/l0rd/podman/tree/import_native_ca) to add a `machine start` flag to mount or copy
the certificates.

In particular we have used this version of the client to compare three scenarios
on Windows and `macOS`:

- `no import`: No import of certificates (default behavior)
- `scp`: Import of the certificates via `SCP`
- `mnt`: Import of the certificates via mount

The following table summarize the mean startup times for each type of run:

| PROVIDER | RUN TYPE | STARTUP TIME |
| ---- | ---- | ---- |
| WSL | no import | 6.2s |
| | mnt | +3.9s |
| | scp | +5.1s |
| HYPERV | no import | 22.3s |
| | mnt | +3.4s |
| | scp | +4.7s |
| APPLEHV | no import | 10.0s |
| | mnt | +1.4s |
| | scp | +1.6s |
| LIBKRUN | no import | 9.0s |
| | mnt | +1.3s |
| | scp | +1.4s |

[This gist](https://gist.github.com/l0rd/6e9b6640cc8ec780b53d2a74d9400936) has
the measurements' details.

## **Use cases**

### Sending requests to an HTTPS server in an enterprise network

The following example shows how to run an application inside a Podman container
that connects to an HTTPS server deployed within the internal enterprise
network. The server TLS certificate is signed by the enterprise CA.

```bash
podman run -ti --rm --security-opt=label=disable \
  -v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt:ro \
  fedora curl -I https://internal.redhat.com
```

### Sending requests to an HTTPS server running on localhost

A developer may deploy a web application locally and use a custom x509
certificate signed by a locally trusted CA (using
[mkcert](https://github.com/FiloSottile/mkcert) for example).

They then need to run an application, in a container, that connects to the web
application. For example:

```bash
podman run -ti --rm --security-opt=label=disable \
  -v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt:ro \
  fedora curl -I https://host.docker.internal:8080
```

## **Target Podman Release**

The target release is version 6.0.0.

## **Link(s)**

- [Jira issue](https://issues.redhat.com/browse/RUN-3839)

## **Stakeholders**

- [x] Podman Users
- [x] Podman Developers
- [ ] Buildah Users
- [ ] Buildah Developers
- [ ] Skopeo Users
- [ ] Skopeo Developers
- [x] Podman Desktop
- [ ] CRI-O
- [ ] Storage library
- [ ] Image library
- [ ] Common library
- [ ] Netavark and aardvark-dns

## **Assignee(s)**

- [@l0rd](https://github.com/l0rd)

## **Impacts**

### **CLI**

The new `--import-native-ca` option for the `podman machine init` command:

```bash
$ podman machine init --help
(...)
Options:
(...)
      --import-native-ca    Import the host trusted CA certificates into the machine
```

The new `--import-native-ca` option for the `podman machine set` command:

```bash
$ podman machine set --help
(...)
Options:
(...)
     --import-native-ca    Import the host trusted CA certificates into the machine
```

When `import-native-ca` is set to `true`, the import of the certificates is
logged at machine start:

```bash
$ podman machine start
Starting machine "podman-machine-default"
(...)
The host trusted CA certificates have been imported successfully
Machine "podman-machine-default" started successfully
```

### **Libpod**

None

### **Others**

Machine configuration ([source](https://github.com/containers/container-libs/blob/28c83ab6f016cf943c01196e1e6b335f5953cf84/common/pkg/config/config.go#L674C6-L674C20)
[config file](https://github.com/containers/container-libs/blob/28c83ab6f016cf943c01196e1e6b335f5953cf84/common/pkg/config/containers.conf#L895)
and [docs](https://github.com/containers/container-libs/blob/28c83ab6f016cf943c01196e1e6b335f5953cf84/common/docs/containers.conf.5.md?plain=1#L984))
needs to be updated.

Documentation needs to be updated:

- [Command: podman machine start](https://github.com/containers/podman/blob/main/docs/source/markdown/podman-machine-start.1.md.in)
- [Tutorial: Installing Crtificate Authority](https://github.com/containers/podman/blob/main/docs/tutorials/podman-install-certificate-authority.md)

## **Further Description (Optional):**

None

## **Test Descriptions (Optional):**

A new machine e2e tests will be added where we check that:

- when the flag `--import-native-ca` is passed, the local certificates are
imported.
- when the flag isn't passed the local certificates aren't imported.
- when a local certificate is removed, it is removed after executing
`podman machine start`.

Adding a custom CA certificate requires admin privileges on macOS and Windows
so we should avoid that. Instead we can

1) match the machine certificates with the host ones
2) look at the diff, in the machine certificates list, when the flag
`--import-native-ca` is turned on.
