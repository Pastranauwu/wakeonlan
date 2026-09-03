package wol

import (
	"testing"
)

func TestParseMAC(t *testing.T) {
	tests := []struct {
		input   string
		want    [6]byte
		wantErr bool
	}{
		{
			input: "AA:BB:CC:DD:EE:FF",
			want:  [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
		},
		{
			input: "aa:bb:cc:dd:ee:ff",
			want:  [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
		},
		{
			input: "AA-BB-CC-DD-EE-FF",
			want:  [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
		},
		{
			input: "aa-bb-cc-dd-ee-ff",
			want:  [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
		},
		{
			input:   "AA:BB:CC:DD:EE",
			wantErr: true,
		},
		{
			input:   "AA:BB:CC:DD:EE:FF:FF",
			wantErr: true,
		},
		{
			input:   "GG:BB:CC:DD:EE:FF",
			wantErr: true,
		},
		{
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseMAC(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMAC(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseMAC(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSend(t *testing.T) {
	// Test with valid inputs
	mac := "AA:BB:CC:DD:EE:FF"
	addr := "255.255.255.255:9"

	err := Send(mac, addr)
	if err != nil {
		t.Errorf("Send(%q, %q) error = %v", mac, addr, err)
	}
}

func TestMagicPacket(t *testing.T) {
	macBytes := [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	packet := magicPacket(macBytes)

	if len(packet) != 102 {
		t.Errorf("magicPacket length = %d, want 102", len(packet))
	}

	// Check first 6 bytes are 0xFF
	for i := 0; i < 6; i++ {
		if packet[i] != 0xFF {
			t.Errorf("magicPacket[%d] = 0x%02X, want 0xFF", i, packet[i])
		}
	}

	// Check MAC is repeated 16 times
	for rep := 0; rep < 16; rep++ {
		offset := 6 + rep*6
		for i := 0; i < 6; i++ {
			if packet[offset+i] != macBytes[i] {
				t.Errorf("magicPacket[%d] = 0x%02X, want 0x%02X", offset+i, packet[offset+i], macBytes[i])
			}
		}
	}
}
