# kreepy

<img src="docs/logo/kreepy-logo.png" width="250" alt="kreepy-logo">

`kreepy` is a Kubernetes operator that removes deprecated CRDs that can be specified by a policy.

## Description

`kreepy` is designed to help Kubernetes administrators manage deprecated Custom Resource Definitions (CRDs) in their clusters. By defining policies, `kreepy` identifies and removes deprecated CRDs, ensuring your cluster remains clean and compliant with the latest Kubernetes standards. This operator is particularly useful for maintaining large clusters where deprecated resources can accumulate over time.

## Features

- **Policy-Driven CRD Management**: Define policies to specify which deprecated CRDs should be removed.
- **Automated Cleanup**: Automatically detect and delete deprecated CRDs based on the defined policies.

## Getting Started

### Prerequisites

- go version v1.23.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

## Installation with Operator Lifecycle Manager (OLM)

The Operator Lifecycle Manager (OLM) simplifies the installation and management of Kubernetes operators. Follow these steps to install `kreepy` using OLM:

### Prerequisites

- OLM installed on your Kubernetes cluster. You can install OLM by following the [official OLM installation guide](https://olm.operatorframework.io/docs/getting-started/).

### Steps to Install

1. **Add the Operator Catalog Source** 
   Add the `kreepy` operator catalog source to your cluster. Create a file named `kreepy-catalog-source.yaml` with the following content:

   ```yaml
   apiVersion: operators.coreos.com/v1alpha1
   kind: CatalogSource
   metadata:
     name: kreepy-catalog
     namespace: olm
   spec:
     sourceType: grpc
     image: ghcr.io/kubecrew/kreepy-catalog:stable
   ```

2. **Create a Subscription**
   Create a file named `kreepy-subscription.yaml` with the following content:

   ```yaml
   apiVersion: operators.coreos.com/v1alpha1
   kind: Subscription
   metadata:
     name: kreepy
     namespace: olm
   spec:
     channel: stable
     name: kreepy
     source: kreepy-catalog
     sourceNamespace: olm
   ```

3. **Apply the Subscription**
   Once the `kreepy-subscription.yaml` file is created, apply it to your cluster using the following command:

   ```sh
   kubectl apply -f kreepy-subscription.yaml
   ```

4. **Verify the Subscription**
   Check if the subscription has been created successfully by running:

   ```sh
   kubectl get subscriptions -n olm
   ```

   Look for the `kreepy` subscription in the output.

5. **Monitor the Installation**
   Verify that the operator is installed and running by checking the `ClusterServiceVersion` (CSV) status:

   ```sh
   kubectl get csv -n olm
   ```

   Ensure the CSV for `kreepy` is in the `Succeeded` phase.

## Using the kreepy Operator

Once the `kreepy` operator is installed and running, you can start using it to manage deprecated CRDs in your Kubernetes cluster. The operator works by processing `CRDCleanupPolicy` resources, which define the CRDs and their versions to be removed.

### How CRD Cleanup Works

The `kreepy` operator watches for `CRDCleanupPolicy` resources and performs cleanup based on the specified policies. Here's an example:

1. **Define a Cleanup Policy**

   Create a cleanup policy in a YAML file, for example, `crd-cleanup-policy.yaml`:

   ```yaml
   apiVersion: policies.kreepy.kubecrew.de/v1alpha1
   kind: CRDCleanupPolicy
   metadata:
     name: crdcleanuppolicy-sample
   spec:
     crdsversions:
       - name: samples.example.com
       - name: multisamples.example.com
         version: v1
   ```

   - **`crdsversions`**: This field lists the CRDs to be cleaned up. Each entry specifies:
     - `name`: The name of the CRD to be removed.
     - `version` (optional): The specific version of the CRD to be removed. If omitted, all versions of the CRD will be targeted.

2. **Apply the Cleanup Policy**

   Apply the policy to your cluster using the following command:

   ```sh
   kubectl apply -f crd-cleanup-policy.yaml
   ```

   This creates the `CRDCleanupPolicy` resource in the cluster.

3. **Operator Processes the Policy**

   The `kreepy` operator reads the `CRDCleanupPolicy` and identifies the specified CRDs and versions in the cluster. It then removes the targeted CRDs.

4. **Monitor the Cleanup**

   You can monitor the operator's logs to ensure the cleanup is proceeding as expected:

   ```sh
   kubectl logs -n olm deployment/kreepy-operator
   ```

   The logs will show details about the CRDs being removed.

5. **Verify the Cleanup**

   After the operator processes the policy, verify that the specified CRDs have been removed:

   ```sh
   kubectl get crds
   ```

   The CRDs listed in the `CRDCleanupPolicy` should no longer appear in the output.

## Contributing

We welcome contributions to `kreepy`! Here's how you can get involved:

1. Fork the repository and create a new branch for your feature or bugfix.
2. Write clear and concise code, following the project's coding standards.
3. Add tests for your changes to ensure they work as expected.
4. Submit a pull request with a detailed description of your changes.

## Testing

### End-to-End Testing with `hack/kind-test.sh`

The `hack/kind-test.sh` script provides an end-to-end test for the `kreepy` operator using a local [Kind](https://kind.sigs.k8s.io/) cluster. This script sets up a Kubernetes cluster, deploys the operator, and runs tests to verify its functionality.

#### Prerequisites

- [Kind](https://kind.sigs.k8s.io/) installed on your system.
- Docker installed and running.

#### Running the Test

To execute the end-to-end test, run the following command:

```sh
./hack/kind-test.sh
```
