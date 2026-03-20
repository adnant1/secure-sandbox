#!/usr/bin/env bash

set -euo pipefail

# Usage:
#   ./scripts/create-bundle.sh ./test-bundle

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <bundle-path>"
  exit 1
fi

BUNDLE_PATH="$1"
ROOTFS_PATH="$BUNDLE_PATH/rootfs"

echo "[+] Creating bundle at: $BUNDLE_PATH"

# Create base directory structure
mkdir -p "$ROOTFS_PATH/bin"
mkdir -p "$ROOTFS_PATH/proc"
mkdir -p "$ROOTFS_PATH/tmp"
mkdir -p "$ROOTFS_PATH/dev"

# Locate busybox
BUSYBOX_PATH="$(command -v busybox || true)"
if [ -z "$BUSYBOX_PATH" ]; then
  echo "[!] busybox not found"
  echo "    Install it with: sudo apt install busybox"
  exit 1
fi

echo "[+] Using busybox at: $BUSYBOX_PATH"

# Copy busybox into rootfs
cp "$BUSYBOX_PATH" "$ROOTFS_PATH/bin/busybox"
chmod +x "$ROOTFS_PATH/bin/busybox"

echo "[+] Creating BusyBox applet symlinks (relative)..."

# Generate applets using busybox itself
APPLET_LIST=$("$ROOTFS_PATH/bin/busybox" --list)

for app in $APPLET_LIST; do
  if [ "$app" != "busybox" ]; then
    ln -sf busybox "$ROOTFS_PATH/bin/$app"
  fi
done

echo "[+] Bundle created successfully!"
echo ""

# Helpful structure output
echo "Structure:"
echo "  $BUNDLE_PATH/"
echo "    └── rootfs/"
echo "        ├── bin/"
echo "        │   ├── busybox"
echo "        │   ├── ls -> busybox"
echo "        │   ├── sh -> busybox"
echo "        │   └── ... (all BusyBox applets)"
echo "        ├── proc/"
echo "        ├── tmp/"
echo "        └── dev/"
echo ""