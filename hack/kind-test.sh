#!/bin/bash

set -e

CLUSTER_NAME="kind"
IMAGE_NAME="ghcr.io/jaydee94/kreepy:snapshot"

# Check if kind cluster exists
if ! kind get clusters | grep -q "^$CLUSTER_NAME$"; then
    echo "Creating kind cluster..."
    kind create cluster --name "$CLUSTER_NAME"
else
    echo "Kind cluster '$CLUSTER_NAME' already exists."
fi

# Build the operator
echo "Building the operator..."
make docker-build IMG="$IMAGE_NAME"

# Load the image into kind
echo "Loading the operator image into kind..."
kind load docker-image "$IMAGE_NAME" --name "$CLUSTER_NAME"

# Deploy the operator using make
echo "Deploying the operator into the cluster..."
make deploy IMG="$IMAGE_NAME"

# Install sample crd
echo "Installing sample CRD..."
kubectl apply -f hack/sample-crd.yaml

echo "Setup complete!"
