#!/bin/bash

# Define variables for your image and container name.
IMAGE_NAME="localhost/gofudge:latest"
CONTAINER_NAME="gofudge"

# Step 1: Build the new container image.
echo "Building new image: $IMAGE_NAME..."
podman build --force-rm -t "$IMAGE_NAME" .

# Check if the build was successful before proceeding.
if [ $? -ne 0 ]; then
    echo "Image build failed. Exiting script."
    exit 1
fi

# Step 2: Check if the container is already running.
if podman container exists "$CONTAINER_NAME"; then
    echo "Container '$CONTAINER_NAME' already exists. Stopping and removing it..."
    # Step 3: Stop the existing container.
    podman stop "$CONTAINER_NAME"
    # Step 4: Remove the stopped container.
    podman rm "$CONTAINER_NAME"
fi

# Step 5: Run a new container from the newly built image.
echo "Starting new container from image: $IMAGE_NAME..."
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

echo "Update and cleanup complete!"
