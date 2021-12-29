	/*
Copyright 2020 The Crossplane Authors.

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

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
)

//Enum for topic attributes
const(
	TopicDeliveryPolicy = "DeliveryPolicy"
	TopicDisplayName = "DisplayName"
	TopicPolicy = "Policy"
	FifoTopic = "FifoTopic"
	TopicKMSMasterKeyID = "KmsMasterKeyId"
	FifoTopicContentBasedDeduplication = "ContentBasedDeduplication"
	TopicSubscriptionConfirmed = "SubscriptionsConfirmed"
	TopicSubscriptionDeleted = "SubscriptionsDeleted"
	TopicSubscriptionPending = "SubscriptionsPending"
	TopicEffectiveDeliveryPolicy = "EffectiveDeliveryPolicy"
	TopicArn = "TopicArn"
)

//TopicParameters are the configurable fields of an Topic.
type TopicParameters struct {
	Region string `json:"region"`
	DeliveryPolicy *string `json:"deliveryPolicy,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
	Policy *string `json:"policy,omitempty"`
	FifoTopic *bool `json:"fifoTopic,omitempty"`
	ContentBasedDeduplication *bool `json:"contentBasedDeduplication,omitempty"`
	KMSMasterKeyID *string `json:"kmsMasterKeyId,omitempty"`
	Tags map[string]string `json:"tags,omitempty"`
}

//TopicObservation are the observable fields of an Topic.
type TopicObservation struct {
	// TopicArn – The topic's ARN
	TopicArn *string `json:"topicArn"`

	// SubscriptionsConfirmed – The number of
	// confirmed subscriptions for the topic.
	SubscriptionsConfirmed *int `json:"subscriptionsConfirmed,omitempty"`

	// SubscriptionsDeleted – The number of
	// deleted subscriptions for the topic.
	SubscriptionsDeleted *int `json:"subscriptionsDeleted,omitempty"`

	// SubscriptionsPending – The number of
	// subscriptions pending confirmation for the topic.
	SubscriptionsPending *int `json:"subscriptionsPending,omitempty"`

	// EffectiveDeliveryPolicy – The JSON serialization of the effective
	// delivery policy, taking system defaults into account.
	EffectiveDeliveryPolicy *string `json:"effectiveDeliveryPolicy,omitempty"`
}



// A TopicSpec defines the desired state of an Topic.
type TopicSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       TopicParameters `json:"forProvider"`
}

// A TopicStatus represents the observed state of an Topic.
type TopicStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          TopicObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A MyType is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,template}
type Topic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TopicSpec   `json:"spec"`
	Status TopicStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TopicList contains a list of Topics
type TopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items []Topic `json:"items"`
}

// Topic type metadata.
var (
	TopicKind             = reflect.TypeOf(Topic{}).Name()
	TopicGroupKind        = schema.GroupKind{Group: Group, Kind: TopicKind}.String()
	TopicKindAPIVersion   = TopicKind + "." + SchemeGroupVersion.String()
	TopicGroupVersionKind = SchemeGroupVersion.WithKind(TopicKind)
)

func init() {
	SchemeBuilder.Register(&Topic{}, &TopicList{})
}
