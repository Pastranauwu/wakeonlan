# wakeup

Local Wake-on-LAN device emulator for Amazon Alexa.

Emulates a Belkin WeMo socket so that Alexa discovers and controls it without internet, AWS account, or open ports. When you tell Alexa to turn it on, it sends a Wake-on-LAN (WOL) magic packet to wake a device on your network.

## Prerequisites

Before using wakeup, ensure:

1. **Target machine has Wake-on-LAN enabled in BIOS** — check your BIOS settings, typically called "Wake on LAN" or "Power on from Network"
2. **Network card supports WOL** — modern Ethernet cards do. Check with:
   ```bash
   ethtool eth0 | grep Wake
   ```
   Should show `Wake-on: g` (magic packet enabled). If not, enable it:
   ```bash
   sudo ethtool -s eth0 wol g
   ```
   Make persistent by adding to `/etc/network/interfaces` or netplan config.

3. **wakeup runs continuously** — it must keep running on a machine that stays powered on (Raspberry Pi, always-on NAS, laptop with lid open, etc.)

## Usage

Build and run with the target device's MAC address and name:

```bash
go build -o wakeup
./wakeup -mac AA:BB:CC:DD:EE:FF -name "Desktop PC"
```

### Flags

- `-mac` (required): MAC address of the device to wake (format: `AA:BB:CC:DD:EE:FF`)
- `-name`: Name to display in Alexa (default: "Device")
- `-port`: HTTP port for WeMo server (default: 49154; use 80 or 49153-49160 for Alexa discovery)
- `-addr`: Broadcast address for WOL packets (default: "255.255.255.255:9")

## Alexa Setup

1. Start wakeup on your LAN machine
2. Open Alexa app → Devices → + (Add) → Light → Discover Devices
3. Alexa scans the network for 30-60 seconds
4. New device appears with the name you specified (e.g., "Desktop PC")
5. Say "Alexa, turn on Desktop PC" to send the WOL packet

Each time Alexa sends "turn on", wakeup sends a fresh WOL packet, so you can wake a sleeping machine multiple times without side effects.

## Systemd Service

To run wakeup as a service at boot, install `deploy/wakeup.service`:

```bash
sudo cp deploy/wakeup.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable wakeup
sudo systemctl start wakeup
```

Edit the service file to set your MAC address and device name before starting.

## How It Works

- wakeup listens for SSDP discovery (used by Alexa to find devices on the network)
- Responds with a fake WeMo socket descriptor so Alexa recognizes it
- When Alexa sends a SOAP control request to turn on, wakeup crafts and sends a magic packet to the target MAC
- The magic packet (102 bytes, broadcast on UDP 9) triggers hardware WOL if enabled on the target
