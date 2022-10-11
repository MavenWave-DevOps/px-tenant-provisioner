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
	"github.com/go-logr/logr"
	projectxv1 "github.com/tony-mw/tenant-bootstrap/api/v1"
	core "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// TenantBootstrapReconciler reconciles a TenantBootstrap object
type TenantBootstrapReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type SubjectKind interface {
	ConstructSubject(s projectxv1.Subject, ns string) rbacv1.Subject
	CreateIdentity(ctx context.Context, s projectxv1.Subject, r *TenantBootstrapReconciler, ns string) error
}

type ServiceAccount struct {
	Kind string
}

type User struct {
	Kind string
}

var l logr.Logger
var tenantConfig projectxv1.TenantBootstrap

//+kubebuilder:rbac:groups=projectx.github.com,resources=tenantbootstraps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projectx.github.com,resources=tenantbootstraps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=projectx.github.com,resources=tenantbootstraps/finalizers,verbs=update
//+kubebuilder:rbac:groups=projectx.github.com,resources=*,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TenantBootstrap object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile

func ConstructRule(rule projectxv1.RbacRule) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		Verbs:     rule.Verbs,
		APIGroups: rule.ApiGroups,
		Resources: rule.Resources,
	}
}

func ConstructRole(rc projectxv1.Rbac, ns string) rbacv1.Role {
	var rules []rbacv1.PolicyRule
	for _, v := range rc.Rules {
		rules = append(rules, ConstructRule(v))
	}
	return rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rc.RoleName,
			Namespace: ns,
		},
		Rules: rules,
	}
}

func (u *User) ConstructSubject(s projectxv1.Subject, ns string) rbacv1.Subject {
	return rbacv1.Subject{
		Kind:      u.Kind,
		Name:      s.Name,
		Namespace: ns,
	}
}

func (sa *ServiceAccount) ConstructSubject(s projectxv1.Subject, ns string) rbacv1.Subject {
	return rbacv1.Subject{
		Kind:      sa.Kind,
		Name:      s.Name,
		Namespace: ns,
	}
}

func (u *User) CreateIdentity(ctx context.Context, s projectxv1.Subject, r *TenantBootstrapReconciler, ns string) error {
	return nil
}

func (sa *ServiceAccount) CreateIdentity(ctx context.Context, s projectxv1.Subject, r *TenantBootstrapReconciler, ns string) error {
	newSa := &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: ns,
		},
	}
	if err := controllerutil.SetControllerReference(&tenantConfig, newSa, r.Scheme); err != nil {
		l.Error(err, "couldnt set namespace ref")
	}

	if err := r.Create(ctx, newSa); err != nil {
		return err
	}
	return nil
}

func (r *TenantBootstrapReconciler) ConstructRoleBinding(rn string, ns string, subjects []rbacv1.Subject) rbacv1.RoleBinding {
	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rb", rn),
			Namespace: ns,
		},
		Subjects: subjects,
		RoleRef: rbacv1.RoleRef{
			Name: rn,
			Kind: "Role",
		},
	}
}

func (r *TenantBootstrapReconciler) CreateRbac(ctx context.Context, spec projectxv1.TenantBootstrapSpec) error {
	ns := tenantConfig.Namespace
	for _, roleConfig := range spec.Rbac {
		role := ConstructRole(roleConfig, ns)
		if err := controllerutil.SetControllerReference(&tenantConfig, &role, r.Scheme); err != nil {
			l.Error(err, "couldnt set namespace ref")
		}
		if err := r.Create(ctx, &role); err != nil {
			return err
		}
		//Create roleBindings per role
		var subjects []rbacv1.Subject
		for j := 0; j < len(roleConfig.Subjects); j++ {
			var subKind SubjectKind
			switch roleConfig.Subjects[j].Kind {
			case "serviceAccount":
				subKind = &ServiceAccount{
					Kind: "ServiceAccount",
				}
			case "User":
				subKind = &User{
					Kind: "user",
				}
			default:
				subKind = &ServiceAccount{
					Kind: "ServiceAccount",
				}
			}
			subjects = append(subjects, subKind.ConstructSubject(roleConfig.Subjects[j], ns))
			if roleConfig.Subjects[j].Create {
				if err := subKind.CreateIdentity(ctx, roleConfig.Subjects[j], r, ns); err != nil {
					return err
				}
			}
		}
		rb := r.ConstructRoleBinding(roleConfig.RoleName, ns, subjects)
		if err := controllerutil.SetControllerReference(&tenantConfig, &rb, r.Scheme); err != nil {
			l.Error(err, "couldnt set namespace ref")
		}
		if err := r.Create(ctx, &rb); err != nil {
			l.Error(err, "couldnt create role binding")
		}
	}
	return nil
}
func (r *TenantBootstrapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	if err := r.Get(ctx, req.NamespacedName, &tenantConfig); err != nil {
		l.Error(err, "Unable to load config")
		return ctrl.Result{}, nil
	}
	l.Info("namespace name: ", "namespace name", req.NamespacedName.Name, "namespace info", req.NamespacedName.Namespace)

	if err := r.CreateRbac(ctx, tenantConfig.Spec); err != nil {
		l.Error(err, "could not create rbac")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantBootstrapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectxv1.TenantBootstrap{}).
		Complete(r)
}
