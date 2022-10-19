package controllers

import (
	"context"
	projectxv1 "github.com/MavenWave-DevOps/px-tenant-provisioner/api/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TenantNamespaceReconciler reconciles a TenantNamespace object
type TenantNamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var namespaceConfig projectxv1.TenantNamespace

func (r *TenantNamespaceReconciler) CreateNamespace(ctx context.Context, ns *core.Namespace) error {
	if err := ctrl.SetControllerReference(&namespaceConfig, ns, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, ns); err != nil {
		return err
	} else {
		return nil
	}
}

//+kubebuilder:rbac:groups=projectx.github.com,resources=tenantnamespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projectx.github.com,resources=tenantnamespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=projectx.github.com,resources=tenantnamespaces/finalizers,verbs=update

// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *TenantNamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l = log.FromContext(ctx)
	if err := r.Get(ctx, req.NamespacedName, &namespaceConfig); err != nil {
		l.Error(err, "Unable to load config")
		return ctrl.Result{}, nil
	}
	for _, namespace := range namespaceConfig.Spec.Namespaces {
		l.Info("Namespace name", "ns", namespaceConfig.Spec.Namespaces)
		ns := ConstructNamespace(namespace)
		//Check if ns already exists
		if ok, _ := r.CheckNamespace(ctx, req, ns); !ok {
			//Ns doesn't exist - create it now
			if err := r.CreateNamespace(ctx, ns); err != nil {
				l.Error(err, "could not create namespace")
				l.Info("attempted", "namespaceConfig", namespaceConfig)
				return ctrl.Result{}, nil
			}
			l.Info("Created namespace!")
			continue
		} else {
			//Namespace already exists
			l.Info("namespace already exists")
			continue
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectxv1.TenantNamespace{}).
		Owns(&core.Namespace{}).
		Complete(r)
}
