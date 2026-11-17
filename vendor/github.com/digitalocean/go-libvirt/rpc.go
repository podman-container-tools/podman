// Copyright 2018 The go-libvirt Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package libvirt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync/atomic"

	"github.com/digitalocean/go-libvirt/internal/constants"
	"github.com/digitalocean/go-libvirt/internal/event"
	xdr "github.com/digitalocean/go-libvirt/internal/go-xdr/xdr2"
	"github.com/digitalocean/go-libvirt/socket"
)

// ErrUnsupported is returned if a procedure is not supported by libvirt
var ErrUnsupported = errors.New("unsupported procedure requested")

// internal rpc response
type response struct {
	Payload []byte
	Status  uint32
}

// Error reponse from libvirt
type Error struct {
	Code    uint32
	Message string
}

func (e Error) Error() string {
	return e.Message
}

// checkError is used to check whether an error is a libvirtError, and if it is,
// whether its error code matches the one passed in. It will return false if
// these conditions are not met.
func checkError(err error, expectedError ErrorNumber) bool {
	for err != nil {
		e, ok := err.(Error)
		if ok {
			return e.Code == uint32(expectedError)
		}
		err = errors.Unwrap(err)
	}
	return false
}

// IsNotFound detects libvirt's ERR_NO_DOMAIN.
func IsNotFound(err error) bool {
	return checkError(err, ErrNoDomain)
}

// callback sends RPC responses to respective callers.
func (l *Libvirt) callback(id int32, res response) {
	l.cmux.Lock()
	defer l.cmux.Unlock()

	c, ok := l.callbacks[id]
	if !ok {
		return
	}

	c <- res
}

// Route sends incoming packets to their listeners.
func (l *Libvirt) Route(h *socket.Header, buf []byte) {
	// Route events to their respective listener
	var event event.Event

	switch {
	case h.Program == constants.QEMUProgram && h.Procedure == constants.QEMUProcDomainMonitorEvent:
		event = &DomainEvent{}
	case h.Program == constants.Program && h.Procedure == constants.ProcDomainEventCallbackLifecycle:
		event = &DomainEventCallbackLifecycleMsg{}
	}

	if event != nil {
		err := eventDecoder(buf, event)
		if err != nil { // event was malformed, drop.
			return
		}

		l.stream(event)
		return
	}

	// send response to caller
	l.callback(h.Serial, response{Payload: buf, Status: h.Status})
}

// serial provides atomic access to the next sequential request serial number.
func (l *Libvirt) serial() int32 {
	return atomic.AddInt32(&l.s, 1)
}

// stream decodes and relays domain events to their respective listener.
func (l *Libvirt) stream(e event.Event) {
	l.emux.RLock()
	defer l.emux.RUnlock()

	q, ok := l.events[e.GetCallbackID()]
	if !ok {
		return
	}

	q.Push(e)
}

// addStream configures the routing for an event stream.
func (l *Libvirt) addStream(s *event.Stream) {
	l.emux.Lock()
	defer l.emux.Unlock()

	l.events[s.CallbackID] = s
}

// removeStream deletes an event stream. The caller should first notify libvirt
// to stop sending events for this stream. Subsequent calls to removeStream are
// idempotent and return nil.
func (l *Libvirt) removeStream(id int32) error {
	l.emux.Lock()
	defer l.emux.Unlock()

	// if the event is already removed, just return nil
	q, ok := l.events[id]
	if ok {
		delete(l.events, id)
		q.Shutdown()
	}

	return nil
}

// removeAllStreams deletes all event streams.  This is meant to be used to
// clean up only once the underlying connection to libvirt is disconnected and
// thus does not attempt to notify libvirt to stop sending events.
func (l *Libvirt) removeAllStreams() {
	l.emux.Lock()
	defer l.emux.Unlock()

	for _, ev := range l.events {
		ev.Shutdown()
		delete(l.events, ev.CallbackID)
	}
}

// register configures a method response callback
func (l *Libvirt) register(id int32, c chan response) {
	l.cmux.Lock()
	defer l.cmux.Unlock()

	l.callbacks[id] = c
}

// deregister destroys a method response callback. It is the responsibility of
// the caller to manage locking (l.cmux) during this call.
func (l *Libvirt) deregister(id int32) {
	_, ok := l.callbacks[id]
	if !ok {
		return
	}

	close(l.callbacks[id])
	delete(l.callbacks, id)
}

// deregisterAll closes all waiting callback channels. This is used to clean up
// if the connection to libvirt is lost. Callers waiting for responses will
// return an error when the response channel is closed, rather than just
// hanging.
func (l *Libvirt) deregisterAll() {
	l.cmux.Lock()
	defer l.cmux.Unlock()

	for id := range l.callbacks {
		l.deregister(id)
	}
}

// request performs a libvirt RPC request.
// returns response returned by server.
// if response is not OK, decodes error from it and returns it.
func (l *Libvirt) request(proc uint32, program uint32, payload []byte) (response, error) {
	return l.requestStream(proc, program, payload, nil, nil)
}

