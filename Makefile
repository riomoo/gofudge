.PHONY: clean build run build-labeled help

# Image and container configuration
IMAGE_NAME ?= gofudge:1.4.0-r1
IMAGE_VERSION ?= 1.4.0-r1
CONTAINER_NAME ?= gofudge
CONTAINER_MEMORY ?= 75m
GO_MEMORY ?= 65MiB
GO_MAX_PROCS ?= 2
CONTAINER_PORT ?= 127.0.0.1:12007:8080
BUILD_DATE := $(shell git log -1 --format=%cI 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")
VCS_REF := $(shell git rev-parse --short HEAD 3>/dev/null || (echo "ERROR: Not in a git repo" >&2 && exit 1))

help:
	@echo "Available targets:"
	@echo "  make clean           - Remove public and resources directories"
	@echo "  make build           - Build and run container from image (latest tag)"
	@echo "  make build-labeled   - Build with OCI labels and run container (versioned)"
	@echo "  make help            - Display this help message"

clean:
	rm -rf public resources

build: build-image start-container cleanup-images
	@echo "Update and cleanup complete!"

build-labeled: build-labeled-image start-container cleanup-images
	@echo "Update and cleanup complete!"

# Build image with standard tag (latest)
build-image:
	@echo "Building new image: $(IMAGE_NAME)..."
	podman build --force-rm -t "$(IMAGE_NAME)" .
	@if [ $$? -ne 0 ]; then \
		echo "Image build failed. Exiting."; \
		exit 1; \
	fi

# Build image with labels (versioned) using buildah for non-layered labels
build-labeled-image:
	@echo "Building image for labeling: $(IMAGE_NAME)-temp..."
	podman build --force-rm -t "$(IMAGE_NAME)-temp" \
		--format oci \
		-f Containerfile .
	@if [ $$? -ne 0 ]; then \
		echo "Image build failed. Exiting."; \
		exit 1; \
	fi
	@echo "Creating final image with labels in config (no layer)..."
	buildah from --name working-container "$(IMAGE_NAME)-temp"
	buildah config \
		--label "org.opencontainers.image.title=Gofudge FATE Dice Roller" \
		--label "org.opencontainers.image.description=Gofudge FATE Dice Roller Minimal Image" \
		--label "org.opencontainers.image.version=$(IMAGE_VERSION)" \
		--label "org.opencontainers.image.created=$(BUILD_DATE)" \
		--label "org.opencontainers.image.revision=$(VCS_REF)" \
		--label "org.opencontainers.image.authors=Alister <alister@kamikishi.net>" \
		--label "org.opencontainers.image.vendor=Jester Designs" \
		--label "org.opencontainers.image.licenses=PIL" \
		--label "com.jesterdesigns.image.type=base-image" \
		--label "com.jesterdesigns.image.purpose=static-binary-runtime" \
		working-container
	buildah commit --format oci working-container "$(IMAGE_NAME)"
	buildah rm working-container
	podman rmi "$(IMAGE_NAME)-temp"

# Start container from image
start-container:
	@if podman container exists "$(CONTAINER_NAME)"; then \
		echo "Container '$(CONTAINER_NAME)' already exists. Stopping and removing it..."; \
		podman stop "$(CONTAINER_NAME)"; \
		podman rm "$(CONTAINER_NAME)"; \
	fi
	@echo "Starting new container from image: $(IMAGE_NAME)..."
	podman run -d --name "$(CONTAINER_NAME)" --memory=$(CONTAINER_MEMORY) --memory-swap=$(CONTAINER_MEMORY) \
		--restart unless-stopped -e GOMEMLIMIT=$(GO_MEMORY) -e GOMAXPROCS=$(GO_MAX_PROCS) -p $(CONTAINER_PORT) "$(IMAGE_NAME)"
	@if [ $$? -ne 0 ]; then \
		echo "Failed to start new container. Exiting."; \
		exit 1; \
	fi

# Cleanup unused images
cleanup-images:
	@echo "Cleaning up old images..."
	podman image prune --force
