% podman-system-connection-list 1

## NAME
podman\-system\-connection\-list - List the destination for the Podman service(s)

## SYNOPSIS
**podman system connection list** [*options*]

**podman system connection ls** [*options*]

## DESCRIPTION
List ssh destination(s) for podman service(s).

## OPTIONS

#### **--format**, **-f**=*format*

Change the default output format.  This can be of a supported type like 'json' or a Go template.
Valid placeholders for the Go template listed below:

| **Placeholder** | **Description**                                                               |
| --------------- | ----------------------------------------------------------------------------- |
| .Default        | Indicates whether connection is the default |
| .Identity       | Path to file containing SSH identity |
| .Name           | Connection Name/Identifier |
| .ReadWrite      | Indicates if this connection can be modified using the system connection commands |
| .TLSCA          | Path to a PEM file containing the certificate authority bundle to verify the server's certificate against. |
| .TLSCert        | Path to a PEM file containing the certificate authority bundle to verify the server's certificate against. |
| .TLSKey         | Path to a PEM file containing the private key matching `.TLSCA` |
| .URI            | URI to podman service. Valid schemes are ssh://[user@]*host*[:port]*Unix domain socket*[?secure=True], unix://*Unix domain socket*, and tcp://localhost[:*port*] |

#### **--quiet**, **-q**

Only show connection names

## EXAMPLE

List system connections:
```
$ podman system connection list
Name URI                                                      Identity	    Default  ReadWrite
deva ssh://root@example.com:/run/podman/podman.sock           ~/.ssh/id_rsa true     true
devb ssh://user@example.com:/run/user/1000/podman/podman.sock ~/.ssh/id_rsa false    true
```
Show connections in JSON format:
```
$ podman system connection list --format json
[
    {
        "Name": "podman-machine-default",
        "URI": "ssh://core@127.0.0.1:53298/run/user/501/podman/podman.sock",
        "Identity": "/Users/ragm/.local/share/containers/podman/machine/machine",
        "IsMachine": true,
        "Default": true,
        "ReadWrite": true
    },
    {
        "Name": "podman-machine-default-root",
        "URI": "ssh://root@127.0.0.1:53298/run/podman/podman.sock",
        "Identity": "/Users/ragm/.local/share/containers/podman/machine/machine",
        "IsMachine": true,
        "Default": false,
        "ReadWrite": true
    }
]
```
Show connection names and URIs:
```
$ podman system connection list --format "{{.Name}}\t{{.URI}}"
podman-machine-default	ssh://core@127.0.0.1:53298/run/user/501/podman/podman.sock
podman-machine-default-root	ssh://root@127.0.0.1:53298/run/podman/podman.sock
```
Show all connection details in a comprehensive format:
```
$ podman system connection list --format "Name: {{.Name}}\nURI: {{.URI}}\nIdentity: {{.Identity}}\nDefault: {{.Default}}\nReadWrite: {{.ReadWrite}}\n---"
Name: podman-machine-default
URI: ssh://core@127.0.0.1:53298/run/user/501/podman/podman.sock
Identity: /Users/ragm/.local/share/containers/podman/machine/machine
Default: true
ReadWrite: true
---
Name: podman-machine-default-root
URI: ssh://root@127.0.0.1:53298/run/podman/podman.sock
Identity: /Users/ragm/.local/share/containers/podman/machine/machine
Default: false
ReadWrite: true
---
```
## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-system(1)](podman-system.1.md)**, **[podman-system-connection(1)](podman-system-connection.1.md)**

## HISTORY
July 2020, Originally compiled by Jhon Honce (jhonce at redhat dot com)
