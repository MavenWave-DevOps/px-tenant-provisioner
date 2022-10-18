package controllers

import (
	"bytes"
	"context"
	"fmt"
	"github.com/googleapis/gax-go/v2"
	"golang.org/x/oauth2"
	iamclient "google.golang.org/api/iam/v1"
	"google.golang.org/grpc/credentials/oauth"
	"io"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"strings"

	//v1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	iam "cloud.google.com/go/iam/credentials/apiv1"
	projectxv1 "github.com/tony-mw/tenant-bootstrap/api/v1"
	"github.com/MavenWave-DevOps/px-tenant-provisioner/common/utils"
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

type GcpServiceAccount struct {
	Workload types.NamespacedName
}

var workloadIdConfig projectxv1.GcpWorkloadIdentity
var annotations map[string]string

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

func (r *GcpWorkloadIdentityReconciler) Exists(sa ServiceAccount, ctx context.Context) bool {
	saObject := &core.ServiceAccount{}

	if err := r.Get(ctx, client.ObjectKey{Name: sa.Name, Namespace: sa.Namespace}, saObject); err != nil {
		return false
	} else {
		return true
	}
}

func (sa *ServiceAccount) CreateK8sWorkloadIdentity(r *GcpWorkloadIdentityReconciler, ctx context.Context, GcpConfig projectxv1.GcpWorkloadIdentityConfig) error {

	annotations = tenantConfig.GetAnnotations()
	if len(annotations) == 0 {
		annotations = map[string]string{}
	}
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

func (r *GcpWorkloadIdentityReconciler) GcpAuth(ctx context.Context, saKey types.NamespacedName, config projectxv1.WorkloadIdentityConfig) oauth2.TokenSource {
	//Workload identity federation auth - based on external secrets https://github.com/external-secrets/external-secrets/blob/ddd1de2390a60e00511fd1a5df21826fa7a64d1a/pkg/provider/gcp/secretmanager/workload_identity.go#L97

	adminSa := &v1core.ServiceAccount{}
	err := r.Get(ctx, saKey, adminSa)
	utils.CheckErr(err, "could not get service account")

	idProvider := fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/%s/clusters/%s",
		config.Gcp.WlAuth.ProjectId,
		config.Gcp.WlAuth.ClusterLocation,
		config.Gcp.WlAuth.ClusterName)
	idPool := fmt.Sprintf("%s.svc.id.goog", config.Gcp.WlAuth.ProjectId)
	audiences := []string{idPool}
	cfg, err := ctrlcfg.GetConfig()
	utils.CheckErr(err, "couldnt get config")
	clientset, err := kubernetes.NewForConfig(cfg)
	utils.CheckErr(err, "couldnt create clientset")

	satg := &k8sSATokenGenerator{
		corev1: clientset.CoreV1(),
	}

	resp, err := satg.Generate(ctx, audiences, saKey.Name, saKey.Namespace)
	utils.CheckErr(err, "couldnt generate a token")
	idBindTokenGen := &gcpIDBindTokenGenerator{
		targetURL: "https://securetoken.googleapis.com/v1/identitybindingtoken",
	}

	idBindToken, err := idBindTokenGen.Generate(ctx, http.DefaultClient, resp.Status.Token, idPool, idProvider)
	utils.CheckErr(err, "couldnt get a bind token")

	iamc, err := newIAMClient(ctx)
	utils.CheckErr(err, "could not create iam client")

	gcpSAResp, err := iamc.GenerateAccessToken(ctx, &credentialspb.GenerateAccessTokenRequest{
		Name:  fmt.Sprintf("projects/-/serviceAccounts/%s", adminSa.Annotations["iam.gke.io/gcp-service-account"]),
		Scope: iam.DefaultAuthScopes(),
	}, gax.WithGRPCOptions(grpc.PerRPCCredentials(oauth.TokenSource{TokenSource: oauth2.StaticTokenSource(idBindToken)})))
	utils.CheckErr(err, fmt.Sprintf("Identity is: %s", adminSa.Annotations["iam.gke.io/gcp-service-account"]))

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: gcpSAResp.GetAccessToken(),
	})

	return tokenSource
}

