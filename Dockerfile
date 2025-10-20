# Build stage
FROM golang:bookworm AS builder

# Install UPX
RUN apt-get update && apt-get install -y wget xz-utils && rm -rf /var/lib/apt/lists/*

# Download the latest UPX binary directly from GitHub
RUN wget https://github.com/upx/upx/releases/download/v5.0.2/upx-5.0.2-amd64_linux.tar.xz
RUN tar -xf upx-5.0.2-amd64_linux.tar.xz && mv upx-5.0.2-amd64_linux/upx /usr/local/bin/upx && rm -r upx-5.0.2-amd64_linux upx-5.0.2-amd64_linux.tar.xz

# Create a simple Go web server
WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Create necessary directories, build, and compress with UPX
RUN mkdir -p /var/sockets
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags="-s -w -extldflags '-static' -X main.GOMEMLIMIT=50MiB -X runtime.defaultGOGC=150" -trimpath -gcflags="-l=4" -asmflags=-trimpath -o bin/main app/gofudge/main.go
RUN upx --best --ultra-brute bin/main
RUN chmod +x bin/main

# Final stage with Chainguard static
FROM cgr.dev/chainguard/static:latest
WORKDIR /app

# Copy only the built binary and necessary directories
COPY --from=builder /app/bin/main ./bin/main

EXPOSE 8080
USER nonroot:nonroot
CMD ["./bin/main"]
