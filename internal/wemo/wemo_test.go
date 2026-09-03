package wemo

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func setBinaryStateBody(state string) string {
	return `<?xml version="1.0" encoding="utf-8"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">` +
		`<s:Body><u:SetBinaryState xmlns:u="urn:Belkin:service:basicevent:1">` +
		`<BinaryState>` + state + `</BinaryState>` +
		`</u:SetBinaryState></s:Body></s:Envelope>`
}

const getBinaryStateBody = `<?xml version="1.0" encoding="utf-8"?>` +
	`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">` +
	`<s:Body><u:GetBinaryState xmlns:u="urn:Belkin:service:basicevent:1">` +
	`<BinaryState>1</BinaryState>` +
	`</u:GetBinaryState></s:Body></s:Envelope>`

func postSOAP(t *testing.T, handler http.Handler, action, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, controlPath, strings.NewReader(body))
	req.Header.Set("SOAPACTION", `"urn:Belkin:service:basicevent:1#`+action+`"`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestSerialIsStableForTheSameName(t *testing.T) {
	if serialFor("computadora") != serialFor("computadora") {
		t.Fatal("serial must be stable across calls so Alexa does not duplicate the device")
	}
	if serialFor("computadora") == serialFor("otra") {
		t.Fatal("different names must yield different serials")
	}
}

func TestSetupXMLDescribesTheDevice(t *testing.T) {
	srv := New("Sala & Cocina", 49153, nil)

	req := httptest.NewRequest(http.MethodGet, setupPath, nil)
	rec := httptest.NewRecorder()
	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Sala &amp; Cocina",
		"<UDN>" + srv.udn() + "</UDN>",
		"<serialNumber>" + srv.serial + "</serialNumber>",
		"<deviceType>" + deviceType + "</deviceType>",
		"<controlURL>" + controlPath + "</controlURL>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("setup.xml missing %q\ngot:\n%s", want, body)
		}
	}
}

func TestEventServiceXMLIsServed(t *testing.T) {
	srv := New("computadora", 49153, nil)

	req := httptest.NewRequest(http.MethodGet, eventServicePath, nil)
	rec := httptest.NewRecorder()
	srv.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SetBinaryState") {
		t.Error("eventservice.xml must declare the SetBinaryState action")
	}
}

func TestTurningOnInvokesTheCallback(t *testing.T) {
	calls := 0
	srv := New("computadora", 49153, func() error { calls++; return nil })

	rec := postSOAP(t, srv.handler(), "SetBinaryState", setBinaryStateBody("1"))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	if calls != 1 {
		t.Fatalf("callback ran %d times, want 1", calls)
	}
	if !strings.Contains(rec.Body.String(), "<BinaryState>1</BinaryState>") {
		t.Errorf("response must echo the new state, got:\n%s", rec.Body.String())
	}
}

func TestTurningOnAgainInvokesTheCallbackAgain(t *testing.T) {
	calls := 0
	srv := New("computadora", 49153, func() error { calls++; return nil })

	postSOAP(t, srv.handler(), "SetBinaryState", setBinaryStateBody("1"))
	postSOAP(t, srv.handler(), "SetBinaryState", setBinaryStateBody("1"))

	if calls != 2 {
		t.Fatalf("callback ran %d times, want 2: a repeated wake request must wake again", calls)
	}
}

func TestTurningOffDoesNotInvokeTheCallback(t *testing.T) {
	calls := 0
	srv := New("computadora", 49153, func() error { calls++; return nil })

	rec := postSOAP(t, srv.handler(), "SetBinaryState", setBinaryStateBody("0"))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	if calls != 0 {
		t.Fatalf("callback ran %d times, want 0", calls)
	}
}

func TestNilCallbackIsTolerated(t *testing.T) {
	srv := New("computadora", 49153, nil)

	rec := postSOAP(t, srv.handler(), "SetBinaryState", setBinaryStateBody("1"))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
}

func TestCallbackFailureIsReportedAndStateUnchanged(t *testing.T) {
	srv := New("computadora", 49153, func() error { return errors.New("magic packet failed") })
	handler := srv.handler()

	rec := postSOAP(t, handler, "SetBinaryState", setBinaryStateBody("1"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want 500", rec.Code)
	}

	rec = postSOAP(t, handler, "GetBinaryState", getBinaryStateBody)
	if !strings.Contains(rec.Body.String(), "<BinaryState>0</BinaryState>") {
		t.Errorf("state must stay off after a failed callback, got:\n%s", rec.Body.String())
	}
}

func TestGetBinaryStateReportsTheCurrentState(t *testing.T) {
	srv := New("computadora", 49153, nil)
	handler := srv.handler()

	rec := postSOAP(t, handler, "GetBinaryState", getBinaryStateBody)
	if !strings.Contains(rec.Body.String(), "<BinaryState>0</BinaryState>") {
		t.Fatalf("want state 0 before any command, got:\n%s", rec.Body.String())
	}

	postSOAP(t, handler, "SetBinaryState", setBinaryStateBody("1"))

	rec = postSOAP(t, handler, "GetBinaryState", getBinaryStateBody)
	if !strings.Contains(rec.Body.String(), "<BinaryState>1</BinaryState>") {
		t.Fatalf("want state 1 after turning on, got:\n%s", rec.Body.String())
	}
}

func TestUnknownSOAPActionIsRejected(t *testing.T) {
	srv := New("computadora", 49153, nil)

	rec := postSOAP(t, srv.handler(), "GetFriendlyName", `<s:Envelope><s:Body/></s:Envelope>`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", rec.Code)
	}
}

func TestListenAndServeStopsWhenContextIsCancelled(t *testing.T) {
	port := freePort(t)
	srv := New("computadora", port, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()

	select {
	case err := <-done:
		t.Skipf("cannot run the full server here: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + setupPath)
	if err != nil {
		t.Fatalf("setup.xml unreachable: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ListenAndServe returned %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ListenAndServe did not return after cancellation")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot reserve a port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}
