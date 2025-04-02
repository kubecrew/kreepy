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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/jaydee94/kreepy/api/v1alpha1"
)

// CRDCleanupPolicyReconciler reconciles a CRDCleanupPolicy object
type CRDCleanupPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=policies.kreepy.jays-lab.de,resources=crdcleanuppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policies.kreepy.jays-lab.de,resources=crdcleanuppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=policies.kreepy.jays-lab.de,resources=crdcleanuppolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions/finalizers,verbs=update

func (r *CRDCleanupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Starting reconciliation for CRDCleanupPolicy", "name", req.NamespacedName)

	// Fetch the CRDCleanupPolicy object
	policy, err := r.fetchPolicy(ctx, req, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Initialize status fields if needed
	r.initializeStatusFields(&policy, log)

	// Process CRDs
	updatedRemainingCRDs, err := r.processCRDs(ctx, &policy, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update the status
	err = r.updatePolicyStatus(ctx, &policy, updatedRemainingCRDs, log)
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
func (r *CRDCleanupPolicyReconciler) fetchPolicy(ctx context.Context, req ctrl.Request, log logr.Logger) (policiesv1alpha1.CRDCleanupPolicy, error) {
	var policy policiesv1alpha1.CRDCleanupPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to fetch CRDCleanupPolicy")
		} else {
			log.Info("CRDCleanupPolicy not found, it might have been deleted")
		}
		return policy, client.IgnoreNotFound(err)
	}
	log.Info("Fetched CRDCleanupPolicy", "name", policy.Name)
	return policy, nil
}

// initializeStatusFields initializes the status fields of the CRDCleanupPolicy if they are nil
func (r *CRDCleanupPolicyReconciler) initializeStatusFields(policy *policiesv1alpha1.CRDCleanupPolicy, log logr.Logger) {
	if policy.Status.ProcessedCRDs == nil {
		policy.Status.ProcessedCRDs = []string{}
	}
	if policy.Status.RemainingCRDs == nil {
		policy.Status.RemainingCRDs = policy.Spec.CRDs
	}
}

// processCRDs processes the CRDs listed in the policy and deletes them
func (r *CRDCleanupPolicyReconciler) processCRDs(ctx context.Context, policy *policiesv1alpha1.CRDCleanupPolicy, log logr.Logger) ([]string, error) {
	var updatedRemainingCRDs []string

	for _, crdName := range policy.Status.RemainingCRDs {
		log.Info("Processing CRD", "Name", crdName)

		// Fetch the CRD definition to get its group, version, and kind
		crd, err := r.fetchCRDDefinition(ctx, crdName, log)
		if err != nil {
			log.Error(err, "Failed to fetch CRD definition", "Name", crdName)
			updatedRemainingCRDs = append(updatedRemainingCRDs, crdName)
			continue
		}

		// Check if there are any instances of this CRD in the cluster
		hasInstances, err := r.checkCRDInstances(ctx, crd, log)
		if err != nil {
			updatedRemainingCRDs = append(updatedRemainingCRDs, crdName)
			continue
		}

		// If there are instances, skip deletion
		if hasInstances {
			log.Info("Instances of CRD found, skipping deletion", "CRD", crdName)
			updatedRemainingCRDs = append(updatedRemainingCRDs, crdName)
			continue
		}

		// Attempt to delete the CRD
		if err := r.deleteCRD(ctx, crd, log); err != nil {
			updatedRemainingCRDs = append(updatedRemainingCRDs, crdName)
			continue
		}

		// Add to processed CRDs
		log.Info("Successfully deleted CRD", "CRD", crdName)
		policy.Status.ProcessedCRDs = append(policy.Status.ProcessedCRDs, crdName)
	}

	return updatedRemainingCRDs, nil
}

// fetchCRDDefinition fetches the CRD definition by name and returns its group, version, and kind
func (r *CRDCleanupPolicyReconciler) fetchCRDDefinition(ctx context.Context, crdName string, log logr.Logger) (*unstructured.Unstructured, error) {
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})
	crd.SetName(crdName)

	if err := r.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
		log.Error(err, "Failed to fetch CRD definition", "Name", crdName)
		return nil, err
	}

	return crd, nil
}

// checkCRDInstances checks if there are any instances of the given CRD in the cluster
func (r *CRDCleanupPolicyReconciler) checkCRDInstances(ctx context.Context, crd *unstructured.Unstructured, log logr.Logger) (bool, error) {
	group, _, _ := unstructured.NestedString(crd.Object, "spec", "group")
	version, _, _ := unstructured.NestedString(crd.Object, "spec", "versions", "0", "name")
	plural, _, _ := unstructured.NestedString(crd.Object, "spec", "names", "plural")

	if group == "" {
		log.Error(nil, "Failed to extract 'group' from CRD", "CRD", crd.GetName())
	}
	if version == "" {
		log.Error(nil, "Failed to extract 'version' from CRD", "CRD", crd.GetName())
	}
	if plural == "" {
		log.Error(nil, "Failed to extract 'plural' from CRD", "CRD", crd.GetName())
	}

	if group == "" || version == "" || plural == "" {
		log.Info("Skipping CRD due to missing group, version, or plural", "CRD", crd.GetName())
		return false, nil
	}

	crdGVR := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: plural,
	}

	var instances unstructured.UnstructuredList
	instances.SetGroupVersionKind(crdGVR.GroupVersion().WithKind(plural))
	if err := r.List(ctx, &instances); err != nil {
		log.Error(err, "Failed to list instances of CRD", "CRD", crd.GetName())
		return false, err
	}

	return len(instances.Items) > 0, nil
}

// deleteCRD deletes the given CRD from the cluster
func (r *CRDCleanupPolicyReconciler) deleteCRD(ctx context.Context, crd *unstructured.Unstructured, log logr.Logger) error {
	if err := r.Delete(ctx, crd); err != nil {
		log.Error(err, "Failed to delete CRD", "CRD", crd.GetName())
		return err
	}

	return nil
}

// updatePolicyStatus updates the status of the CRDCleanupPolicy
func (r *CRDCleanupPolicyReconciler) updatePolicyStatus(ctx context.Context, policy *policiesv1alpha1.CRDCleanupPolicy, updatedRemainingCRDs []string, log logr.Logger) error {
	// Validate that the policy has a name and namespace
	if policy.Name == "" || policy.Namespace == "" {
		log.Error(nil, "CRDCleanupPolicy resource name or namespace is empty", "policy", policy)
		return fmt.Errorf("CRDCleanupPolicy resource name or namespace is empty")
	}

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
