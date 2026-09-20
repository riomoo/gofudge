# Build stage
FROM --platform=linux/amd64 docker.io/library/alpine@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS downloader

# Install build dependencies and UPX
RUN apk add --no-cache \
	tar=1.35-r5 \
	gcc=15.2.0-r5 \
	git=2.54.0-r0 \
	musl-dev=1.2.6-r2 \
	wget=1.25.0-r3 \
	xz=5.8.4-r0

ARG GO_VERSION=1.27.0
ARG GO_SHA256=675c26c449cbb18fc24b74650de1eabbae6e16f64326fd85a283fb3b58280685
ARG UPX_VERSION=v5.2.0
ARG UPX_SHA256=3db5d3294707439db97866feab8d75d800f028f48481a40547411824da4288a1
RUN wget https://github.com/upx/upx/releases/download/${UPX_VERSION}/upx-${UPX_VERSION#v}-amd64_linux.tar.xz \
    && echo "${UPX_SHA256}  upx-${UPX_VERSION#v}-amd64_linux.tar.xz" | sha256sum -c - \
    && tar -xf upx-${UPX_VERSION#v}-amd64_linux.tar.xz \
    && mv upx-${UPX_VERSION#v}-amd64_linux/upx /upx
RUN wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz && \
    echo "${GO_SHA256}  go${GO_VERSION}.linux-amd64.tar.gz" | sha256sum -c - && \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz && \
    rm go${GO_VERSION}.linux-amd64.tar.gz

# Stage 2: Build Go server
FROM --platform=linux/amd64 docker.io/library/alpine@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS builder

COPY --from=downloader /upx /usr/local/bin/upx
COPY --from=downloader /usr/local/go /usr/local

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod ./
RUN go mod download
COPY . .

# Create necessary directories, build, and compress with UPX
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
	-ldflags="-s -w -extldflags '-static'" \
	-trimpath \
	-o /gofudge app/gofudge/main.go
RUN upx --best --lzma /gofudge
RUN chmod +x /gofudge

FROM git.jester-designs.com/riomoo/alisterbase@sha256:903b779f25bebc7d38d83b58b42cb349a214f789f76aa24e7c511aa9f0e180f0

# Copy only the built binary and necessary directories
COPY --from=builder /gofudge /gofudge

EXPOSE 8080
ENTRYPOINT ["/gofudge"]
