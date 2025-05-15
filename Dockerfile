# Stage 1: Build the Go application
FROM golang:1.22-alpine AS builder
# Alpine is small; ensure all build dependencies for CGO (if any, though we disable it) are there if needed.
# For a pure Go static build, Alpine is fine. If complex CGO, might need a Debian-based golang image.

WORKDIR /app

# Install git, needed for go mod download if you have private repos or specific versions
# Alpine uses 'apk'
RUN apk add --no-cache git

# Copy Go module files
COPY go.mod go.sum ./
# Optional: Copy go.work if used
# COPY go.work ./

# Download Go module dependencies
RUN go mod download
# Verify dependencies (optional but good practice)
RUN go mod verify

# Copy the rest of the application source code
COPY . .

# Build the HyprBench Go application statically
# CGO_ENABLED=0 is crucial for static linking without C libraries for Alpine.
# -ldflags="-w -s" makes the binary smaller by stripping debug info.
# The output binary will be in /app/hyprbench
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/hyprbench ./main.go

# Stage 2: Create the final small image with dependencies for running HyprBench
FROM ubuntu:22.04

# Avoid interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies (external tools HyprBench will call)
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    sysbench \
    fio \
    git \
    php-cli \
    php-xml \
    stress-ng \
    iperf3 \
    curl \
    jq \
    lsb-release \
    pciutils \
    util-linux \
    dmidecode \
    coreutils \
    bc \
    gnupg1 \
    apt-transport-https \
    dirmngr && \
    # --- Add Ookla's speedtest-cli ---
    echo "Attempting to download Ookla Speedtest CLI installation script..." && \
    curl -fsSL https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh -o /tmp/speedtest_install.sh && \
    if [ ! -s /tmp/speedtest_install.sh ]; then echo "Error: Downloaded speedtest_install.sh is empty or failed."; exit 1; fi && \
    echo "Downloaded speedtest_install.sh successfully." && \
    chmod +x /tmp/speedtest_install.sh && \
    echo "Executing speedtest_install.sh..." && \
    /tmp/speedtest_install.sh && \
    echo "Finished executing speedtest_install.sh." && \
    echo "Updating package list after adding Ookla repository..." && \
    apt-get update && \
    echo "Installing speedtest package..." && \
    apt-get install -y speedtest && \
    echo "Ookla Speedtest CLI installed successfully." && \
    # --- Cleanup ---
    echo "Cleaning up apt caches and temp files..." && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/speedtest_install.sh

# Copy the built static binary from the builder stage
COPY --from=builder /app/hyprbench /usr/local/bin/hyprbench

# Ensure it's executable
RUN chmod +x /usr/local/bin/hyprbench

# Set the entrypoint for the container to run HyprBench
# The user will need to run the container with --privileged for dmidecode/fio etc.
ENTRYPOINT ["/usr/local/bin/hyprbench"]

# Default command (can be overridden), e.g., show help
CMD ["--help"]