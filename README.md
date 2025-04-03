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
