/*
Package varlink provides varlink client and server implementations. See http://varlink.org
for more information about varlink.

Example varlink interface definition in a org.example.this.varlink file:
	interface org.example.this

	method Ping(in: string) -> (out: string)

Generated Go module in a orgexamplethis/orgexamplethis.go file. The generated module
provides reply methods for all methods specified in the varlink interface description.
The stub implementations return a MethodNotImplemented error; the service implementation
using this module will override the methods with its own implementation.
	// Generated with github.com/varlink/go/cmd/varlink-go-interface-generator
	package orgexamplethis

	import "github.com/varlink/go/varlink"

	type orgexamplethisInterface interface {
		Ping(c VarlinkCall, in string) error
	}

	type VarlinkCall struct{ varlink.Call }

	func (c *VarlinkCall) ReplyPing(out string) error {
		var out struct {
			Out string `json:"out,omitempty"`
		}
		out.Out = out
		return c.Reply(&out)
	}

	func (s *VarlinkInterface) Ping(c VarlinkCall, in string) error {
		return c.ReplyMethodNotImplemented("Ping")
	}

	[...]

Service implementing the interface and its method:
	import ("orgexamplethis")

	type Data struct {
		orgexamplethis.VarlinkInterface
		data string
	}

	data := Data{data: "test"}

	func (d *Data) Ping(call orgexamplethis.VarlinkCall, ping string) error {
		return call.ReplyPing(ping)
	}

	service, _ = varlink.NewService(
	        "Example",
	        "This",
	        "1",
	         "https://example.org/this",
	)

	service.RegisterInterface(orgexamplethis.VarlinkNew(&data))
	err := service.Listen("unix:/run/org.example.this", 0)

Client connecting to a service:

	ctx := context.Background()
	conn, err := varlink.NewConnection(ctx, "unix:/run/org.example.this")
	if err != nil {
		// handle error
	}

	defer conn.Close()

Custom dialer for connecting to a Unix socket on a remote host via SSH:

	import "golang.org/x/crypto/ssh"

	// SSH into remote host
	sshConfig := &ssh.ClientConfig{
		User: "user",
		Auth: []ssh.AuthMethod{
			ssh.Password("password"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // don't do this
	}

	sshClient, err := ssh.Dial("tcp", "remote.example.com:22", sshConfig)
	if err != nil {
		// handle error
	}
	defer sshClient.Close()

	// Custom dialer that connects through SSH
	type sshDialer struct {
		client *ssh.Client
	}

	func (d *sshDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
		return d.client.Dial(network, addr)
	}

	// Connect to Unix socket on the remote host
	conn, err := varlink.NewConnectionWithDialer(ctx, "unix:/run/org.example.service", &sshDialer{sshClient})
*/
package varlink
