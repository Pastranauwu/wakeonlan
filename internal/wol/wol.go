package wol

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

func ParseMAC(mac string) ([6]byte, error) {
	var result [6]byte

	mac = strings.ToUpper(mac)
	mac = strings.ReplaceAll(mac, "-", ":")

	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		return result, fmt.Errorf("invalid MAC address format: expected 6 octets, got %d", len(parts))
	}

	for i, part := range parts {
		if len(part) != 2 {
			return result, fmt.Errorf("invalid MAC address: octet %d has invalid length", i)
		}
		b, err := hex.DecodeString(part)
		if err != nil {
			return result, fmt.Errorf("invalid MAC address: octet %d is not hex: %w", i, err)
		}
		result[i] = b[0]
	}

	return result, nil
}

func Send(mac, addr string) error {
	macBytes, err := ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("failed to parse MAC: %w", err)
	}

	packet := magicPacket(macBytes)

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return fmt.Errorf("failed to dial UDP: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}

	return nil
}

func magicPacket(mac [6]byte) []byte {
	packet := make([]byte, 102)

	// First 6 bytes: broadcast (0xFF)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}

	// Repeat MAC 16 times
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac[:])
	}

	return packet
}
