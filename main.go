package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"wakeup/internal/wemo"
	"wakeup/internal/wol"
)

func main() {
	mac := flag.String("mac", "", "MAC address of the device to wake (e.g. AA:BB:CC:DD:EE:FF)")
	name := flag.String("name", "Device", "Name of the WeMo device")
	port := flag.Int("port", 49154, "HTTP port for WeMo server")
	addr := flag.String("addr", "255.255.255.255:9", "Broadcast address for WOL packets")
	flag.Parse()

	// Validate MAC at startup
	if _, err := wol.ParseMAC(*mac); err != nil {
		log.Fatalf("invalid MAC address: %v", err)
	}

	// Create WeMo server with callback that sends WOL
	server := wemo.New(*name, *port, func() error {
		return wol.Send(*mac, *addr)
	})

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Start server
	if err := server.ListenAndServe(ctx); err != nil {
		log.Fatal(err)
	}
}
