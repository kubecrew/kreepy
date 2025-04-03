/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CRDCleanupVersion struct {
	// Name is the name of the CustomResourceDefinition that the operator should delete.
	Name string `json:"name"`

	// Version is the apiVersion of the CustomResourceDefinition that the operator should delete.
	Version string `json:"version,omitempty"`
}

// CRDCleanupPolicySpec defines the desired state of CRDCleanupPolicy.
type CRDCleanupPolicySpec struct {
	// CRDsVersions is a list of names and apiVersions of CustomResourceDefinitions that the operator should delete.
	// Only the name of the CRD is required.
	CRDsVersions []CRDCleanupVersion `json:"crdsversions,omitempty"`
}

// CRDCleanupPolicyStatus defines the observed state of CRDCleanupPolicy.
type CRDCleanupPolicyStatus struct {
	// StatusMessage provides information about the current state of the cleanup process.
	StatusMessage string `json:"statusMessage,omitempty"`

	// ProcessedCRDs is a list of names of CRDs that have already been processed by the operator.
	ProcessedCRDs []string `json:"processedCrds"`

	// RemainingCRDs is a list of names of CRDs that are yet to be processed.
	RemainingCRDs []string `json:"remainingCrds"`

	// NonExistentCRDs is a list of names of CRDs that were not existing while processing.
	NonExistentCRDs []string `json:"nonExistentCrds"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CRDCleanupPolicy is the Schema for the crdcleanuppolicies API.
type CRDCleanupPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CRDCleanupPolicySpec   `json:"spec,omitempty"`
	Status CRDCleanupPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CRDCleanupPolicyList contains a list of CRDCleanupPolicy.
type CRDCleanupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CRDCleanupPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CRDCleanupPolicy{}, &CRDCleanupPolicyList{})
}
