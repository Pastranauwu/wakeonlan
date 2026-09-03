// Package wemo emulates a Belkin WeMo socket on the local network so that an
// Amazon Echo discovers it and controls it without a skill, an AWS account or
// any port open to the internet.
package wemo

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	deviceType       = "urn:Belkin:device:controllee:1"
	serviceType      = "urn:Belkin:service:basicevent:1"
	setupPath        = "/setup.xml"
	controlPath      = "/upnp/control/basicevent1"
	eventPath        = "/upnp/event/basicevent1"
	eventServicePath = "/eventservice.xml"

	// maxEnvelopeBytes caps the SOAP body read from the network: any host on
	// the LAN can reach the control endpoint.
	maxEnvelopeBytes = 64 << 10

	readHeaderTimeout = 2 * time.Second
	shutdownTimeout   = 3 * time.Second
)

// Server is an emulated WeMo socket. Turning it on runs the callback given to
// New; turning it off only records the state, since there is nothing to undo.
type Server struct {
	name   string
	port   int
	onOn   func() error
	serial string

	mu sync.Mutex
	on bool
}

// New builds a socket named name, reachable over HTTP on port, that runs onOn
// every time Alexa is asked to turn it on. Recent Echo firmware only completes
// discovery when port is 80 or falls in the 49153-49160 range.
func New(name string, port int, onOn func() error) *Server {
	return &Server{name: name, port: port, onOn: onOn, serial: serialFor(name)}
}

// ListenAndServe answers SSDP discovery and WeMo control requests until ctx is
// cancelled or one of the two listeners fails.
func (s *Server) ListenAndServe(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("wemo: listen on tcp port %d: %w", s.port, err)
	}
	s.port = listener.Addr().(*net.TCPAddr).Port

	discovery, err := listenSSDP()
	if err != nil {
		listener.Close()
		return fmt.Errorf("wemo: join the ssdp multicast group: %w", err)
	}

	control := &http.Server{Handler: s.handler(), ReadHeaderTimeout: readHeaderTimeout}
	failures := make(chan error, 2)
	go func() { failures <- ignore(control.Serve(listener), http.ErrServerClosed) }()
	go func() { failures <- s.serveSSDP(ctx, discovery) }()

	var reported []error
	select {
	case <-ctx.Done():
	case err := <-failures:
		reported = append(reported, err)
	}

	discovery.Close()
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	reported = append(reported, control.Shutdown(shutdown))
	for len(reported) < 3 {
		reported = append(reported, <-failures)
	}
	return errors.Join(reported...)
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+setupPath, s.describeDevice)
	mux.HandleFunc("GET "+eventServicePath, serveXML(eventServiceXML))
	mux.HandleFunc("POST "+controlPath, s.control)
	// Any method: the event subscription an Echo may open is a SUBSCRIBE, and
	// acknowledging it is enough since this socket never sends events.
	mux.HandleFunc(eventPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func (s *Server) describeDevice(w http.ResponseWriter, _ *http.Request) {
	serveXML(fmt.Sprintf(setupXML, xmlEscape(s.name), s.udn(), s.serial, s.binaryState()))(w, nil)
}

// control implements the two basicevent actions an Echo uses: SetBinaryState
// to switch the socket and GetBinaryState to poll it.
func (s *Server) control(w http.ResponseWriter, r *http.Request) {
	var envelope struct {
		Set *struct {
			BinaryState string `xml:"BinaryState"`
		} `xml:"Body>SetBinaryState"`
		Get *struct{} `xml:"Body>GetBinaryState"`
	}
	if err := xml.NewDecoder(http.MaxBytesReader(w, r.Body, maxEnvelopeBytes)).Decode(&envelope); err != nil {
		http.Error(w, "malformed soap envelope", http.StatusBadRequest)
		return
	}

	switch {
	case envelope.Set != nil:
		if err := s.switchTo(strings.TrimSpace(envelope.Set.BinaryState) == "1"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		serveXML(fmt.Sprintf(actionResponseXML, "SetBinaryState", s.binaryState(), "SetBinaryState"))(w, nil)
	case envelope.Get != nil:
		serveXML(fmt.Sprintf(actionResponseXML, "GetBinaryState", s.binaryState(), "GetBinaryState"))(w, nil)
	default:
		http.Error(w, "unsupported soap action", http.StatusBadRequest)
	}
}

// switchTo runs the callback on every on request, even when the socket is
// already on: the state we keep is a guess about the real machine, and asking
// twice must be able to wake a machine that went back to sleep on its own. The
// callback runs outside the lock, so a slow wake does not stall discovery.
func (s *Server) switchTo(on bool) error {
	if on && s.onOn != nil {
		if err := s.onOn(); err != nil {
			return fmt.Errorf("wemo: turning %q on: %w", s.name, err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.on = on
	return nil
}

func (s *Server) binaryState() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.on {
		return "1"
	}
	return "0"
}

func (s *Server) udn() string { return "uuid:Socket-1_0-" + s.serial }

// serialFor derives a serial from the name so that a restart keeps the same
// identity and Alexa does not end up with a duplicated device.
func serialFor(name string) string {
	digest := fnv.New64a()
	digest.Write([]byte(name))
	return fmt.Sprintf("%016X", digest.Sum64())
}

func serveXML(document string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		fmt.Fprint(w, document)
	}
}

func xmlEscape(text string) string {
	var escaped strings.Builder
	xml.EscapeText(&escaped, []byte(text))
	return escaped.String()
}

func ignore(err, expected error) error {
	if errors.Is(err, expected) {
		return nil
	}
	return err
}
