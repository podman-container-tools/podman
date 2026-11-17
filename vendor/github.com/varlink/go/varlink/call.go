package varlink

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Call is a method call retrieved by a Service. The connection from the
// client can be terminated by returning an error from the call instead
// of sending a reply or error reply.
type Call struct {
	Conn      ReadWriterContext
	Request   *[]byte
	In        *serviceCall
	Continues bool
	Upgrade   bool
	// inFDs points at the file descriptors received from the client. It is a
	// pointer, not a slice, because dispatcher.VarlinkDispatch takes Call by
	// value: a handler only ever sees a copy of this Call. Without the
	// indirection, a handler's RequestFDs() would clear its own copy's field
	// and leave the caller's original Call still holding (and later closing)
	// the very fds the handler claimed. The pointer is shared across all
	// copies, so claiming the fds through any copy is visible to every other.
	inFDs *[]*os.File
}

// WantsMore indicates if the calling client accepts more than one reply to this method call.
func (c *Call) WantsMore() bool {
	return c.In.More
}

// WantsUpgrade indicates that the calling client wants the connection to be upgraded.
func (c *Call) WantsUpgrade() bool {
	return c.In.Upgrade
}

// IsOneway indicate that the calling client does not expect a reply.
func (c *Call) IsOneway() bool {
	return c.In.Oneway
}

// GetParameters retrieves the method call parameters.
func (c *Call) GetParameters(p interface{}) error {
	if c.In.Parameters == nil {
		return fmt.Errorf("empty parameters")
	}
	return json.Unmarshal(*c.In.Parameters, p)
}

// sendMessageWithFDs marshals r, appends the NUL framing byte, and writes it
// to the connection. If fds is non-empty and the connection implements FDReadWriter,
// the fds are attached as SCM_RIGHTS ancillary data.
//
// Ownership: the caller retains ownership of fds; the library never closes them.
func (c *Call) sendMessageWithFDs(ctx context.Context, r *serviceReply, fds []*os.File) error {
	if c.In.Oneway {
		return nil
	}

	b, err := json.Marshal(r)
	if err != nil {
		return err
	}

	b = append(b, 0)

	if len(fds) > 0 {
		fdw, ok := c.Conn.(FDReadWriter)
		if !ok {
			return ErrFDsUnsupported
		}
		_, err = fdw.WriteWithFDs(ctx, b, fds)
	} else {
		_, err = c.Conn.Write(ctx, b)
	}
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

func (c *Call) sendMessage(ctx context.Context, r *serviceReply) error {
	return c.sendMessageWithFDs(ctx, r, nil)
}

// RequestFDs returns the file descriptors received from the client with this
// method call and clears the internal reference (transfers ownership to the caller).
// The caller is responsible for closing the returned files.
// Returns nil if no fds were received or they have already been claimed.
func (c *Call) RequestFDs() []*os.File {
	if c.inFDs == nil {
		return nil
	}
	fds := *c.inFDs
	*c.inFDs = nil
	return fds
}

// Reply sends a reply to this method call.
func (c *Call) Reply(ctx context.Context, parameters interface{}) error {
	if !c.Continues {
		return c.sendMessage(ctx, &serviceReply{
			Parameters: parameters,
		})
	}

	if !c.In.More {
		return fmt.Errorf("call did not set more, it does not expect continues")
	}

	return c.sendMessage(ctx, &serviceReply{
		Continues:  true,
		Parameters: parameters,
	})
}

// ReplyWithFDs sends a reply to this method call, attaching fds as SCM_RIGHTS
// ancillary data on the reply message.
//
// On non-unix transports, passing len(fds) > 0 returns ErrFDsUnsupported.
// Ownership: the caller retains ownership of fds; the library never closes them.
func (c *Call) ReplyWithFDs(ctx context.Context, parameters interface{}, fds []*os.File) error {
	if !c.Continues {
		return c.sendMessageWithFDs(ctx, &serviceReply{
			Parameters: parameters,
		}, fds)
	}

	if !c.In.More {
		return fmt.Errorf("call did not set more, it does not expect continues")
	}

	return c.sendMessageWithFDs(ctx, &serviceReply{
		Continues:  true,
		Parameters: parameters,
	}, fds)
}

// ReplyError sends an error reply to this method call.
func (c *Call) ReplyError(ctx context.Context, name string, parameters interface{}) error {
	r := strings.LastIndex(name, ".")
	if r <= 0 {
		return fmt.Errorf("invalid error name")
	}
	if name[:r] == "org.varlink.service" {
		return fmt.Errorf("refused to send org.varlink.service errors")
	}
	return c.sendMessage(ctx, &serviceReply{
		Error:      name,
		Parameters: parameters,
	})
}
