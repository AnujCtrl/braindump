#!/bin/bash
# Setup udev rule for Analog Devices H58 thermal printer (USB 0456:0808)
# This allows the braindump CLI to write to /dev/usb/lp0 without sudo.

set -e

RULE='SUBSYSTEM=="usb", ATTR{idVendor}=="0456", ATTR{idProduct}=="0808", MODE="0666"'
RULE_FILE="/etc/udev/rules.d/99-braindump-printer.rules"

echo "Creating udev rule at $RULE_FILE"
echo "$RULE" | sudo tee "$RULE_FILE"
sudo udevadm control --reload-rules
sudo udevadm trigger

echo ""
echo "Done. Replug the printer or reboot for changes to take effect."
echo "Test with: echo 'test' > /dev/usb/lp0"
