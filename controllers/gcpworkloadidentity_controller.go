/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"golang.org/x/oauth2/google"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	projectxv1 "github.com/tony-mw/tenant-bootstrap/api/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GcpWorkloadIdentityReconciler reconciles a GcpWorkloadIdentity object
type GcpWorkloadIdentityReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var workloadIdConfig projectxv1.GcpWorkloadIdentity

func (sa *ServiceAccount) Exists(r *GcpWorkloadIdentityReconciler, ctx context.Context) bool {
	saObject := &core.ServiceAccount{}

	if err := r.Get(ctx, client.ObjectKey{ Name: sa.Name, Namespace: sa.Namespace}, saObject); err != nil {
		return false
	} else {
		return true
	}
}

func (sa *ServiceAccount) CreateK8sWorkloadIdentity(r *GcpWorkloadIdentityReconciler, ctx context.Context, GcpConfig projectxv1.GcpWorkloadIdentityConfig) error {
	annotations := tenantConfig.GetAnnotations()
	annotations["iam.gke.io/gcp-service-account"]=fmt.Sprintf("%s@%s",GcpConfig.ServiceAccountName, GcpConfig.ProjectId)
	newSa := &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        sa.Name,
			Namespace:   sa.Namespace,
			Annotations: annotations,
			Labels:      workloadIdConfig.GetLabels(),
		},
	}
	if err := controllerutil.SetControllerReference(&workloadIdConfig, newSa, r.Scheme); err != nil {
		l.Error(err, "couldnt set wl id owner")
	}
	if err := r.Create(ctx, newSa); err != nil {
		return err
	}
	return nil
}

//+kubebuilder:rbac:groups=projectx.github.com,resources=gcpworkloadidentities,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projectx.github.com,resources=gcpworkloadidentities/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=projectx.github.com,resources=gcpworkloadidentities/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GcpWorkloadIdentity object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *GcpWorkloadIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//Populate our struct
	if err := r.Get(ctx, req.NamespacedName, &workloadIdConfig); err != nil {
		l.Error(err, "unable to load config")
		return ctrl.Result{}, nil
	}

	//Check for K8s Sa
	for _, config := range workloadIdConfig.Spec.WorkloadIdentityConfigs {
		sa := &ServiceAccount{
			Name: config.Kubernetes.ServiceAccountName,
			Namespace: config.Kubernetes.Namespace,
		}
		if !sa.Exists(r, ctx) {
			if err := sa.CreateK8sWorkloadIdentity(r, ctx, config.Gcp); err != nil {
				l.Error(err, "unable to create Kubernetes service account")
				return ctrl.Result{}, nil
			}
		}

		//Create GCP Service Account and Attach roles
		google.FindDefaultCredentials()
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GcpWorkloadIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectxv1.GcpWorkloadIdentity{}).
		Complete(r)
}
