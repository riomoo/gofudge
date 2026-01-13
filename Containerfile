# Build stage
FROM golang:alpine AS builder

# Install build dependencies and UPX
RUN apk add --no-cache \
    musl-dev \
    gcc \
    wget \
    xz \
    git

RUN wget https://github.com/upx/upx/releases/download/v5.0.2/upx-5.0.2-amd64_linux.tar.xz && \
    tar -xf upx-5.0.2-amd64_linux.tar.xz && \
    mv upx-5.0.2-amd64_linux/upx /usr/local/bin/upx && \
    rm -r upx-5.0.2-amd64_linux upx-5.0.2-amd64_linux.tar.xz

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Create necessary directories, build, and compress with UPX
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags="-s -w -extldflags '-static' -X main.GOMEMLIMIT=50MiB -X runtime.defaultGOGC=50" -trimpath -gcflags="-l=4" -asmflags=-trimpath -o bin/main app/gofudge/main.go
RUN upx --best --ultra-brute bin/main
RUN chmod +x bin/main

FROM scratch
WORKDIR /app

# Copy only the built binary and necessary directories
COPY --from=builder /app/bin/main ./bin/main

EXPOSE 8080
ENTRYPOINT ["/app/bin/main"]
