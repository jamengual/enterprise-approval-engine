FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /issueops-approvals ./cmd/action

# Final stage - minimal image
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates git

# Copy binary from builder
COPY --from=builder /issueops-approvals /issueops-approvals

# Note: Running as root is required for GitHub Actions to write outputs
# The mounted file_commands directory requires root access
ENTRYPOINT ["/issueops-approvals"]
