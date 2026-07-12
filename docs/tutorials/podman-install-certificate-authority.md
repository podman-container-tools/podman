![PODMAN logo](../../logo/podman-logo-source.svg)

Install Certificate Authority
=============================

Organizations may create their own local certificate authority (CA) or acquire one from a third party.  This may mean more than one certificate, such as one or more intermediate certificates and a root certificate, for example.  In any case, it is necessary to add the certificate authority (CA) certificate(s) so that it can be employed for various use cases.

### Automatic import of host certificates

The simplest way to trust the same CAs inside a Podman machine as on the host is to use the `--import-native-ca` flag.  This works on macOS, Windows, and Linux.

When creating a new machine:
```
podman machine init --import-native-ca
```

To enable it on an existing machine:
```
podman machine set --import-native-ca
```

When enabled, the host's trusted CA certificates are automatically imported into the machine on every startup.  If the automated approach does not cover your use case (for example, you need to install a certificate that is not in the host trust store), the manual methods below can be used instead.

### Method one

Certificates may be either individual or concatenated (bundles). The following steps are one method to add such certificates to Podman.  It is assumed that Podman is running and the certificate(s) to be installed are available on an accessible server via curl.  If such access is not possible, an alternative method follows.

First, assuming a running Podman machine, ssh into the machine:
```
podman machine ssh
```

If Podman is running in the default rootless mode, an additional command is required to get to a root shell:

```
[core@localhost ~]$ sudo su -
```

After issuing the above command, the prompt should change to indicate the "root" instead of the "core" user.

Next, while in the machine, change to the directory where the certificate(s) should be installed:
```
[root@localhost ~]# cd /etc/pki/ca-trust/source/anchors
```

Then use curl to download the certificate.  Notes:
* The -k is only necessary if connecting securely to a server for which the certificate is not yet trusted
* The MY-SERVER.COM/SOME-CERTIFICATE.pem should be replaced as appropriate
```
[root@localhost anchors]# curl -k -o some-certificate.pem https://MY-SERVER.COM/SOME-CERTIFICATE.pem
```

Repeat as necessary for multiple certificates.

Once all of the certificates have been downloaded, run the command to add the certificates to the list of trusted CAs:
```
[root@localhost anchors]# update-ca-trust
```

Exit the machine:
```
[root@localhost anchors]# exit
```

If the "sudo su -" command was used to switch to a root shell as described above, an additional exit command is needed to exit the machine:

```
[core@localhost ~]$ exit
```

If the certificates are still not accepted, it might be necessary to restart the Podman machine. To do this, issue the following commands on the host (and not inside the Podman machine):

```
podman machine stop
podman machine start
```

### Alternative Method

If the above method is for some reason not practical or desirable, the certificate may be created using vi.

As above, assuming a running Podman machine, ssh into the machine:

```
podman machine ssh
```

If the prompt starts with "core" instead of "root", switch to a root shell:

```
[core@localhost ~]$ sudo su -
```

Next, change to the directory where the certificate(s) should be installed:
```
[root@localhost ~]# cd /etc/pki/ca-trust/source/anchors
```

Then use vi to create the certificate.
```
[root@localhost ~]# vi SOME-CERTIFICATE.pem
```
After vi opens, copy the certificate to the clipboard, then in insert mode, paste the clipboard contents to vi.  Lastly, save the file and close vi.

Repeat as necessary for multiple certificates.

Once all of the certificates have been created, run the command to add the certificates to the list of trusted CAs:
```
[root@localhost anchors]# update-ca-trust
```

Exit the machine:
```
[root@localhost anchors]# exit
```

If the "sudo su -" command described above was used, an additional exit command is needed:

```
[core@localhost ~]$ exit
```

### Final Notes

The certificate installation will persist during machine restarts.  There is no need to stop and start the machine to begin using the certificate.
