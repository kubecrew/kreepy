#!/bin/bash

set -e

CLUSTER_NAME="kind"
IMAGE_NAME="ghcr.io/kubecrew/kreepy:snapshot"

# Check if kind cluster exists
if ! kind get clusters | grep -q "^$CLUSTER_NAME$"; then
    echo "Creating kind cluster..."
    echo ""
    kind create cluster --name "$CLUSTER_NAME"
else
    echo "Kind cluster '$CLUSTER_NAME' already exists."
    echo ""
fi

# Build the operator
echo "Building the operator..."
make docker-build IMG="$IMAGE_NAME"
echo ""

# Load the image into kind
echo "Loading the operator image into kind..."
kind load docker-image "$IMAGE_NAME" --name "$CLUSTER_NAME"
echo ""

# Deploy the operator using make
echo "Deploying the operator into the cluster..."
make deploy IMG="$IMAGE_NAME"
echo ""

# Install sample crd
echo "Installing sample CRD..."
kubectl apply -f hack/sample-crd.yaml
kubectl apply -f hack/sample-crd-multi-version.yaml
kubectl apply -f hack/sample.yaml
kubectl apply -f hack/multisample.yaml
echo ""

# Deploy the CRD Cleanup Policy
echo "Deploying the CRD Cleanup Policy..."
kubectl apply -f hack/crd-cleanup-policy.yaml
echo ""

# Wait for the operator pod to be ready
echo "Waiting for the operator pod to be ready..."
OPERATOR_POD=$(kubectl get pods -n kreepy-system -l control-plane=controller-manager -o jsonpath='{.items[0].metadata.name}')
kubectl wait --for=condition=Ready pod/"$OPERATOR_POD" -n kreepy-system --timeout=120s
echo ""

# Check for remaining CRDs
echo "Checking for remaining CRDs..."
echo "Expecting to see remaining CRDs in the status of crdcleanuppolicy-sample..."
REMAINING_CRDS=$(kubectl get crdcleanuppolicy crdcleanuppolicy-sample -n default -o jsonpath='{.status.remainingCrds}')

if [[ -n "$REMAINING_CRDS" && "$REMAINING_CRDS" != "[]" ]]; then
    echo "✅ Remaining CRDs detected: $REMAINING_CRDS. Test PASSED."
    echo ""
else
    echo "❌ No remaining CRDs found. Test FAILED."
    exit 1
fi

# Check for removed apiVersion in multisamples.example.com CRD
echo "Checking for removed apiVersion in multisamples.example.com CRD..."
echo "Expecting to see apiVersion v1 removed from the multisamples.example.com CRD..."
# Check if v1 still exists in the CRD
API_VERSION_EXISTS=$(kubectl get crd multisamples.example.com -o jsonpath='{.spec.versions[?(@.name=="v1")].name}')

if [[ -z "$API_VERSION_EXISTS" ]]; then
    echo "✅ apiVersion v1 is removed from multisamples.example.com CRD. Test PASSED."
    echo ""
else
    echo "❌ apiVersion v1 is still present in multisamples.example.com CRD. Test FAILED."
    exit 1
fi

# Checking for removed samples.example.com CRD after deleting all samples.example.com CRs
echo "Checking for removed samples.example.com CRD after deleting all samples.example.com CRs..."
echo "Expecting to see samples.example.com CRD removed after deleting all samples.example.com CRs..."
kubectl delete samples.example.com --all -n default
echo "Waiting one minute for the samples.example.com CRD to be removed to test the reconciliation after requeue..."
sleep 60
REMAINING_CRDS=$(kubectl get crdcleanuppolicy crdcleanuppolicy-sample -n default -o jsonpath='{.status.remainingCrds}')

if [[ -z "$REMAINING_CRDS" || "$REMAINING_CRDS" == "[]" ]]; then
    echo "✅ No remaining CRDs found. Test PASSED."
    echo ""
else
    echo "❌ Remaining CRDs detected: $REMAINING_CRDS. Test FAILED."
    exit 1
fi

echo "E2E test completed successfully."
kind delete cluster
