# Copyright 2025 Marc Siegenthaler
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build stage
FROM golang:1.24.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the csi-driver binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o csi-driver ./cmd/csi-driver

# Runtime stage
FROM alpine:3.22.0

LABEL org.opencontainers.image.source=https://github.com/m4rcsi/csi-xen-orchestra-driver
LABEL org.opencontainers.image.description="CSI driver for Xen Orchestra"
LABEL org.opencontainers.image.licenses=Apache-2.0

# Install ca-certificates for HTTPS requests, util-linux for mount command, and e2fsprogs for ext4 formatting
RUN apk --no-cache add ca-certificates tzdata util-linux e2fsprogs

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/csi-driver .

# Change ownership to non-root user
RUN chown appuser:appgroup /app/csi-driver

# Switch to non-root user
USER appuser

# Set the binary as the entrypoint
ENTRYPOINT ["./csi-driver"]

# Default command (can be overridden)
CMD ["--help"] 





