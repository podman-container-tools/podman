####> This option file is used in:
####>   podman artifact pull, build, create, farm build, podman-image.unit.5.md.in, pull, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
<< if is_quadlet >>
### `DecryptionKey=key[:passphrase]`
<< else >>
#### **--decryption-key**=*key[:passphrase]*
<< endif >>

The [key[:passphrase]] to be used for decryption of images. Key can point to keys and/or certificates. Decryption is tried with all keys. If the key is protected by a passphrase, it is required to be passed in the argument and omitted otherwise.
