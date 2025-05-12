#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status.
# Treat unset variables as an error when substituting.
# Pipelines return the exit status of the last command that failed, or zero if all succeed.
set -euo pipefail

# --- Configuration ---
INSTALL_DIR="/usr/local/bin"
REQUIRED_COMMANDS=(
    lscpu free dmidecode lsb_release lspci lsblk nproc # System Info
    sysbench fio jq git php stress-ng curl iperf3 bc  # Benchmarking & Core
    speedtest fast                                  # Network Speed (at least one)
)
# Corresponding package names for hints (order matters for the hint message)
DEBIAN_PACKAGES="util-linux procps dmidecode lsb-release pciutils coreutils sysbench fio jq git php-cli php-xml stress-ng curl iperf3 bc speedtest-cli nodejs npm"
FEDORA_PACKAGES="util-linux procps-ng dmidecode lsb-release pciutils coreutils sysbench fio jq git php-cli php-xml stress-ng curl iperf3 bc speedtest nodejs npm"

# --- Functions ---

# Function to print error messages in red
error_msg() {
    printf "\e[31mERROR: %s\e[0m\n" "$1" >&2
}

# Function to print info messages in green
info_msg() {
    printf "\e[32mINFO: %s\e[0m\n" "$1"
}

# Function to check if a command exists
check_command() {
    command -v "$1" >/dev/null 2>&1
}

# --- Script Logic ---

# 1. Root Check
if [ "$(id -u)" -ne 0 ]; then
    error_msg "This script must be run as root. Please use 'sudo ./install.sh'."
    exit 1
fi

# 2. Dependency Check
info_msg "Checking for required dependencies..."
missing_commands=()
speedtest_found=0
fast_found=0

for cmd in "${REQUIRED_COMMANDS[@]}"; do
    if ! check_command "$cmd"; then
        # Special handling for speedtest/fast
        if [[ "$cmd" == "speedtest" || "$cmd" == "fast" ]]; then
            continue # Don't add to missing_commands yet
        fi
        missing_commands+=("$cmd")
        # printf "Missing: %s\n" "$cmd" # Debugging
    else
        # Mark if found
        if [[ "$cmd" == "speedtest" ]]; then
            speedtest_found=1
        fi
        if [[ "$cmd" == "fast" ]]; then
            fast_found=1
        fi
        # printf "Found: %s\n" "$cmd" # Debugging
    fi
done

# Check if at least one speed test tool is available
if [[ "$speedtest_found" -eq 0 && "$fast_found" -eq 0 ]]; then
    missing_commands+=("speedtest OR fast") # Add a placeholder indicating the requirement
fi

# Report missing dependencies and exit if any are found
if [ ${#missing_commands[@]} -gt 0 ]; then
    error_msg "The following required commands are missing:"
    for missing in "${missing_commands[@]}"; do
        printf "  - %s\n" "$missing" >&2
    done
    printf "\nPlease install the missing packages.\n" >&2
    printf "Example for Debian/Ubuntu:\n  sudo apt update && sudo apt install -y %s\n" "$DEBIAN_PACKAGES" >&2
    printf "  (For fast-cli: sudo npm install --global fast-cli)\n" >&2
    printf "  (For official speedtest: See https://www.speedtest.net/apps/cli)\n" >&2
    printf "Example for Fedora/RHEL/CentOS:\n  sudo dnf install -y %s\n" "$FEDORA_PACKAGES" >&2
    printf "  (For fast-cli: sudo npm install --global fast-cli)\n" >&2
    printf "  (For official speedtest: curl -s https://install.speedtest.net/app/cli/install.rpm.sh | sudo bash; sudo dnf install speedtest)\n" >&2
    exit 1
fi

info_msg "All dependencies found."

# 3. Confirmation Prompt
echo "--------------------------------------------------"
echo "This script will install HyprBench and its components:"
echo "  - HyprBench.sh"
echo "  - hyprbench-netblast.sh"
echo "to the directory: $INSTALL_DIR"
echo "--------------------------------------------------"
read -p "Proceed with installation? (y/N): " confirm

# Convert confirmation to lowercase
confirm_lower=$(echo "$confirm" | tr '[:upper:]' '[:lower:]')

if [[ "$confirm_lower" != "y" ]]; then
    info_msg "Installation aborted by user."
    exit 0
fi

# 4. Installation Steps
info_msg "Starting installation..."

# Check if source scripts exist in the current directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
SOURCE_HYPRBENCH="$SCRIPT_DIR/HyprBench.sh"
SOURCE_NETBLAST="$SCRIPT_DIR/hyprbench-netblast.sh"

if [[ ! -f "$SOURCE_HYPRBENCH" || ! -f "$SOURCE_NETBLAST" ]]; then
    error_msg "Could not find 'HyprBench.sh' or 'hyprbench-netblast.sh' in the current directory ($SCRIPT_DIR)."
    error_msg "Please run this script from the same directory as the benchmark scripts."
    exit 1
fi

# Create target directory if it doesn't exist (though /usr/local/bin usually does)
mkdir -p "$INSTALL_DIR"

# Copy files
info_msg "Copying scripts to $INSTALL_DIR..."
if cp "$SOURCE_HYPRBENCH" "$INSTALL_DIR/"; then
    info_msg "Copied HyprBench.sh"
else
    error_msg "Failed to copy HyprBench.sh to $INSTALL_DIR"
    exit 1
fi

if cp "$SOURCE_NETBLAST" "$INSTALL_DIR/"; then
    info_msg "Copied hyprbench-netblast.sh"
else
    error_msg "Failed to copy hyprbench-netblast.sh to $INSTALL_DIR"
    # Attempt cleanup of the first file if the second failed
    rm -f "$INSTALL_DIR/HyprBench.sh"
    exit 1
fi

# Set execute permissions
info_msg "Setting execute permissions..."
if chmod +x "$INSTALL_DIR/HyprBench.sh" && chmod +x "$INSTALL_DIR/hyprbench-netblast.sh"; then
    info_msg "Permissions set successfully."
else
    error_msg "Failed to set execute permissions on scripts in $INSTALL_DIR"
    # Attempt cleanup
    rm -f "$INSTALL_DIR/HyprBench.sh" "$INSTALL_DIR/hyprbench-netblast.sh"
    exit 1
fi

# 5. Success Message
echo "--------------------------------------------------"
info_msg "HyprBench and hyprbench-netblast successfully installed to $INSTALL_DIR."
info_msg "You should now be able to run 'sudo HyprBench.sh' and 'sudo hyprbench-netblast.sh' from anywhere,"
info_msg "assuming $INSTALL_DIR is in your system's PATH."
echo "--------------------------------------------------"

exit 0