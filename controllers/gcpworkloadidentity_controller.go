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
	"bytes"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"context"
	"fmt"
	"github.com/googleapis/gax-go/v2"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/credentials/oauth"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	iam "cloud.google.com/go/iam/credentials/apiv1"
	projectxv1 "github.com/tony-mw/tenant-bootstrap/api/v1"
	"google.golang.org/api/option"
	credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	authenticationv1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcfg "sigs.k8s.io/controller-runtime/pkg/client/config"
	//iamauthed "cloud.google.com/go/iam"
)

type GcpWorkloadIdentityReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type gcpIDBindTokenGenerator struct {
	targetURL string
}

type k8sSATokenGenerator struct {
	corev1 clientcorev1.CoreV1Interface
}

var workloadIdConfig projectxv1.GcpWorkloadIdentity

func newIAMClient(ctx context.Context) (*iam.IamCredentialsClient, error) {
	iamOpts := []option.ClientOption{
		option.WithUserAgent("external-secrets-operator"),
		// tell the secretmanager library to not add transport-level ADC since
		// we need to override on a per call basis
		option.WithoutAuthentication(),
		// grpc oauth TokenSource credentials require transport security, so
		// this must be set explicitly even though TLS is used
		option.WithGRPCDialOption(grpc.WithTransportCredentials(credentials.NewTLS(nil))),
		option.WithGRPCConnectionPool(5),
	}
	return iam.NewIamCredentialsClient(ctx, iamOpts...)
}

func (g *gcpIDBindTokenGenerator) Generate(ctx context.Context, client *http.Client, k8sToken, idPool, idProvider string) (*oauth2.Token, error) {
	body, err := json.Marshal(map[string]string{
		"grant_type":           "urn:ietf:params:oauth:grant-type:token-exchange",
		"subject_token_type":   "urn:ietf:params:oauth:token-type:jwt",
		"requested_token_type": "urn:ietf:params:oauth:token-type:access_token",
		"subject_token":        k8sToken,
		"audience":             fmt.Sprintf("identitynamespace:%s:%s", idPool, idProvider),
		"scope":                "https://www.googleapis.com/auth/cloud-platform",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", g.targetURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get idbindtoken token, status: %v", resp.StatusCode)
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	idBindToken := &oauth2.Token{}
	if err := json.Unmarshal(respBody, idBindToken); err != nil {
		return nil, err
	}
	return idBindToken, nil
}

func (g *k8sSATokenGenerator) Generate(ctx context.Context, audiences []string, name, namespace string) (*authenticationv1.TokenRequest, error) {
	// Request a serviceaccount token for the pod
	ttl := int64((15 * time.Minute).Seconds())
	return g.corev1.
		ServiceAccounts(namespace).
		CreateToken(ctx, name,
			&authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{
					ExpirationSeconds: &ttl,
					Audiences:         audiences,
				},
			},
			metav1.CreateOptions{},
		)
}

func (sa *ServiceAccount) Exists(r *GcpWorkloadIdentityReconciler, ctx context.Context) bool {
	saObject := &core.ServiceAccount{}

	if err := r.Get(ctx, client.ObjectKey{Name: sa.Name, Namespace: sa.Namespace}, saObject); err != nil {
		return false
	} else {
		return true
	}
}

func (sa *ServiceAccount) CreateK8sWorkloadIdentity(r *GcpWorkloadIdentityReconciler, ctx context.Context, GcpConfig projectxv1.GcpWorkloadIdentityConfig) error {
	annotations := tenantConfig.GetAnnotations()
	annotations["iam.gke.io/gcp-service-account"] = fmt.Sprintf("%s@%s", GcpConfig.ServiceAccountName, GcpConfig.ProjectId)
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
			Name:      config.Kubernetes.ServiceAccountName,
			Namespace: config.Kubernetes.Namespace,
		}
		if !sa.Exists(r, ctx) {
			if err := sa.CreateK8sWorkloadIdentity(r, ctx, config.Gcp); err != nil {
				l.Error(err, "unable to create Kubernetes service account")
				return ctrl.Result{}, nil
			}
		}

		//Workload identity config - based on external secrets https://github.com/external-secrets/external-secrets/blob/ddd1de2390a60e00511fd1a5df21826fa7a64d1a/pkg/provider/gcp/secretmanager/workload_identity.go#L97
		//iamc, err := newIAMClient(ctx)
		//if err != nil {
		//	return nil, err
		//}

		saKey := types.NamespacedName{
			Name:      config.Gcp.WlAuth.ServiceAccountName,
			Namespace: config.Gcp.WlAuth.Namespace,
		}

		idProvider := fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/%s/clusters/%s",
			config.Gcp.WlAuth.ProjectId,
			config.Gcp.WlAuth.ClusterLocation,
			config.Gcp.WlAuth.ClusterName)
		idPool := fmt.Sprintf("%s.svc.id.goog", config.Gcp.WlAuth.ProjectId)
		audiences := []string{idPool}
		cfg, err := ctrlcfg.GetConfig()
		if err != nil {
			l.Error(err, "err")
		}
		clientset, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			l.Error(err, "err")
		}

		satg := &k8sSATokenGenerator{
			corev1: clientset.CoreV1(),
		}

		resp, err := satg.Generate(ctx, audiences, saKey.Name, saKey.Namespace)
		if err != nil {
			l.Error(err, "err")
		}
		idBindTokenGen := &gcpIDBindTokenGenerator{
			targetURL: "https://securetoken.googleapis.com/v1/identitybindingtoken",
		}

		idBindToken, err := idBindTokenGen.Generate(ctx, http.DefaultClient, resp.Status.Token, idPool, idProvider)
		if err != nil {
			l.Error(err, "err")
		}

		iamc, err := newIAMClient(ctx)
		if err != nil {
			l.Error(err, "err")
		}

		gcpSAResp, err := iamc.GenerateAccessToken(ctx, &credentialspb.GenerateAccessTokenRequest{
			Name:  fmt.Sprintf("projects/-/serviceAccounts/%s", config.Gcp.ServiceAccountName),
			Scope: secretmanager.DefaultAuthScopes(),
		}, gax.WithGRPCOptions(grpc.PerRPCCredentials(oauth.TokenSource{TokenSource: oauth2.StaticTokenSource(idBindToken)})))
		if err != nil {
			l.Error(err, "err")
		}
		tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: gcpSAResp.GetAccessToken(),
		})

		//TODO - use the Authenticated iam client
		l.Info("Got a token.", "token", tokenSource)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GcpWorkloadIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectxv1.GcpWorkloadIdentity{}).
		Complete(r)
}
