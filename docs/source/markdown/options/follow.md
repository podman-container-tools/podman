####> This option file is used in:
####>   podman logs, pod logs
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--follow**, **-f**

Follow log output.  Default is false.

Note: When following a <<container|pod>> which is removed by `podman <<container|pod>> rm`
or removed on exit (`podman run --rm ...`), there is a chance that the log
file is removed before `podman<< pod|>> logs` reads the final content.
