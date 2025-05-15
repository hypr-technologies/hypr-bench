#!/bin/bash

# HyprBench Linux Installation Script
# This script installs HyprBench on a Linux system

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}HyprBench Linux Installer${NC}"
echo "This script will install HyprBench to /usr/local/bin"
echo ""

# Check if running as root for system-wide installation
if [ "$EUID" -ne 0 ]; then
  echo -e "${YELLOW}Not running as root. Installing to local directory instead.${NC}"
  INSTALL_DIR="$HOME/bin"
  
  # Create local bin directory if it doesn't exist
  if [ ! -d "$INSTALL_DIR" ]; then
    echo "Creating directory: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"
  fi
  
  # Add to PATH if not already there
  if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo "Adding $INSTALL_DIR to your PATH"
    echo 'export PATH="$HOME/bin:$PATH"' >> "$HOME/.bashrc"
    echo "Please run 'source ~/.bashrc' after installation or start a new terminal"
  fi
else
  INSTALL_DIR="/usr/local/bin"
fi

# Check if hyprbench binary exists
if [ ! -f "./hyprbench" ]; then
  echo -e "${RED}Error: hyprbench binary not found in current directory${NC}"
  echo "Please extract the tarball first with: tar -xzf hyprbench-linux-amd64.tar.gz"
  exit 1
fi

# Copy binary to installation directory
echo "Installing HyprBench to $INSTALL_DIR/hyprbench"
cp ./hyprbench "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/hyprbench"

# Verify installation
if [ -x "$INSTALL_DIR/hyprbench" ]; then
  echo -e "${GREEN}HyprBench has been successfully installed!${NC}"
  echo ""
  echo "You can now run it by typing:"
  echo "  hyprbench"
  
  # Check for dependencies
  echo ""
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
else
  echo -e "${RED}Installation failed. Please check permissions and try again.${NC}"
  exit 1
fi

echo ""
echo -e "${GREEN}Installation complete!${NC}"
