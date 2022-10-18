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

func (r *TenantNamespaceReconciler) CheckNamespace(ctx context.Context, req ctrl.Request, ns *core.Namespace) (bool, *core.Namespace) {

	if err := r.Get(ctx, req.NamespacedName, ns); err != nil {
		return false, nil
	} else {
		return true, ns
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

	ns := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        namespaceConfig.Spec.Namespace,
			Annotations: namespaceConfig.GetAnnotations(),
			Labels:      namespaceConfig.GetLabels(),
		},
	}

	//Check if ns already exists
	if ok, _ := r.CheckNamespace(ctx, req, ns); !ok {
		//Ns doesn't exist - create it now
		if err := r.CreateNamespace(ctx, ns); err != nil {
			l.Error(err, "could not create namespace")
			l.Info("attempted", "namespace", namespaceConfig.Spec.Namespace)
			return ctrl.Result{}, nil
		}
		l.Info("Created namespace!")
		return ctrl.Result{}, nil
	} else {
		//Namespace already exists
		l.Info("namespace already exists")
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectxv1.TenantNamespace{}).
		Owns(&core.Namespace{}).
		Complete(r)
}
