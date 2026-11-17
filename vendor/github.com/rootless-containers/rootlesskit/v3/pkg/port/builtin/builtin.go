package builtin

import (
	"io"

	"github.com/rootless-containers/rootlesskit/v3/pkg/port"
	"github.com/rootless-containers/rootlesskit/v3/pkg/port/builtin/child"
	"github.com/rootless-containers/rootlesskit/v3/pkg/port/builtin/parent"
)

var (
	NewParentDriver func(logWriter io.Writer, stateDir string, sourceIPTransparent bool, sourceIPTransparentBackend string) (port.ParentDriver, error) = parent.NewDriver
	NewChildDriver  func(logWriter io.Writer) port.ChildDriver                                                                                         = child.NewDriver
)

// Available indicates whether this port driver is compiled in (used for generating help text)
const Available = true
