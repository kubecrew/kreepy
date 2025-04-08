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

package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kubecrew/kreepy/api/v1alpha1"
)

// CRDCleanupPolicyReconciler reconciles a CRDCleanupPolicy object
type CRDCleanupPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=policies.kreepy.kubecrew.de,resources=crdcleanuppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policies.kreepy.kubecrew.de,resources=crdcleanuppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=policies.kreepy.kubecrew.de,resources=crdcleanuppolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions/finalizers,verbs=update
// +kubebuilder:rbac:groups="*",resources="*",verbs="list"

func (r *CRDCleanupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Starting reconciliation for CRDCleanupPolicy", "name", req.NamespacedName)

	// Fetch the CRDCleanupPolicy object
	policy, err := r.fetchPolicy(ctx, req, log)
	if err != nil {
		return ctrl.Result{}, err
	}
	if policy == nil {
		return ctrl.Result{}, nil
	}
	// Initialize status fields if needed
	r.initializeStatusFields(policy)

	// Process CRDs
	updatedRemainingCRDs, err := r.processCRDs(ctx, policy, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update the status
	err = r.updatePolicyStatus(ctx, policy, updatedRemainingCRDs, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Requeue if there are still CRDs to process
	if len(updatedRemainingCRDs) > 0 {
		log.Info("Requeuing reconciliation as there are still CRDs to process")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	log.Info("Reconciliation complete for CRDCleanupPolicy", "name", req.NamespacedName)
	return ctrl.Result{}, nil
}

// fetchPolicy fetches the CRDCleanupPolicy object from the cluster
func (r *CRDCleanupPolicyReconciler) fetchPolicy(ctx context.Context, req ctrl.Request, log logr.Logger) (*policiesv1alpha1.CRDCleanupPolicy, error) {
	policy := &policiesv1alpha1.CRDCleanupPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if errors.IsNotFound(err) {
			log.Info("No CRDCleanupPolicy found. May be deleted", "name", policy.Name)
			return nil, nil
		}
		log.Error(err, "Failed to fetch CRDCleanupPolicy")

		return nil, err
	}
	log.Info("Fetched CRDCleanupPolicy", "name", policy.Name)
	return policy, nil
}

// initializeStatusFields initializes the status fields of the CRDCleanupPolicy if they are nil
func (r *CRDCleanupPolicyReconciler) initializeStatusFields(policy *policiesv1alpha1.CRDCleanupPolicy) {
	if policy.Status.ProcessedCRDs == nil {
		policy.Status.ProcessedCRDs = []string{}
	}
	if policy.Status.NonExistentCRDs == nil {
		policy.Status.NonExistentCRDs = []string{}
	}
	if policy.Status.RemainingCRDs == nil {
		for _, crdVersion := range policy.Spec.CRDsVersions {
			if crdVersion.Version == "" {
				policy.Status.RemainingCRDs = append(policy.Status.RemainingCRDs, crdVersion.Name)
				continue
			}
			policy.Status.RemainingCRDs = append(policy.Status.RemainingCRDs, fmt.Sprintf("%s/%s", crdVersion.Name, crdVersion.Version))
		}
	}
}

// processCRDs processes the CRDs listed in the policy and deletes them
func (r *CRDCleanupPolicyReconciler) processCRDs(ctx context.Context, policy *policiesv1alpha1.CRDCleanupPolicy, log logr.Logger) ([]string, error) {
	updatedRemainingCRDs := make([]string, 0)

	for _, originalCRDName := range policy.Status.RemainingCRDs {
		log.Info("Processing CRD", "Name", originalCRDName)
		crdVersion := ""
		crdName := originalCRDName
		if strings.Contains(originalCRDName, "/") {
			// In case of specific api versions should be processed
			parts := strings.SplitN(originalCRDName, "/", 2)
			crdName = parts[0]
			crdVersion = parts[1]
		}

		// Fetch the CRD definition to get its group, version, and kind
		crd, err := r.fetchCRDDefinition(ctx, crdName, log)
		if err != nil {
			updatedRemainingCRDs = append(updatedRemainingCRDs, originalCRDName)
			continue
		}

		// Continue since the CRD must be deleted
		if crd == nil {
			policy.Status.NonExistentCRDs = append(policy.Status.NonExistentCRDs, originalCRDName)
			continue
		}

		// Check if there are any instances of this CRD in the cluster
		instanceCount, err := r.checkCRDInstances(ctx, crd, log, crdVersion, originalCRDName, policy)
		if err != nil {
			updatedRemainingCRDs = append(updatedRemainingCRDs, originalCRDName)
			continue
		}

		// If there are instances, skip deletion
		if instanceCount > 0 {
			log.Info("Instances of CRD found, skipping deletion", "CRD", originalCRDName)
			updatedRemainingCRDs = append(updatedRemainingCRDs, originalCRDName)
			continue
		}

		if instanceCount < 0 {
			policy.Status.NonExistentCRDs = append(policy.Status.NonExistentCRDs, originalCRDName)
			continue
		}
		// Attempt to delete the CRD
		if err := r.deleteCRDorVersion(ctx, crd, log, crdVersion); err != nil {
			updatedRemainingCRDs = append(updatedRemainingCRDs, originalCRDName)
			continue
		}

		// Add to processed CRDs
		log.Info("Successfully deleted CRD", "CRD", originalCRDName)
		policy.Status.ProcessedCRDs = append(policy.Status.ProcessedCRDs, originalCRDName)
	}

	return updatedRemainingCRDs, nil
}

func (r *CRDCleanupPolicyReconciler) fetchCRDDefinition(ctx context.Context, crdName string, log logr.Logger) (*v1.CustomResourceDefinition, error) {
	crd := &v1.CustomResourceDefinition{}
	crd.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	crd.SetName(crdName)
	log.Info("Fetching CRD definition for", "Name", crdName)

	if err := r.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil
		}
		log.Error(err, "Failed to fetch CRD definition", "Name", crdName)
		return nil, err
	}

	return crd, nil
}

