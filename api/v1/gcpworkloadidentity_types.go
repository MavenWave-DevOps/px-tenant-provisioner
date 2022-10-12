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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type K8sWorkloadIdentityConfig struct {
	ServiceAccountName string `json:"serviceAccountName"`
	Namespace          string `json:"namespace"`
}

type Auth struct {
	ServiceAccountName string `json:"serviceAccountName"`
	ProjectId          string `json:"projectId"`
	ClusterLocation    string `json:"clusterLocation"`
	ClusterName        string `json:"clusterName"`
	Namespace          string `json:"namespace"`
}

type GcpWorkloadIdentityConfig struct {
	ProjectId          string   `json:"projectId"`
	ServiceAccountName string   `json:"serviceAccountName"`
	IamRoles           []string `json:"iamRoles"`
	WlAuth             Auth     `json:"auth"`
}
type WorkloadIdentityConfig struct {
	Kubernetes K8sWorkloadIdentityConfig `json:"kubernetes"`
	Gcp        GcpWorkloadIdentityConfig `json:"gcp"`
}

// GcpWorkloadIdentitySpec defines the desired state of GcpWorkloadIdentity
type GcpWorkloadIdentitySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of GcpWorkloadIdentity. Edit gcpworkloadidentity_types.go to remove/update
	WorkloadIdentityConfigs []WorkloadIdentityConfig `json:"workloadIdentityConfigs"`
}

// GcpWorkloadIdentityStatus defines the observed state of GcpWorkloadIdentity
type GcpWorkloadIdentityStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GcpWorkloadIdentity is the Schema for the gcpworkloadidentities API
type GcpWorkloadIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GcpWorkloadIdentitySpec   `json:"spec,omitempty"`
	Status GcpWorkloadIdentityStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GcpWorkloadIdentityList contains a list of GcpWorkloadIdentity
type GcpWorkloadIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GcpWorkloadIdentity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GcpWorkloadIdentity{}, &GcpWorkloadIdentityList{})
}
