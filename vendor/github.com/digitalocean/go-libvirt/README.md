libvirt
[![GoDoc](http://godoc.org/github.com/digitalocean/go-libvirt?status.svg)](http://godoc.org/github.com/digitalocean/go-libvirt)
[![Build Status](https://github.com/digitalocean/go-libvirt/actions/workflows/main.yml/badge.svg)](https://github.com/digitalocean/go-libvirt/actions/)
[![Report Card](https://goreportcard.com/badge/github.com/digitalocean/go-libvirt)](https://goreportcard.com/report/github.com/digitalocean/go-libvirt)
====

Package `go-libvirt` provides a pure Go interface for interacting with libvirt.

Rather than using libvirt's C bindings, this package makes use of
libvirt's RPC interface, as documented [here](https://libvirt.org/kbase/internals/rpc.html).
Connections to the libvirt server may be local, or remote. RPC packets are encoded
using the XDR standard as defined by [RFC 4506](https://tools.ietf.org/html/rfc4506.html).

libvirt's RPC interface is quite extensive, and changes from one version to the
next, so this project uses a pair of code generators to build the go bindings.
The code generators should be run whenever you want to build go-libvirt for a
new version of libvirt. See the next section for directions on re-generating
go-libvirt.

[Pull requests are welcome](https://github.com/digitalocean/go-libvirt/blob/master/CONTRIBUTING.md)!

Running the Code Generators
---------------------------

The code generator doesn't run automatically when you build go-libvirt. It's
meant to be run manually any time you change the version of libvirt you're
using. When you download go-libvirt it will come with generated files
corresponding to a particular version of libvirt. You can use the library as-is,
but the generated code may be missing libvirt functions, if you're using a newer
version of libvirt, or it may have extra functions that will return
'unimplemented' errors if you try to call them. If this is a problem, you should
re-run the code generator. To do this, follow these steps:

- First, download a copy of the libvirt sources corresponding to the version you
  want to use.
- Change directories into where you've unpacked your distribution of libvirt.
- The second step depends on the version of libvirt you'd like to build against.
  It's not necessary to actually build libvirt, but it is necessary to run libvirt's
  "configure" step because it generates required files.
  - For libvirt < v6.7.0:
    - `$ mkdir build; cd build`
    - `$ ../autogen.sh`
  - For libvirt >= v6.7.0:
    - `$ meson setup build`
- Finally, set the environment variable `LIBVIRT_SOURCE` to the directory you
  put libvirt into, and run `go generate ./...` from the go-libvirt directory.
  This runs both of the go-libvirt's code generators.

How to Use This Library
-----------------------

Once you've vendored go-libvirt into your project, you'll probably want to call
some libvirt functions. There's some example code below showing how to connect
to libvirt and make one such call, but once you get past the introduction you'll
next want to call some other libvirt functions. How do you find them?

Start with the [libvirt API reference](https://libvirt.org/html/index.html).
Let's say you want to gracefully shutdown a VM, and after reading through the
libvirt docs you determine that virDomainShutdown() is the function you want to
call to do that. Where's that function in go-libvirt? We transform the names
slightly when building the go bindings. There's no need for a global prefix like
"vir" in Go, since all our functions are inside the package namespace, so we
drop it. That means the Go function for `virDomainShutdown()` is just `DomainShutdown()`,
and sure enough, you can find the Go function `DomainShutdown()` in libvirt.gen.go,
with parameters and return values equivalent to those documented in the API
reference.

Suppose you then decide you need more control over your shutdown, so you switch
over to `virDomainShutdownFlags()`. As its name suggests, this function takes a
flag parameter which has possible values specified in an enum called
`virDomainShutdownFlagValues`. Flag types like this are a little tricky for the
code generator, because the C functions just take an integer type - only the
libvirt documentation actually ties the flags to the enum types. In most cases
though we're able to generate a wrapper function with a distinct flag type,
making it easier for Go tooling to suggest possible flag values while you're
working. Checking the documentation for this function:

`godoc github.com/digitalocean/go-libvirt DomainShutdownFlags`

returns this:

`func (l *Libvirt) DomainShutdownFlags(Dom Domain, Flags DomainShutdownFlagValues) (err error)`

If you want to see the possible flag values, `godoc` can help again:

```
$ godoc github.com/digitalocean/go-libvirt DomainShutdownFlagValues

type DomainShutdownFlagValues int32
    DomainShutdownFlagValues as declared in libvirt/libvirt-domain.h:1121

const (
    DomainShutdownDefault      DomainShutdownFlagValues = iota
    DomainShutdownAcpiPowerBtn DomainShutdownFlagValues = 1
    DomainShutdownGuestAgent   DomainShutdownFlagValues = 2
    DomainShutdownInitctl      DomainShutdownFlagValues = 4
    DomainShutdownSignal       DomainShutdownFlagValues = 8
    DomainShutdownParavirt     DomainShutdownFlagValues = 16
)
    DomainShutdownFlagValues enumeration from libvirt/libvirt-domain.h:1121
```

One other suggestion: most of the code in go-libvirt is now generated, but a few
hand-written routines still exist in libvirt.go, and wrap calls to the generated
code with slightly different parameters or return values. We suggest avoiding
these hand-written routines and calling the generated routines in libvirt.gen.go
instead. Over time these handwritten routines will be removed from go-libvirt.

Warning
-------

While these package are reasonably well-tested and have seen some use inside of
DigitalOcean, there may be subtle bugs which could cause the packages to act
in unexpected ways.  Use at your own risk!

In addition, the API is not considered stable at this time.  If you would like
to include package `libvirt` in a project, we highly recommend vendoring it into
your project.

Example
-------

```go
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/digitalocean/go-libvirt"
)

func main() {
	// This dials libvirt on the local machine, but you can substitute the first
	// two parameters with "tcp", "<ip address>:<port>" to connect to libvirt on
	// a remote machine.
	c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
	if err != nil {
		log.Fatalf("failed to dial libvirt: %v", err)
	}

	l := libvirt.New(c)
	if err := l.Connect(); err != nil {
		log.Fatalf("failed to connect: %v", err)
	}

	v, err := l.Version()
	if err != nil {
		log.Fatalf("failed to retrieve libvirt version: %v", err)
	}
	fmt.Println("Version:", v)

	domains, err := l.Domains()
	if err != nil {
		log.Fatalf("failed to retrieve domains: %v", err)
	}

	fmt.Println("ID\tName\t\tUUID")
	fmt.Printf("--------------------------------------------------------\n")
	for _, d := range domains {
		fmt.Printf("%d\t%s\t%x\n", d.ID, d.Name, d.UUID)
	}

	if err := l.Disconnect(); err != nil {
		log.Fatalf("failed to disconnect: %v", err)
	}
}

```

```
Version: 1.3.4
ID	Name		UUID
--------------------------------------------------------
1	Test-1		dc329f87d4de47198cfd2e21c6105b01
2	Test-2		dc229f87d4de47198cfd2e21c6105b01
```

Example (Connect to libvirt via TLS over TCP)
-------

```go
package main

import (
        "crypto/tls"
        "crypto/x509"

        "fmt"
        "io/ioutil"
        "log"

        "github.com/digitalocean/go-libvirt"
)

func main() {
        // This dials libvirt on the local machine
        // It connects to libvirt via TLS over TCP
        // To connect to a remote machine, you need to have the ca/cert/key of it.
        keyFileXML, err := ioutil.ReadFile("/etc/pki/libvirt/private/clientkey.pem")
        if err != nil {
                log.Fatalf("%v", err)
        }

        certFileXML, err := ioutil.ReadFile("/etc/pki/libvirt/clientcert.pem")
        if err != nil {
                log.Fatalf("%v", err)
        }

        caFileXML, err := ioutil.ReadFile("/etc/pki/CA/cacert.pem")
        if err != nil {
                log.Fatalf("%v", err)
        }
        cert, err := tls.X509KeyPair([]byte(certFileXML), []byte(keyFileXML))
        if err != nil {
                log.Fatalf("%v", err)
        }

        roots := x509.NewCertPool()
        roots.AppendCertsFromPEM([]byte(caFileXML))

        config := &tls.Config{
                Certificates: []tls.Certificate{cert},
                RootCAs:      roots,
        }

        // Use host name or IP which is valid in certificate
        addr := "10.10.10.10"
        port := "16514"
        c, err := tls.Dial("tcp", addr + ":" + port, config)
        if err != nil {
                log.Fatalf("failed to dial libvirt: %v", err)
        }

        // Drop a byte before libvirt.New(c)
        // More details at https://github.com/digitalocean/go-libvirt/issues/89
        // Remove this line if the issue does not exist any more
        c.Read(make([]byte, 1))

        l := libvirt.New(c)
        if err := l.Connect(); err != nil {
                log.Fatalf("failed to connect: %v", err)
        }

        v, err := l.Version()
        if err != nil {
                log.Fatalf("failed to retrieve libvirt version: %v", err)
        }
        fmt.Println("Version:", v)

        // Return both running and stopped VMs
        flags := libvirt.ConnectListDomainsActive | libvirt.ConnectListDomainsInactive
        domains, _, err := l.ConnectListAllDomains(1, flags)
        if err != nil {
                log.Fatalf("failed to retrieve domains: %v", err)
        }

        fmt.Println("ID\tName\t\tUUID")
        fmt.Println("--------------------------------------------------------")
        for _, d := range domains {
                fmt.Printf("%d\t%s\t%x\n", d.ID, d.Name, d.UUID)
        }

        if err := l.Disconnect(); err != nil {
                log.Fatalf("failed to disconnect: %v", err)
        }
}
```

Running the Integration Tests
-----------------------------

Github actions workflows are defined in .github/worflows and can be triggered
manually in the github GUI after pushing a branch.  There are not currently
convenient scripts for setting up and running integration tests locally, but
installing libvirt and defining only the artifacts described by the files in
testdata should be sufficient to be able to run the integration test file against.

