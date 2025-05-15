#!/bin/bash

# HyprBench Go Edition Installation Script
# This script installs HyprBench Go Edition on a Linux x86_64 system

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo -e "${YELLOW}Warning: Not running as root. Installation may fail if you don't have sufficient permissions.${NC}"
  echo "Consider running with sudo if you encounter permission issues."
  echo ""
fi

# Check architecture
ARCH=$(uname -m)
if [ "$ARCH" != "x86_64" ]; then
  echo -e "${RED}Error: This binary is built for x86_64 architecture only.${NC}"
  echo "Your architecture is: $ARCH"
  exit 1
fi

# Check OS
OS=$(uname -s)
if [ "$OS" != "Linux" ]; then
  echo -e "${RED}Error: This binary is built for Linux only.${NC}"
  echo "Your OS is: $OS"
  exit 1
fi

# Installation directory
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="hyprbench"
BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"

# Create installation directory if it doesn't exist
if [ ! -d "$INSTALL_DIR" ]; then
  echo "Creating installation directory: $INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
fi

# Check if binary exists in current directory
if [ ! -f "hyprbench-linux-amd64" ]; then
  echo -e "${RED}Error: hyprbench-linux-amd64 binary not found in current directory.${NC}"
  exit 1
fi

# Copy binary to installation directory
echo "Installing HyprBench to $BINARY_PATH..."
cp hyprbench-linux-amd64 "$BINARY_PATH"
chmod +x "$BINARY_PATH"

# Check if installation was successful
if [ -x "$BINARY_PATH" ]; then
  echo -e "${GREEN}HyprBench has been successfully installed!${NC}"
  echo "You can now run it by typing 'hyprbench' in your terminal."
  echo ""
  echo "Example usage:"
  echo "  hyprbench                  # Run all benchmarks"
  echo "  hyprbench --skip-disk      # Skip disk benchmarks"
  echo "  hyprbench --help           # Show help"
  echo ""
  echo "For more information, visit: https://github.com/hypr-technologies/hypr-bench"
else
  echo -e "${RED}Installation failed. Please check permissions and try again.${NC}"
  exit 1
fi

# Check for dependencies
echo "Checking for dependencies..."
DEPS=("sysbench" "fio" "stress-ng" "speedtest" "iperf3")
MISSING_DEPS=()

for dep in "${DEPS[@]}"; do
  if ! command -v "$dep" &> /dev/null; then
    MISSING_DEPS+=("$dep")
  fi
done

if [ ${#MISSING_DEPS[@]} -gt 0 ]; then
  echo -e "${YELLOW}Warning: Some dependencies are missing:${NC} ${MISSING_DEPS[*]}"
  echo "You can install them manually or run HyprBench with the --auto-install-deps flag."
  echo ""
  echo "For Debian/Ubuntu:"
  echo "  sudo apt update"
  echo "  sudo apt install sysbench fio stress-ng iperf3"
  echo "  # For speedtest, follow instructions at: https://www.speedtest.net/apps/cli"
  echo ""
  echo "For RHEL/CentOS/Fedora:"
  echo "  sudo dnf install sysbench fio stress-ng iperf3"
  echo "  # For speedtest, follow instructions at: https://www.speedtest.net/apps/cli"
else
  echo -e "${GREEN}All dependencies are installed!${NC}"
fi

echo ""
echo -e "${GREEN}Installation complete!${NC}"