func (r *CRDCleanupPolicyReconciler) checkCRDInstances(ctx context.Context, crd *v1.CustomResourceDefinition, log logr.Logger, crdVersion string, originalCRDName string, policy *policiesv1alpha1.CRDCleanupPolicy) (int, error) {
	group := crd.Spec.Group
	kind := crd.Spec.Names.Singular
	version := crdVersion
	if !slices.ContainsFunc(crd.Spec.Versions, func(v v1.CustomResourceDefinitionVersion) bool {
		return crdVersion == "" || v.Name == crdVersion
	}) {
		log.Info("No version found in CRD", "CRD", crd.GetName(), "Version", crdVersion)
		return -1, nil
	}
	if crdVersion == "" {
		version = crd.Spec.Versions[0].Name
	}
	log.Info("Checking for CRD instances", "CRD", crd.GetName(), "Version", version)

	crdGVR := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: kind,
	}

	var instances unstructured.UnstructuredList
	instances.SetGroupVersionKind(crdGVR.GroupVersion().WithKind(kind))
	if err := r.List(ctx, &instances); client.IgnoreNotFound(err) != nil {
		log.Error(err, "Failed to list instances of CRD", "CRD", crd.GetName())
		return -1, err
	}
	return len(slices.DeleteFunc(instances.Items, func(item unstructured.Unstructured) bool {
		return item.GetAPIVersion() != crdVersion && crdVersion != ""
	})), nil
}

// deleteCRDorVersion deletes the given CRD or a specific apiVersion of the CRD from the cluster
func (r *CRDCleanupPolicyReconciler) deleteCRDorVersion(ctx context.Context, crd *v1.CustomResourceDefinition, log logr.Logger, crdVersion string) error {
	if crdVersion == "" {
		// Delete the entire CRD
		log.Info("Delete the entire CRD since no specific apiVerson was specified", "CRD", crd.GetName())
		return r.deleteCRD(ctx, crd, log)
	}
	return r.deleteCRDVersion(ctx, crd, log, crdVersion)
}

func (r *CRDCleanupPolicyReconciler) deleteCRD(ctx context.Context, crd *v1.CustomResourceDefinition, log logr.Logger) error {
	if err := r.Delete(ctx, crd); err != nil {
		log.Error(err, "Failed to delete CRD", "CRD", crd.GetName())
		return err
	}
	log.Info("Successfully deleted CRD", "CRD", crd.GetName())
	return nil
}

func (r *CRDCleanupPolicyReconciler) deleteCRDVersion(ctx context.Context, crd *v1.CustomResourceDefinition, log logr.Logger, crdVersion string) error {
	// Remove the specific version from the CRD
	newVersions := filterVersions(crd.Spec.Versions, crdVersion)
	crd.Spec.Versions = newVersions

	if err := r.Update(ctx, crd); err != nil {
		log.Error(err, "Failed to update CRD", "CRD", crd.GetName(), "Version", crdVersion)
		return err
	}

	log.Info("Successfully removed version from CRD", "CRD", crd.GetName(), "Version", crdVersion)

	return nil
}

func filterVersions(versions []v1.CustomResourceDefinitionVersion, crdVersion string) []v1.CustomResourceDefinitionVersion {
	newVersions := []v1.CustomResourceDefinitionVersion{}
	for _, version := range versions {
		if version.Name != crdVersion {
			newVersions = append(newVersions, version)
		}
	}
	return newVersions
}

// updatePolicyStatus updates the status of the CRDCleanupPolicy
func (r *CRDCleanupPolicyReconciler) updatePolicyStatus(ctx context.Context, policy *policiesv1alpha1.CRDCleanupPolicy, updatedRemainingCRDs []string, log logr.Logger) error {
	policy.Status.RemainingCRDs = updatedRemainingCRDs

	if len(updatedRemainingCRDs) == 0 {
		policy.Status.StatusMessage = "All CRDs have been successfully processed."
		log.Info("All CRDs processed successfully")
	} else {
		policy.Status.StatusMessage = "Some CRDs are still pending deletion."
		log.Info("Some CRDs are still pending deletion", "RemainingCRDsCount", len(updatedRemainingCRDs))
	}

	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update CRDCleanupPolicy status", "policy", policy)
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CRDCleanupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&policiesv1alpha1.CRDCleanupPolicy{}).
		Complete(r)
}
