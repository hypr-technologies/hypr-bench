#!/bin/bash

# Package HyprBench for Linux x86_64
echo "Packaging HyprBench for Linux x86_64..."

# Create a temporary directory
TEMP_DIR="hyprbench-pkg"
rm -rf "$TEMP_DIR"
mkdir -p "$TEMP_DIR"

# Copy files to the temporary directory, stripping all macOS attributes
# First, create clean copies without extended attributes
cat hyprbench-linux-amd64 > "$TEMP_DIR/hyprbench"
cat README-linux.md > "$TEMP_DIR/README.md"
cat install-linux.sh > "$TEMP_DIR/install.sh"

# Set proper permissions
chmod +x "$TEMP_DIR/hyprbench"
chmod +x "$TEMP_DIR/install.sh"

# Create the tarball with options to exclude macOS metadata
# COPYFILE_DISABLE=1 prevents creation of ._* files on macOS
COPYFILE_DISABLE=1 tar --no-mac-metadata -czf hyprbench-linux-amd64.tar.gz -C "$TEMP_DIR" .

# Clean up
rm -rf "$TEMP_DIR"

echo "Package created: hyprbench-linux-amd64.tar.gz"
echo "You can distribute this file to Linux x86_64 systems."