// DeleteGcpWorkloadIdentities projects/px-tony-project-rqt/serviceAccounts/tenantconsumerc@px-tony-project-rqt.iam.gserviceaccount.com
func DeleteGcpWorkloadIdentities(ctx context.Context, config projectxv1.WorkloadIdentityConfig, tokenSource oauth2.TokenSource) error {
	accountId := strings.Replace(config.Gcp.ServiceAccountName, "-", "", -1)
	service, err := iamclient.NewService(ctx, option.WithTokenSource(tokenSource))
	//Check if SA already exists
	_, err = service.Projects.ServiceAccounts.Delete(fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", config.Gcp.ProjectId, accountId, config.Gcp.ProjectId)).Do()
	if err != nil {
		l.Error(err, "couldnt create sa")
		return err
	}
	l.Info("Deleted Service Account")
	return nil
}

func CreateGcpWorkloadIdentities(ctx context.Context, config projectxv1.WorkloadIdentityConfig, tokenSource oauth2.TokenSource) error {

	var account *iamclient.ServiceAccount

	l.Info("Creating GCP SA")
	accountId := strings.Replace(config.Gcp.ServiceAccountName, "-", "", -1)
	service, err := iamclient.NewService(ctx, option.WithTokenSource(tokenSource))
	l.Info("Created client OK")
	//Check if SA already exists
	checkSa, err := service.Projects.ServiceAccounts.Get(fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", config.Gcp.ProjectId, accountId, config.Gcp.ProjectId)).Do()
	//var account *iamclient.ServiceAccount
	if err == nil {
		l.Info("Service Account exists", "sa:", checkSa.Name)
		return nil
	}
	l.Error(err, fmt.Sprintf("error retrieving service account"))
	request := &iamclient.CreateServiceAccountRequest{
		AccountId: accountId,
		ServiceAccount: &iamclient.ServiceAccount{
			DisplayName: config.Gcp.ServiceAccountName,
		},
	}
	account, err = service.Projects.ServiceAccounts.Create(fmt.Sprintf("projects/%s", config.Gcp.ProjectId), request).Do()
	utils.CheckErr(err, "couldnt create sa")
	var Bindings []*iamclient.Binding

	b := &iamclient.Binding{
		Members: []string{fmt.Sprintf("serviceAccount:%s.svc.id.goog[%s/%s]", config.Gcp.ProjectId, config.Kubernetes.Namespace, config.Kubernetes.ServiceAccountName)},
		Role:    "roles/iam.workloadIdentityUser",
	}

	Bindings = append(Bindings, b)
	Policy := &iamclient.Policy{
		Bindings: Bindings,
	}
	wlIdPolicyRequest := &iamclient.SetIamPolicyRequest{
		Policy: Policy,
	}

	l.Info("Creating additional service")
	saService := iamclient.NewProjectsServiceAccountsService(service)
	l.Info("Setting policy")
	r, err := saService.SetIamPolicy(account.Name, wlIdPolicyRequest).Do()
	l.Info("policy was set")
	if err != nil {
		l.Error(err, "couldnt set binding")
		return err
	}
	l.Info("Created wl id binding", "info", r.Bindings)

	return nil
}

//+kubebuilder:rbac:groups=projectx.github.com,resources=gcpworkloadidentities,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=projectx.github.com,resources=gcpworkloadidentities/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=projectx.github.com,resources=gcpworkloadidentities/finalizers,verbs=update

// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *GcpWorkloadIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var deleteSa bool = false

	//Populate our struct
	if err := r.Get(ctx, req.NamespacedName, &workloadIdConfig); err != nil {
		deleteSa = true
		l.Info("Crd is gone, deleting GCP serviceAccounts")
	}
	l.Info("Workload identity configs: ", "Length", len(workloadIdConfig.Spec.WorkloadIdentityConfigs))
	for _, config := range workloadIdConfig.Spec.WorkloadIdentityConfigs {
		saKey := GcpServiceAccount{
			Workload: types.NamespacedName{
				Name:      config.Gcp.WlAuth.ServiceAccountName,
				Namespace: config.Gcp.WlAuth.Namespace,
			},
		}
		tokenSource := r.GcpAuth(ctx, saKey.Workload, config)
		t, err := tokenSource.Token()
		utils.CheckErr(err, "Error getting tokensource")
		l.Info("Got a token.", "token", t)
		if deleteSa {
			if err := DeleteGcpWorkloadIdentities(ctx, config, tokenSource); err != nil {
				l.Error(err, "error deleting sa")
			}
		} else {
			l.Info("Creating SA")
			sa := ServiceAccount{
				Name:      config.Kubernetes.ServiceAccountName,
				Namespace: config.Kubernetes.Namespace,
			}
			if !r.Exists(sa, ctx) {
				l.Info("Entering create k8s SA function")
				if err := sa.CreateK8sWorkloadIdentity(r, ctx, config.Gcp); err != nil {
					l.Error(err, "unable to create Kubernetes service account")
					return ctrl.Result{}, nil
				}
			}
			l.Info("Entering create GCP SA function")
			if err := CreateGcpWorkloadIdentities(ctx, config, tokenSource); err != nil {
				l.Error(err, "error creating sa")
			}
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GcpWorkloadIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&projectxv1.GcpWorkloadIdentity{}).
		Owns(&core.ServiceAccount{}).
		Complete(r)
}
