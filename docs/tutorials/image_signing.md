# How to sign and distribute container images using Podman

Signing container images originates from the motivation of trusting only
dedicated image providers to mitigate man-in-the-middle (MITM) attacks or
attacks on container registries. One way to sign images is to utilize a GNU
Privacy Guard ([GPG][0]) key. This technique is generally compatible with any
OCI compliant container registry like [Quay.io][1]. It is worth mentioning that
the OpenShift integrated container registry supports this signing mechanism out
of the box, which makes separate signature storage unnecessary.

[0]: https://gnupg.org
[1]: https://quay.io

From a technical perspective, we can utilize Podman to sign the image before
pushing it into a remote registry. After that, all systems running Podman have
to be configured to retrieve the signatures from a remote server, which can
be any simple web server. This means that every unsigned image will be rejected
during an image pull operation. But how does this work?

First of all, we have to create a GPG key pair or select an already locally
available one. To generate a new GPG key, just run `gpg --full-gen-key` and
follow the interactive dialog. Now we should be able to verify that the key
exists locally:

```bash
> gpg --list-keys sgrunert@suse.com
pub   rsa2048 2018-11-26 [SC] [expires: 2020-11-25]
      XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
uid           [ultimate] Sascha Grunert <sgrunert@suse.com>
sub   rsa2048 2018-11-26 [E] [expires: 2020-11-25]
```

Now let’s assume that we run a container registry. For example we could simply
start one on our local machine:

```bash
sudo podman run -d -p 5000:5000 docker.io/registry
```

The registry does not know anything about image signing, it just provides the remote
storage for the container images. This means if we want to sign an image, we
have to take care of how to distribute the signatures.

Let’s choose a standard `alpine` image for our signing experiment:

```bash
sudo podman pull docker://docker.io/alpine:latest
```

```bash
sudo podman images alpine
REPOSITORY                 TAG      IMAGE ID       CREATED       SIZE
docker.io/library/alpine   latest   e7d92cdc71fe   6 weeks ago   5.86 MB
```

Now we can re-tag the image to point it to our local registry:

```bash
sudo podman tag alpine localhost:5000/alpine
```

```bash
sudo podman images alpine
REPOSITORY                 TAG      IMAGE ID       CREATED       SIZE
localhost:5000/alpine      latest   e7d92cdc71fe   6 weeks ago   5.86 MB
docker.io/library/alpine   latest   e7d92cdc71fe   6 weeks ago   5.86 MB
```

Podman would now be able to push the image and sign it in one command. But to
let this work, we have to modify our system-wide registries configuration at
`/etc/containers/registries.d/default.yaml`:

```yaml
default-docker:
  sigstore: http://localhost:8000 # Added by us
  sigstore-staging: file:///var/lib/containers/sigstore
```

We can see that we have two signature stores configured:

- `sigstore`: referencing a web server for signature reading
- `sigstore-staging`: referencing a file path for signature writing

Now, let’s push and sign the image:

```bash
sudo -E GNUPGHOME=$HOME/.gnupg \
    podman push \
    --tls-verify=false \
    --sign-by sgrunert@suse.com \
    localhost:5000/alpine
…
Storing signatures
```

If we now take a look at the systems signature storage, then we see that there
is a new signature available, which was caused by the image push:

```bash
sudo ls /var/lib/containers/sigstore
'alpine@sha256=e9b65ef660a3ff91d28cc50eba84f21798a6c5c39b4dd165047db49e84ae1fb9'
```

The default signature store in our edited version of
`/etc/containers/registries.d/default.yaml` references a web server listening at
`http://localhost:8000`. For our experiment, we simply start a new server inside
the local staging signature store:

```bash
sudo bash -c 'cd /var/lib/containers/sigstore && python3 -m http.server'
Serving HTTP on 0.0.0.0 port 8000 (http://0.0.0.0:8000/) ...
```

Let’s remove the local images for our verification test:

```
sudo podman rmi docker.io/alpine localhost:5000/alpine
```

We have to write a policy to enforce that the signature has to be valid. This
can be done by adding a new rule in `/etc/containers/policy.json`. From the
below example, copy the `"docker"` entry into the `"transports"` section of your
`policy.json`.

```json
{
  "default": [{ "type": "insecureAcceptAnything" }],
  "transports": {
    "docker": {
      "localhost:5000": [
        {
          "type": "signedBy",
          "keyType": "GPGKeys",
          "keyPath": "/tmp/key.gpg"
        }
      ]
    }
  }
}
```

The `keyPath` does not exist yet, so we have to put the GPG key there:

```bash
gpg --output /tmp/key.gpg --armor --export sgrunert@suse.com
```

If we now pull the image:

```bash
sudo podman pull --tls-verify=false localhost:5000/alpine
…
Storing signatures
e7d92cdc71feacf90708cb59182d0df1b911f8ae022d29e8e95d75ca6a99776a
```

Then we can see in the logs of the web server that the signature has been
accessed:

```
127.0.0.1 - - [04/Mar/2020 11:18:21] "GET /alpine@sha256=e9b65ef660a3ff91d28cc50eba84f21798a6c5c39b4dd165047db49e84ae1fb9/signature-1 HTTP/1.1" 200 -
```

As a counterpart example, if we specify the wrong key at `/tmp/key.gpg`:

```bash
gpg --output /tmp/key.gpg --armor --export mail@saschagrunert.de
File '/tmp/key.gpg' exists. Overwrite? (y/N) y
```

Then a pull is not possible any more:

```bash
sudo podman pull --tls-verify=false localhost:5000/alpine
Trying to pull localhost:5000/alpine...
Error: pulling image "localhost:5000/alpine": unable to pull localhost:5000/alpine: unable to pull image: Source image rejected: Invalid GPG signature: …
```

So in general there are four main things to be taken into consideration when
signing container images with Podman and GPG:

1. We need a valid private GPG key on the signing machine and corresponding
   public keys on every system which would pull the image
2. A web server has to run somewhere which has access to the signature storage
3. The web server has to be configured in any
   `/etc/containers/registries.d/*.yaml` file
4. Every image pulling system has to be configured to contain the enforcing
   policy configuration via `policy.conf`

That’s it for image signing and GPG. The cool thing is that this setup works out
of the box with [CRI-O][2] as well and can be used to sign container images in
Kubernetes environments.

[2]: https://cri-o.io
