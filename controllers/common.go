package controllers

import (
	"context"
	"errors"
	projectxv1 "github.com/MavenWave-DevOps/px-tenant-provisioner/api/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// Retry times
var Retry int = 5

// Time interval to wait for namespace in seconds
var RetryInterval time.Duration = time.Duration(time.Second * 5)

type Reconciler interface {
	exists(ctx context.Context, req ctrl.Request, ns *core.Namespace) bool
}

type TenantNamespace struct {
	NamespaceSpec projectxv1.TenantNamespaceSpec
}

func (r *TenantNamespaceReconciler) CheckNamespace(ctx context.Context, req ctrl.Request, ns *core.Namespace) (bool, *core.Namespace) {
	if err := r.Get(ctx, req.NamespacedName, ns); err != nil {
		return false, nil
	} else {
		return true, ns
	}
}

func (r *TenantBootstrapReconciler) exists(ctx context.Context, req ctrl.Request, ns *core.Namespace) bool {
	if err := r.Get(ctx, req.NamespacedName, ns); err != nil {
		return true
	} else {
		return false
	}
}

func (r *GcpWorkloadIdentityReconciler) exists(ctx context.Context, req ctrl.Request, ns *core.Namespace) bool {
	if err := r.Get(ctx, req.NamespacedName, ns); err != nil {
		return true
	} else {
		return false
	}
}

func ConstructNamespace(nsName string) *core.Namespace {
	return &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nsName,
			Annotations: namespaceConfig.GetAnnotations(),
			Labels:      namespaceConfig.GetLabels(),
		},
	}
}

func (ns TenantNamespace) CheckNs(ctx context.Context, req ctrl.Request, r Reconciler) bool {
	for _, namespace := range ns.NamespaceSpec.Namespaces {
		ns := ConstructNamespace(namespace)
		l.Info("checking for namespace", "ns", ns.Name)

		nsExists := false
		for j := 0; j < Retry; j++ {
			if ok := r.exists(ctx, req, ns); ok != true {
				l.Error(errors.New("ns check failed"), "ns check returned an error")
				time.Sleep(RetryInterval)
				continue
			} else {
				nsExists = true
				break
			}
		}
		if nsExists == false {
			return false
		}
	}
	return true
}
