package wemo

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func mSearch(searchTarget string) []byte {
	return []byte("M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 3\r\n" +
		"ST: " + searchTarget + "\r\n\r\n")
}

var echoAddr = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321}

func TestSearchForBelkinDevicesIsAnswered(t *testing.T) {
	srv := New("computadora", 49153, nil)

	resp := string(srv.searchResponse(mSearch(belkinSearchTarget), echoAddr))

	if resp == "" {
		t.Fatal("a Belkin M-SEARCH must be answered")
	}
	for _, want := range []string{
		"HTTP/1.1 200 OK\r\n",
		"ST: " + belkinSearchTarget + "\r\n",
		"USN: " + srv.udn() + "::" + belkinSearchTarget + "\r\n",
		"LOCATION: http://127.0.0.1:49153" + setupPath + "\r\n",
	} {
		if !strings.Contains(resp, want) {
			t.Errorf("response missing %q\ngot:\n%s", want, resp)
		}
	}
	if !strings.HasSuffix(resp, "\r\n\r\n") {
		t.Error("response must end with a blank line")
	}
}

func TestSearchForAllDevicesIsAnsweredAsBelkin(t *testing.T) {
	srv := New("computadora", 49153, nil)

	resp := string(srv.searchResponse(mSearch("ssdp:all"), echoAddr))

	if !strings.Contains(resp, "ST: "+belkinSearchTarget) {
		t.Fatalf("ssdp:all must be answered advertising the Belkin device, got:\n%s", resp)
	}
}

func TestUnrelatedSearchesAreIgnored(t *testing.T) {
	srv := New("computadora", 49153, nil)

	cases := map[string][]byte{
		"other device type": mSearch("urn:schemas-upnp-org:device:MediaRenderer:1"),
		"not an M-SEARCH":   []byte("NOTIFY * HTTP/1.1\r\nNTS: ssdp:alive\r\n\r\n"),
		"no discover MAN":   []byte("M-SEARCH * HTTP/1.1\r\nST: " + belkinSearchTarget + "\r\n\r\n"),
		"empty":             nil,
	}
	for name, payload := range cases {
		if resp := srv.searchResponse(payload, echoAddr); resp != nil {
			t.Errorf("%s: want no response, got:\n%s", name, resp)
		}
	}
}

func TestServeSSDPAnswersOnTheWire(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot open a local UDP socket: %v", err)
	}
	srv := New("computadora", 49153, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.serveSSDP(ctx, conn) }()

	echo, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("cannot dial the responder: %v", err)
	}
	defer echo.Close()
	if _, err := echo.Write(mSearch(belkinSearchTarget)); err != nil {
		t.Fatalf("cannot send M-SEARCH: %v", err)
	}

	buf := make([]byte, 2048)
	echo.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := echo.Read(buf)
	if err != nil {
		t.Fatalf("no SSDP response: %v", err)
	}
	if !strings.HasPrefix(string(buf[:n]), "HTTP/1.1 200 OK") {
		t.Fatalf("unexpected response:\n%s", buf[:n])
	}

	cancel()
	conn.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveSSDP returned %v, want nil after cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveSSDP did not return after cancellation")
	}
}
