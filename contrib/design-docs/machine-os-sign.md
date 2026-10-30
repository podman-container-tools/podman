# Change Request

## **Short Summary**

Podman machine images are not signed. We need to sign them to ensure images pulled from quay.io/podman/machine-os are authentic and haven't been tampered with.

## **Objective**

Images from `podman-machine-os` CI are signed. Running `podman machine init` successfully verifies signed images and rejects unsigned or tampered images.

## **Detailed Description:**

The standard approach to container image signing is Sigstore Cosign. In CI environments, the recommended method is OIDC-based keyless signing using Fulcio for certificate issuance and Rekor for transparency logging.

Keyless signing works by using OIDC tokens instead of managing private keys. This means we do not need to store any keys as a GH Action secret. When our workflow runs, it gets an OIDC token from GitHub. Cosign sends this token to Fulcio (Sigstore's certificate authority), which verifies with GitHub that the request really came from our specific workflow. Fulcio then issues a short-lived certificate. Cosign generates an ephemeral key pair, signs the image, uploads the signature to Rekor (a transparency log), and discards the private key. The signature gets attached to the image in quay.io.

When users run `podman machine init`, Podman pulls the image and its signature. The `containers/image` library verifies the signature using bundled trust roots (Fulcio's CA certificate and Rekor's public key) and checks that it matches our policy, which specifies that only our official release workflow can sign images.

In practice, the above solution looks like this:

### podman-machine-os
Add signing to `.github/workflows/release.yml`.
1. Add `id-token: write` permission so GitHub will generate OIDC tokens
1. Install cosign using the official sigstore/`cosign-installer@v3` action
1. Add a signing step after we push the image to quay.io.
    - `cosign sign --yes quay.io/podman/machine-os:${VERSION}.`
    - No secrets needed - uses OIDC

Since we need to push then sign the image, there is a possible race condition where if a user pulls the image after the image is pushed but before the image is signed, they will receive an unsigned image. The best way around this would be to push the image tagged something like `quay.io/podman/machine-os:X.Y-staging`, sign the image, then tag the image as the official `quay.io/podman/machine-os:X.Y` without the staging suffix

### podman
No Go code changes are required. `podman machine init` already respects `policy.json` if a machine image needs to be pulled.

Note that images that are not the official `podman-machine-os` image will not be checked, and signatures will only be checked on pull. Cached images will also not be checked.

Packaging changes:
1. Bundle the trust roots, which verifies signatures from public Sigstore
    - Fulcio's CA certificate - `fulcio_v1.crt.pem`
    - Rekor's public key - `rekor.pub`
1. Update `policy.json` to require sigstore signatures for our machine images.
    - Set `oidcIssuer` to https://token.actions.githubusercontent.com (GitHub Actions)
    - Set `subjectEmail` to https://github.com/podman-container-tools/podman-machine-os/.github/workflows/release.yml@refs/tags/
1. Update installers to install `policy.json` and the two trust roots

The policy.json change would look like this:

```
"transports": {
        "docker": {
            "quay.io/podman/machine-os": [
                {
                    "type": "sigstoreSigned",
                    "fulcio": {
                        "caPath": "/etc/containers/fulcio_v1.crt.pem",
                        "oidcIssuer": "https://token.actions.githubusercontent.com",
                        "subjectEmail": "https://github.com/podman-container-tools/podman-machine-os/.github/workflows/release.yml@refs/heads/main"
                    },
                    "rekorPublicKeyPath": "/etc/containers/rekor.pub",
                    "signedIdentity": {
                        "type": "matchRepository"
                    }
                },
                {
                    "type": "sigstoreSigned",
                    "fulcio": {
                        "caPath": "/etc/containers/fulcio_v1.crt.pem",
                        "oidcIssuer": "https://token.actions.githubusercontent.com",
                        "subjectEmail": "https://github.com/podman-container-tools/podman-machine-os/.github/workflows/release.yml@refs/tags/"
                    },
                    "rekorPublicKeyPath": "/etc/containers/rekor.pub",
                    "signedIdentity": {
                        "type": "matchRepository"
                    }
                }
            ]
        }
    }
```


## **Use cases**

Whenever we need to use Podman machine

## **Target Podman Release**

## **Link(s)**

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

## ** Assignee(s) **

acui

## **Impacts**

### **CLI**

### **Libpod**

### **Others**

podman machine

## **Further Description (Optional):**

We'll implement signing in `podman-machine-os` first, which makes images signed but doesn't enforce verification yet. We can test manually with cosign to confirm signatures are valid. Then we'll update Podman installers to bundle the trust roots and policy. When we release the new Podman version, verification will be enabled. From that point forward, `podman machine init` will automatically verify all images.

Backwards compatibility is maintained because Podman's existing code already supports optional `policy.json`. Without the policy file, unsigned images still work (current behavior). When the policy is installed, verification is enforced.

## **Test Descriptions (Optional):**
