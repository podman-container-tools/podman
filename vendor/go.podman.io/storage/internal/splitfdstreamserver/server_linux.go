//go:build linux

package splitfdstreamserver

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/varlink/go/varlink"
	"golang.org/x/sys/unix"
)

const (
	interfaceName        = "org.composefs.Oci"
	interfaceDescription = `
# The interface is defined by composefs-rs, which also implements it for
# its own repositories; this is the part a containers-storage image serves.
interface org.composefs.Oci

method GetInfo() -> (features: []string)

# layer_index selects a layer of the image the connection serves: 0 is the
# base layer, in the order of the layers the store records for the image.
# That is not necessarily the order of the rootfs.diff_ids of the image's
# config; see the note on verification below.
type GetLayerParams (
  layer_index: ?int,
  consumer_has_cap_dac_override: ?bool
)

error InternalError (message: string)
error NoSuchLayerIndex (layer_index: int)

# Streams the layer at layer_index.  handle is ignored, and so is any other
# way of naming a layer: only the store knows which local layers make up an
# image.
#
# The reply carries the stream, dir_count directories the stream refers to
# file content in, and a lifetime descriptor.  The layer stays locked, and
# the names in the stream valid, until the client closes every copy of the
# lifetime descriptor.  Holding it holds up writers to the store, so the
# client must close it as soon as it is done, and must not wait for anything
# else, such as another GetLayer, while it holds it: that can deadlock.
#
# The stream is what the store has recorded for the layer.  It is not
# checked against the image's config: the client must verify that what it
# reassembles hashes to the diff ID it expects.
method GetLayer(handle: int, params: GetLayerParams) -> (dir_count: int)
`
)

// ociInterface implements the varlink dispatcher interface for org.composefs.Oci.
type ociInterface struct {
	server  *Server
	backend ImageBackend
	// tmpDir is where streams are written; see openStreamFile.
	tmpDir string
}

func (i *ociInterface) VarlinkGetName() string {
	return interfaceName
}

func (i *ociInterface) VarlinkGetDescription() string {
	return interfaceDescription
}

func (i *ociInterface) VarlinkDispatch(ctx context.Context, call varlink.Call, methodname string) error {
	switch methodname {
	case "GetInfo":
		return i.getInfo(ctx, call)
	case "GetLayer":
		return i.getLayer(ctx, call)
	default:
		return call.ReplyMethodNotFound(ctx, methodname)
	}
}

func (i *ociInterface) getInfo(ctx context.Context, call varlink.Call) error {
	type reply struct {
		Features []string `json:"features"`
	}
	return call.Reply(ctx, &reply{
		Features: []string{"splitdirfdstream-v0"},
	})
}

func (i *ociInterface) getLayer(ctx context.Context, call varlink.Call) error {
	// handle is not decoded: it names a repository for composefs-rs, and
	// a connection here serves exactly one image.
	var params struct {
		Params struct {
			LayerIndex                *int  `json:"layer_index"`
			ConsumerHasCapDACOverride *bool `json:"consumer_has_cap_dac_override"`
		} `json:"params"`
	}
	if err := call.GetParameters(&params); err != nil {
		return call.ReplyInvalidParameter(ctx, "params")
	}
	if params.Params.LayerIndex == nil {
		return call.ReplyInvalidParameter(ctx, "layer_index")
	}
	index := *params.Params.LayerIndex

	layerID, err := i.backend.LayerIDByIndex(index)
	if err != nil {
		// NoSuchLayerIndex carries only the index, so the reason is logged.
		logrus.Debugf("splitfdstream: resolving layer %d: %v", index, err)
		return replyNoSuchLayerIndex(ctx, call, index)
	}

	layer, err := i.backend.OpenLayer(layerID)
	if err != nil {
		return replyOciError(ctx, call, "InternalError", err.Error())
	}

	// A consumer that cannot bypass file permissions could not open a file
	// that is not world-readable, so such content is sent inline instead.
	onlyWorldReadable := params.Params.ConsumerHasCapDACOverride == nil || !*params.Params.ConsumerHasCapDACOverride
	streamFile, err := i.buildStream(layerID, layer, onlyWorldReadable)
	if err != nil {
		layer.Close()
		return replyOciError(ctx, call, "InternalError", err.Error())
	}
	defer streamFile.Close()

	l, keepW, err := newLease(layer)
	if err != nil {
		layer.Close()
		return replyOciError(ctx, call, "InternalError", err.Error())
	}
	// From here on the lease owns the layer.  It ends once every copy of
	// keepW is closed: ours on the way out of here, and the consumer's when
	// it is done with the layer, or right away if the reply never reaches
	// it.
	if !i.server.startLease(l) {
		layer.Close()
		keepW.Close()
		return replyOciError(ctx, call, "InternalError", "the store is no longer serving layers")
	}
	defer keepW.Close()

	// The stream references directories by their index in this array, less
	// the leading stream FD.
	dirFDs := layer.Files().DirFDs()
	allFDs := make([]*os.File, 0, 1+len(dirFDs)+1)
	allFDs = append(allFDs, streamFile)
	allFDs = append(allFDs, dirFDs...)
	allFDs = append(allFDs, keepW)

	type reply struct {
		DirCount int `json:"dir_count"`
	}
	return call.ReplyWithFDs(ctx, &reply{DirCount: len(dirFDs)}, allFDs)
}

// buildStream writes the splitdirfdstream for a layer into an unnamed file,
// and returns it positioned at the start, ready to be handed to the
// consumer.
func (i *ociInterface) buildStream(layerID string, layer Layer, onlyWorldReadable bool) (*os.File, error) {
	streamFile, err := openStreamFile(i.tmpDir)
	if err != nil {
		return nil, err
	}
	if err := convert(streamFile, layer, onlyWorldReadable); err != nil {
		streamFile.Close()
		return nil, fmt.Errorf("converting layer %s: %w", layerID, err)
	}
	if _, err := streamFile.Seek(0, io.SeekStart); err != nil {
		streamFile.Close()
		return nil, fmt.Errorf("rewinding stream: %w", err)
	}
	return streamFile, nil
}

// openStreamFile returns a new file to write a stream to, in tmpDir.
//
// Content that cannot be referenced is copied into the stream, and with a
// consumer that cannot read files that are not world-readable that can be
// a large part of the layer, so the stream is kept on the store's file
// system rather than in memory.  The file is opened with O_TMPFILE, so it
// never has a name: nothing is left behind, and tmpDir can be any directory
// on that file system, including one that something else manages.
func openStreamFile(tmpDir string) (*os.File, error) {
	f, err := os.OpenFile(tmpDir, unix.O_TMPFILE|unix.O_RDWR|unix.O_CLOEXEC|unix.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("creating stream file in %s: %w", tmpDir, err)
	}
	return f, nil
}

func replyNoSuchLayerIndex(ctx context.Context, call varlink.Call, index int) error {
	type errorParams struct {
		LayerIndex int `json:"layer_index"`
	}
	return call.ReplyError(ctx, interfaceName+".NoSuchLayerIndex", &errorParams{LayerIndex: index})
}

func replyOciError(ctx context.Context, call varlink.Call, name, message string) error {
	type errorParams struct {
		Message string `json:"message"`
	}
	return call.ReplyError(ctx, interfaceName+"."+name, &errorParams{Message: message})
}
