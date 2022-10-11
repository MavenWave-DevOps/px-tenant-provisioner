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
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	projectxv1 "github.com/tony-mw/tenant-bootstrap/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TenantNamespaceReconciler reconciles a TenantNamespace object
type TenantNamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *TenantNamespaceReconciler) CreateNamespace(ctx context.Context, spec projectxv1.TenantNamespaceSpec) error {
	ns := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: spec.Namespace,
		},
	}
	//if err := controllerutil.SetControllerReference(&tenantConfig, ns, r.Scheme); err != nil {
	//	l.Error(err, "couldnt set namespace ref")
	//}
	if err := r.Create(ctx, ns); err != nil {
		return err
	} else {
		return nil
	}
}

//+kubebuilder:rbac:groups=projectx.github.com,resources=tenantnamespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projectx.github.com,resources=tenantnamespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=projectx.github.com,resources=tenantnamespaces/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TenantNamespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *TenantNamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l = log.FromContext(ctx)
	//Check to see if we should create the namespace
	var namespaceConfig projectxv1.TenantNamespace
	if err := r.Get(ctx, req.NamespacedName, &namespaceConfig); err != nil {
		l.Error(err, "Unable to load config")
		return ctrl.Result{}, nil
	}
	l.Info("namespace name: ", "namespace name", req.NamespacedName.Name, "namespace info", req.NamespacedName.Namespace)

	if err := r.CreateNamespace(ctx, namespaceConfig.Spec); err != nil {
		l.Error(err, "could not create namespace")
		l.Info("attempted", "namespace", namespaceConfig.Spec.Namespace)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectxv1.TenantNamespace{}).
		Complete(r)
}
