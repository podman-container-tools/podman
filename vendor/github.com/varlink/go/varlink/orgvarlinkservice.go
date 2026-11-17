package varlink

import "context"

// The requested interface was not found.
type InterfaceNotFound struct {
	Interface string `json:"interface"`
}

func (e InterfaceNotFound) Error() string {
	return "org.varlink.service.InterfaceNotFound"
}

// The requested method was not found
type MethodNotFound struct {
	Method string `json:"method"`
}

func (e MethodNotFound) Error() string {
	return "org.varlink.service.MethodNotFound"
}

// The interface defines the requested method, but the service does not
// implement it.
type MethodNotImplemented struct {
	Method string `json:"method"`
}

func (e MethodNotImplemented) Error() string {
	return "org.varlink.service.MethodNotImplemented"
}

// One of the passed parameters is invalid.
type InvalidParameter struct {
	Parameter string `json:"parameter"`
}

func (e InvalidParameter) Error() string {
	return "org.varlink.service.InvalidParameter"
}

func doReplyError(ctx context.Context, c *Call, name string, parameters interface{}) error {
	return c.sendMessage(ctx, &serviceReply{
		Error:      name,
		Parameters: parameters,
	})
}

// ReplyInterfaceNotFound sends a org.varlink.service error reply to this method call
func (c *Call) ReplyInterfaceNotFound(ctx context.Context, interfaceA string) error {
	var out InterfaceNotFound
	out.Interface = interfaceA
	return doReplyError(ctx, c, "org.varlink.service.InterfaceNotFound", &out)
}

// ReplyMethodNotFound sends a org.varlink.service error reply to this method call
func (c *Call) ReplyMethodNotFound(ctx context.Context, method string) error {
	var out MethodNotFound
	out.Method = method
	return doReplyError(ctx, c, "org.varlink.service.MethodNotFound", &out)
}

// ReplyMethodNotImplemented sends a org.varlink.service error reply to this method call
func (c *Call) ReplyMethodNotImplemented(ctx context.Context, method string) error {
	var out MethodNotImplemented
	out.Method = method
	return doReplyError(ctx, c, "org.varlink.service.MethodNotImplemented", &out)
}

// ReplyInvalidParameter sends a org.varlink.service error reply to this method call
func (c *Call) ReplyInvalidParameter(ctx context.Context, parameter string) error {
	var out InvalidParameter
	out.Parameter = parameter
	return doReplyError(ctx, c, "org.varlink.service.InvalidParameter", &out)
}

func (c *Call) replyGetInfo(ctx context.Context, vendor string, product string, version string, url string, interfaces []string) error {
	var out struct {
		Vendor     string   `json:"vendor,omitempty"`
		Product    string   `json:"product,omitempty"`
		Version    string   `json:"version,omitempty"`
		URL        string   `json:"url,omitempty"`
		Interfaces []string `json:"interfaces,omitempty"`
	}
	out.Vendor = vendor
	out.Product = product
	out.Version = version
	out.URL = url
	out.Interfaces = interfaces
	return c.Reply(ctx, &out)
}

func (c *Call) replyGetInterfaceDescription(ctx context.Context, description string) error {
	var out struct {
		Description string `json:"description,omitempty"`
	}
	out.Description = description
	return c.Reply(ctx, &out)
}

func (s *Service) orgvarlinkserviceDispatch(ctx context.Context, c Call, methodname string) error {
	switch methodname {
	case "GetInfo":
		return s.getInfo(ctx, c)
	case "GetInterfaceDescription":
		var in struct {
			Interface string `json:"interface"`
		}
		err := c.GetParameters(&in)
		if err != nil {
			return c.ReplyInvalidParameter(ctx, "parameters")
		}
		return s.getInterfaceDescription(ctx, c, in.Interface)

	default:
		return c.ReplyMethodNotFound(ctx, methodname)
	}
}

func (s *orgvarlinkserviceInterface) VarlinkDispatch(ctx context.Context, call Call, methodname string) error {
	return nil
}

func (s *orgvarlinkserviceInterface) VarlinkGetName() string {
	return `org.varlink.service`
}

func (s *orgvarlinkserviceInterface) VarlinkGetDescription() string {
	return `# The Varlink Service Interface is provided by every varlink service. It
# describes the service and the interfaces it implements.
interface org.varlink.service

# Get a list of all the interfaces a service provides and information
# about the implementation.
method GetInfo() -> (
  vendor: string,
  product: string,
  version: string,
  url: string,
  interfaces: []string
)

# Get the description of an interface that is implemented by this service.
method GetInterfaceDescription(interface: string) -> (description: string)

# The requested interface was not found.
error InterfaceNotFound (interface: string)

# The requested method was not found
error MethodNotFound (method: string)

# The interface defines the requested method, but the service does not
# implement it.
error MethodNotImplemented (method: string)

# One of the passed parameters is invalid.
error InvalidParameter (parameter: string)`
}

type orgvarlinkserviceInterface struct{}

func orgvarlinkserviceNew() *orgvarlinkserviceInterface {
	return &orgvarlinkserviceInterface{}
}
