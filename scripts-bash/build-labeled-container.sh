#!/bin/bash

IMAGE_NAME="localhost/gofudge:1.3.0"
CONTAINER_NAME="gofudge"
VERSION="1.3.0"
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
VCS_REF=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo "Building new image: $IMAGE_NAME..."
podman build --force-rm -t "${IMAGE_NAME}-temp" \
  --format oci \
  -f Containerfile .

if [ $? -ne 0 ]; then
    echo "Image build failed. Exiting script."
    exit 1
fi

if podman container exists "$CONTAINER_NAME"; then
    echo "Container '$CONTAINER_NAME' already exists. Stopping and removing it..."
    podman stop "$CONTAINER_NAME"
    podman rm "$CONTAINER_NAME"
fi

echo "Creating final image with labels in config (no layer)..."

# Use buildah to modify the image config directly
buildah from --name working-container "${IMAGE_NAME}-temp"

# Add labels directly to the config (doesn't create a layer!)
buildah config \
  --label "org.opencontainers.image.title=Gofudge" \
  --label "org.opencontainers.image.description=Gofudge Minimal Image" \
  --label "org.opencontainers.image.version=${VERSION}" \
  --label "org.opencontainers.image.created=${BUILD_DATE}" \
  --label "org.opencontainers.image.revision=${VCS_REF}" \
  --label "org.opencontainers.image.authors=Alister <alister@kamikishi.net>" \
  --label "org.opencontainers.image.vendor=Jester Designs" \
  --label "org.opencontainers.image.licenses=PIL" \
  --label "com.jesterdesigns.image.type=base-image" \
  --label "com.jesterdesigns.image.purpose=static-binary-runtime" \
  working-container

# Commit the container to the final image name
buildah commit --format oci working-container "$IMAGE_NAME"

# Cleanup
buildah rm working-container
podman rmi "${IMAGE_NAME}-temp"

echo "Starting new container from image: $IMAGE_NAME..."
# IMPROVED: Better memory settings and limits
podman run -d --name "$CONTAINER_NAME" --memory=75m --restart unless-stopped -p 12007:8080 "$IMAGE_NAME"

# Check if the new container started successfully.
if [ $? -ne 0 ]; then
    echo "Failed to start new container. Exiting script."
    exit 1
fi

# Step 6: Clean up old, unused images.
# This command removes any images that are not being used by a container.
echo "Cleaning up old images..."
podman image prune --force
podman tag ${CONTAINER_NAME}:${VERSION} git.jester-designs.com/riomoo/${CONTAINER_NAME}:${VERSION}
podman tag ${CONTAINER_NAME}:${VERSION} ${CONTAINER_NAME}:latest
podman tag ${CONTAINER_NAME}:latest git.jester-designs.com/riomoo/${CONTAINER_NAME}:latest
podman push git.jester-designs.com/riomoo/${CONTAINER_NAME}:${VERSION}
podman push git.jester-designs.com/riomoo/${CONTAINER_NAME}:latest
#podman tag rr:${VERSION} ghcr.io/riomoo/rr:${VERSION}
#podman tag rr:${VERSION} ggcr.io/riomoo/rr:${VERSION}
#podman tag rr:latest ghcr.io/riomoo/rr:latest
#podman tag rr:latest ggcr.io/riomoo/rr:latest

echo "Update and cleanup complete!"
