package wemo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"
)

// belkinSearchTarget is the search target an Echo uses to look for WeMo
// devices; ssdp:all sweeps also have to be answered with it.
const belkinSearchTarget = "urn:Belkin:device:**"

var ssdpGroup = &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}

// ponytail: nil interface joins the group on the interface of the default
// route only, so on a multi-homed host whose default route is not the LAN the
// Echo lives on, discovery never arrives. Enumerate net.Interfaces and open one
// socket per multicast-capable interface if that turns out to matter.
func listenSSDP() (*net.UDPConn, error) {
	return net.ListenMulticastUDP("udp4", nil, ssdpGroup)
}

// serveSSDP answers discovery requests until conn is closed. A read error once
// ctx is done is the expected way out, not a failure.
func (s *Server) serveSSDP(ctx context.Context, conn *net.UDPConn) error {
	datagram := make([]byte, 2048)
	for {
		read, from, err := conn.ReadFromUDP(datagram)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("wemo: read ssdp datagram: %w", err)
		}
		response := s.searchResponse(datagram[:read], from)
		if response == nil {
			continue
		}
		if _, err := conn.WriteToUDP(response, from); err != nil {
			slog.Warn("wemo: cannot answer ssdp discovery", "echo", from, "error", err)
		}
	}
}

// searchResponse returns the unicast reply owed to an M-SEARCH, or nil when the
// datagram is not a discovery request this socket should answer.
func (s *Server) searchResponse(datagram []byte, from *net.UDPAddr) []byte {
	request := string(datagram)
	if !strings.HasPrefix(request, "M-SEARCH ") || !strings.Contains(request, `"ssdp:discover"`) {
		return nil
	}
	if !strings.Contains(request, belkinSearchTarget) && !strings.Contains(request, "ssdp:all") {
		return nil
	}
	reachableAt := addressReachableFrom(from.IP)
	if reachableAt == "" {
		return nil
	}

	return []byte(fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
		"CACHE-CONTROL: max-age=86400\r\n"+
		"DATE: %s\r\n"+
		"EXT:\r\n"+
		"LOCATION: http://%s:%d%s\r\n"+
		"OPT: \"http://schemas.upnp.org/upnp/1/0/\"; ns=01\r\n"+
		"01-NLS: %s\r\n"+
		"SERVER: Unspecified, UPnP/1.0, Unspecified\r\n"+
		"X-User-Agent: redsonic\r\n"+
		"ST: %s\r\n"+
		"USN: %s::%s\r\n\r\n",
		time.Now().UTC().Format(time.RFC1123),
		reachableAt, s.port, setupPath,
		s.serial,
		belkinSearchTarget,
		s.udn(), belkinSearchTarget))
}

// addressReachableFrom picks the local address the Echo can dial back, which on
// a host with several interfaces is the one routing towards the Echo itself.
func addressReachableFrom(echo net.IP) string {
	route, err := net.Dial("udp4", net.JoinHostPort(echo.String(), "1900"))
	if err != nil {
		return ""
	}
	defer route.Close()
	return route.LocalAddr().(*net.UDPAddr).IP.String()
}
