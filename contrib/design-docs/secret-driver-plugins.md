# Change Request

## **Short Summary**
Introduce an API for Podman "Secret Providers", allowing external binaries (e.g., `systemd-creds`, HashiCorp Vault, AWS Secrets Manager) to dynamically provide secrets at container runtime. This bypasses the need for `podman secret create` and avoids storing sensitive data persistently in Podman's internal database.

## **Objective**
Currently, to use external secret managers, secrets must be duplicated into Podman via `podman secret create`. Maintainers have indicated a desire to move away from stateful secret drivers. 
This design introduces a stateless, just-in-time lookup mechanism triggered via the `provider=<name>` option in `podman run --secret` or Quadlet definitions. 

## **Detailed Description:**

### 1. Configuration (`containers.conf`)
We will introduce a new map in `containers.conf` to register providers.

```toml
[secrets.providers]
systemd-creds = "/usr/libexec/podman-secret-provider-systemd-creds"
vault = "/opt/bin/podman-provider-vault"
```
The key is the provider name used in the CLI, and the value is the absolute path to the executable binary.

### 2. CLI and Execution Surface
A user mounts a secret into a container directly via the run command (or Quadlet), completely skipping `podman secret create`:

```bash
podman run --secret my_secret,provider=systemd-creds,provider-opts=foo=bar alpine cat /run/secrets/my_secret
```

Upon container creation, Podman will:
1. Identify the requested provider in `containers.conf`.
2. Directly `exec` the associated binary without a shell.
3. Capture the binary's `stdout`.
4. Write the output to an ephemeral `tmpfs`/`tmpfile` associated with the container.
5. Mount the temporary file into the container.
6. Clean up the tmpfile when the container exits.

### 3. The Provider API Contract
To ensure maximum security and simplicity, the provider binary acts as a read-only lookup tool.

#### Invocation
The provider is invoked directly:
`/usr/libexec/podman-secret-provider-systemd-creds`

#### Standard Input / Output
Metadata is passed via `stdin` as JSON to avoid logging or process-tree leaks. Standard error (`stderr`) is captured by Podman for logging and surfacing errors. Standard output (`stdout`) is strictly reserved for the raw secret bytes.

* **Stdin:** A JSON object containing the requested secret name and any provider options passed via the CLI/Quadlet.
```json
{
  "SecretName": "my_secret",
  "ProviderOpts": {
    "foo": "bar"
  }
}
```
* **Stdout (Success - Exit Code 0):** The raw secret bytes. 
```text
<RAW_SECRET_BYTES>
```
* **Stdout/Stderr (Failure - Exit Code >0):** If the binary fails, Podman aborts the container startup and surfaces the `stderr` string to the user.

## **Use cases**
1. **systemd-creds Integration:** Looking up systemd credentials on the fly without staging them into a Podman DB.
2. **Cloud Secret Managers:** dynamically pulling Vault/AWS/Azure secrets directly into memory at container start.
3. **Bootc & Immutable Infrastructure:** Allowing Quadlets to securely reference system-level secrets without requiring pre-exec hooks to generate Podman secrets databases.

## **Target Podman Release**
TBD 

## **Stakeholders**
* [x] Podman Users
* [x] Podman Developers
* [x] Common library

## **Assignee(s)**
@Veector40

## **Impacts**

### **CLI**
* `podman run --secret` and `podman create --secret` will parse new comma-separated keys: `provider` and `provider-opts`.
* Deprecation path: Over time, current external drivers (`shell`, `pass`) may be deprecated in favor of this stateless execution model.

### **Libpod / Common library**
* **Config:** Add `SecretProviders map[string]string` to `pkg/config/config.go`.
* **Runtime:** Update `pkg/secrets` and `libpod/container_internal.go` to intercept `provider=` flags, execute the lookup, and pipe the output to the standard ephemeral secret mount logic.

### **Others**
* Documentation updates required for `containers.conf.5.md` (`[secrets.providers]`) and `podman-run.1.md` (the `--secret` flag options).

## **Test Descriptions:**
1. **Unit Tests in `containers/common`:** Create a mock provider binary that echoes back a constructed secret. Ensure `stdin` parses JSON properly.
2. **E2E Tests in `containers/podman`:** Add a BATS test that registers a mock provider in a temporary `containers.conf` and runs a container with `podman run --secret my_sec,provider=mock`, asserting that the container can read the correct value from `/run/secrets/my_sec`.