// requestStream performs a libvirt RPC request. The `out` and `in` parameters
// are optional, and should be nil when RPC endpoints don't return a stream.
func (l *Libvirt) requestStream(proc uint32, program uint32, payload []byte,
	out io.Reader, in io.Writer) (response, error) {
	serial := l.serial()
	c := make(chan response)

	l.register(serial, c)
	defer func() {
		l.cmux.Lock()
		defer l.cmux.Unlock()

		l.deregister(serial)
	}()

	err := l.socket.SendPacket(serial, proc, program, payload, socket.Call,
		socket.StatusOK)
	if err != nil {
		return response{}, err
	}

	resp, err := l.getResponse(c)
	if err != nil {
		return resp, err
	}

	if out != nil {
		abort := make(chan bool)
		outErr := make(chan error)
		go func() {
			outErr <- l.socket.SendStream(serial, proc, program, out, abort)
		}()

		// Even without incoming stream server sends confirmation once all data is received
		resp, err = l.processIncomingStream(c, in)
		if err != nil {
			abort <- true
			return resp, err
		}

		err = <-outErr
		if err != nil {
			return response{}, err
		}
	}

	switch in {
	case nil:
		return resp, nil
	default:
		return l.processIncomingStream(c, in)
	}
}

// processIncomingStream is called once we've successfully sent a request to
// libvirt. It writes the responses back to the stream passed by the caller
// until libvirt sends a packet with statusOK or an error.
func (l *Libvirt) processIncomingStream(c chan response, inStream io.Writer) (response, error) {
	for {
		resp, err := l.getResponse(c)
		if err != nil {
			return resp, err
		}

		// StatusOK indicates end of stream
		if resp.Status == socket.StatusOK {
			return resp, nil
		}

		// FIXME: this smells.
		// StatusError is handled in getResponse, so this must be StatusContinue
		// StatusContinue is only valid here for stream packets
		// libvirtd breaks protocol and returns StatusContinue with an
		// empty response Payload when the stream finishes
		if len(resp.Payload) == 0 {
			return resp, nil
		}
		if inStream != nil {
			_, err = inStream.Write(resp.Payload)
			if err != nil {
				return response{}, err
			}
		}
	}
}

func (l *Libvirt) getResponse(c chan response) (response, error) {
	resp := <-c
	if resp.Status == socket.StatusError {
		return resp, decodeError(resp.Payload)
	}

	return resp, nil
}

// encode XDR encodes the provided data.
func encode(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	_, err := xdr.Marshal(&buf, data)

	return buf.Bytes(), err
}

// decodeError extracts an error message from the provider buffer.
func decodeError(buf []byte) error {
	dec := xdr.NewDecoder(bytes.NewReader(buf))

	e := struct {
		Code     uint32
		DomainID uint32
		Padding  uint8
		Message  string
		Level    uint32
	}{}
	_, err := dec.Decode(&e)
	if err != nil {
		return err
	}

	if strings.Contains(e.Message, "unknown procedure") {
		return ErrUnsupported
	}

	// if libvirt returns ERR_OK, ignore the error
	if ErrorNumber(e.Code) == ErrOk {
		return nil
	}

	return Error{Code: uint32(e.Code), Message: e.Message}
}

// eventDecoder decodes an event from a xdr buffer.
func eventDecoder(buf []byte, e interface{}) error {
	dec := xdr.NewDecoder(bytes.NewReader(buf))
	_, err := dec.Decode(e)
	return err
}

type typedParamDecoder struct{}

// Decode decodes a TypedParam. These are part of the libvirt spec, and not xdr
// proper. TypedParams contain a name, which is called Field for some reason,
// and a Value, which itself has a "discriminant" - an integer enum encoding the
// actual type, and a value, the length of which varies based on the actual
// type.
func (tpd typedParamDecoder) Decode(d *xdr.Decoder, v reflect.Value) (int, error) {
	// Get the name of the typed param first
	name, n, err := d.DecodeString()
	if err != nil {
		return n, err
	}
	val, n2, err := tpd.decodeTypedParamValue(d)
	n += n2
	if err != nil {
		return n, err
	}
	tp := &TypedParam{Field: name, Value: *val}
	v.Set(reflect.ValueOf(*tp))

	return n, nil
}

// decodeTypedParamValue decodes the Value part of a TypedParam.
func (typedParamDecoder) decodeTypedParamValue(d *xdr.Decoder) (*TypedParamValue, int, error) {
	// All TypedParamValues begin with a uint32 discriminant that tells us what
	// type they are.
	discriminant, n, err := d.DecodeUint()
	if err != nil {
		return nil, n, err
	}
	var n2 int
	var tpv *TypedParamValue
	switch discriminant {
	case 1:
		var val int32
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 2:
		var val uint32
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 3:
		var val int64
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 4:
		var val uint64
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 5:
		var val float64
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 6:
		var val int32
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 7:
		var val string
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}

	default:
		err = fmt.Errorf("invalid parameter type %v", discriminant)
	}
	n += n2

	return tpv, n, err
}
