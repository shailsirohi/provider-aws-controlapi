package sns

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/smithy-go"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"provider-aws-controlapi/apis/sns/v1alpha1"
	awsclient "provider-aws-controlapi/internal/clients"
	"strconv"
	"strings"
)

const (
	// TopicNotFound is the error code send by AWS API
	// if topic doesn't exist
	TopicNotFound = "NotFound"
)

type Client interface {
	CreateTopic(ctx context.Context, params *awssns.CreateTopicInput, optFns ...func(*awssns.Options)) (*awssns.CreateTopicOutput, error)
	DeleteTopic(ctx context.Context, params *awssns.DeleteTopicInput, optFns ...func(*awssns.Options)) (*awssns.DeleteTopicOutput, error)
	GetTopicAttributes(ctx context.Context, params *awssns.GetTopicAttributesInput, optFns ...func(*awssns.Options)) (*awssns.GetTopicAttributesOutput, error)
	SetTopicAttributes(ctx context.Context, params *awssns.SetTopicAttributesInput, optFns ...func(*awssns.Options)) (*awssns.SetTopicAttributesOutput, error)
	TagResource(ctx context.Context, params *awssns.TagResourceInput, optFns ...func(*awssns.Options)) (*awssns.TagResourceOutput, error)
	UntagResource(ctx context.Context, params *awssns.UntagResourceInput, optFns ...func(*awssns.Options)) (*awssns.UntagResourceOutput, error)
	ListTagsForResource(ctx context.Context, params *awssns.ListTagsForResourceInput, optFns ...func(*awssns.Options)) (*awssns.ListTagsForResourceOutput, error)
}

//GetClient returns the aws client for calling AWS SNS Apis
func GetClient(cfg aws.Config) Client{
	client := awssns.NewFromConfig(cfg)
	return client
}

// IsNotFound checks if the error returned by AWS API says that the queue being probed doesn't exist
func IsNotFound(err error) bool {
	var awsErr smithy.APIError
	return errors.As(err, &awsErr) && awsErr.ErrorCode() == TopicNotFound
}


// LateInitialize fills the empty fields in *v1alpha1.TopicParameters with
// the values returned by GetTopicAttributes
func LateInitialize(in *v1alpha1.TopicParameters,attributes map[string]string, tags []types.Tag){
	if in.Tags == nil && len(tags) > 0 {
		in.Tags = map[string]string{}
		for _, v := range tags {
			in.Tags[aws.ToString(v.Key)] = aws.ToString(v.Value)
		}
	}

	in.FifoTopic = awsclient.LateInitializeBoolPtr(in.FifoTopic,awsclient.StrToBoolPtr(attributes[v1alpha1.FifoTopic]))
	in.DeliveryPolicy = awsclient.LateInitializeStringPtr(in.DeliveryPolicy,aws.String(attributes[v1alpha1.TopicDeliveryPolicy]))
	in.DisplayName = awsclient.LateInitializeStringPtr(in.DisplayName,aws.String(attributes[v1alpha1.TopicDisplayName]))
	in.Policy = awsclient.LateInitializeStringPtr(in.Policy,aws.String(attributes[v1alpha1.TopicPolicy]))
	in.ContentBasedDeduplication = awsclient.LateInitializeBoolPtr(in.ContentBasedDeduplication,awsclient.StrToBoolPtr(attributes[v1alpha1.FifoTopicContentBasedDeduplication]))
	if in.KMSMasterKeyID == nil && attributes[v1alpha1.TopicKMSMasterKeyID] != ""{
		in.KMSMasterKeyID = aws.String(attributes[v1alpha1.TopicKMSMasterKeyID])
	}
}

// GenerateObservation generates the observation for the Topic object
// based on the Topic attributes received from AWS
func GenerateObservation(attributes map[string]string) v1alpha1.TopicObservation{

	ob := v1alpha1.TopicObservation{
		TopicArn: aws.String(attributes[v1alpha1.TopicArn]),
		SubscriptionsConfirmed: awsclient.StrToIntPtr(attributes[v1alpha1.TopicSubscriptionConfirmed]),
		SubscriptionsPending: awsclient.StrToIntPtr(attributes[v1alpha1.TopicSubscriptionPending]),
		SubscriptionsDeleted: awsclient.StrToIntPtr(attributes[v1alpha1.TopicSubscriptionDeleted]),
		EffectiveDeliveryPolicy: aws.String(attributes[v1alpha1.TopicEffectiveDeliveryPolicy]),
	}
	return ob
}

// IsUpToDate returns true if the Topic attributes in AWS
// are same as Topic spec, else returns false
func IsUpToDate(p v1alpha1.TopicParameters, attributes map[string]string, tags []types.Tag) bool{

	if len(p.Tags) != len(tags){
		return false
	}

	for _,v := range tags{
		tagVal, ok := p.Tags[aws.ToString(v.Key)]
		if !ok || !strings.EqualFold(tagVal,aws.ToString(v.Value)){
			return false
		}
	}

	if !strings.EqualFold(aws.ToString(p.Policy),attributes[v1alpha1.TopicPolicy]){
		return false
	}
	if !strings.EqualFold(aws.ToString(p.DisplayName),attributes[v1alpha1.TopicDisplayName]){
		return false
	}
	if !strings.EqualFold(aws.ToString(p.DeliveryPolicy),attributes[v1alpha1.TopicDeliveryPolicy]){
		return false
	}

	if !strings.EqualFold(aws.ToString(p.KMSMasterKeyID),attributes[v1alpha1.TopicKMSMasterKeyID]){
		return false
	}

	b, e := strconv.ParseBool(attributes[v1alpha1.FifoTopic])
	if e != nil || aws.ToBool(p.FifoTopic) != b{
		return false
	}

	b, e = strconv.ParseBool(attributes[v1alpha1.FifoTopicContentBasedDeduplication])
	if e != nil || aws.ToBool(p.ContentBasedDeduplication) != b{
		return false
	}
	return true
}

// GetConnectionDetails returns the Topic Arn which will be included in the secret
func GetConnectionDetails(in v1alpha1.Topic) managed.ConnectionDetails{
	if in.Status.AtProvider.TopicArn == nil{
		return nil
	}
	c := managed.ConnectionDetails{
		xpv1.ResourceCredentialsSecretEndpointKey: []byte(aws.ToString(in.Status.AtProvider.TopicArn)),
	}
	return c
}

// GenerateTopicAttributeMap returns a map of all the topic attributes
func GenerateTopicAttributeMap(in v1alpha1.TopicParameters) map[string]string{

	attributes := make(map[string]string)
	if in.Policy != nil{
		attributes[v1alpha1.TopicPolicy] = aws.ToString(in.Policy)
	}
	if in.FifoTopic != nil{
		attributes[v1alpha1.FifoTopic] = strconv.FormatBool(aws.ToBool(in.FifoTopic))
	}
	if in.DisplayName != nil{
		attributes[v1alpha1.TopicDisplayName] = aws.ToString(in.DisplayName)
	}
	if in.KMSMasterKeyID != nil{
		attributes[v1alpha1.TopicKMSMasterKeyID] = aws.ToString(in.KMSMasterKeyID)
	}
	if in.DeliveryPolicy != nil{
		attributes[v1alpha1.TopicDeliveryPolicy] = aws.ToString(in.DeliveryPolicy)
	}
	if in.ContentBasedDeduplication != nil{
		attributes[v1alpha1.FifoTopicContentBasedDeduplication] = strconv.FormatBool(aws.ToBool(in.ContentBasedDeduplication))
	}
	if len(attributes) == 0{
		return nil
	}
	return attributes
}

// GetAttributeDiff returns the map of Topic attributes which are not
// synced with external resource
func GetAttributeDiff(in v1alpha1.TopicParameters, attributes map[string]string) map[string]string{
	out := make(map[string]string)

	if !strings.EqualFold(aws.ToString(in.Policy),attributes[v1alpha1.TopicPolicy]){
		out[v1alpha1.TopicPolicy] = aws.ToString(in.Policy)
	}
	if aws.ToBool(in.FifoTopic) != aws.ToBool(awsclient.StrToBoolPtr(attributes[v1alpha1.FifoTopic])){
		out[v1alpha1.FifoTopic] = strconv.FormatBool(aws.ToBool(in.FifoTopic))
	}
	if !strings.EqualFold(aws.ToString(in.DisplayName),attributes[v1alpha1.TopicDisplayName]){
		out[v1alpha1.TopicDisplayName] = aws.ToString(in.DisplayName)
	}
	if !strings.EqualFold(aws.ToString(in.KMSMasterKeyID),attributes[v1alpha1.TopicKMSMasterKeyID]){
		out[v1alpha1.TopicKMSMasterKeyID] = aws.ToString(in.KMSMasterKeyID)
	}
	if !strings.EqualFold(aws.ToString(in.DeliveryPolicy),attributes[v1alpha1.TopicDeliveryPolicy]){
		out[v1alpha1.TopicDeliveryPolicy] = aws.ToString(in.DeliveryPolicy)
	}
	if aws.ToBool(in.ContentBasedDeduplication) != aws.ToBool(awsclient.StrToBoolPtr(attributes[v1alpha1.FifoTopicContentBasedDeduplication])){
		out[v1alpha1.FifoTopicContentBasedDeduplication] = strconv.FormatBool(aws.ToBool(in.ContentBasedDeduplication))
	}

	if len(out) == 0{
		return nil
	}
	return out
}

// GetDiffTags returns tags which are required to be added
// or removed from external resource
func GetDiffTags(in v1alpha1.TopicParameters,tags []types.Tag) (addTags []types.Tag, removeTags []types.Tag){

	var managedResourceTags map[string]string
	//Deep copy of managed resource tags
	for k,v := range in.Tags{
		managedResourceTags[k] = v
	}

	// Comparing external resource tags with managed resource tags
	for _,v := range tags{
		t,ok := in.Tags[aws.ToString(v.Key)]
		if !ok{
			removeTags = append(removeTags, v)
		}
		if strings.Compare(t,aws.ToString(v.Value)) != 0{
			removeTags = append(removeTags, v)
			addTags = append(addTags, types.Tag{
				Key: v.Key,
				Value: aws.String(t),
			})
		}
		delete(managedResourceTags,aws.ToString(v.Key))
	}

	// Adding net new tags
	for k,v := range managedResourceTags{
		addTags = append(addTags, types.Tag{
			Key: aws.String(k),
			Value: aws.String(v),
		})
	}

	if len(addTags) == 0{
		addTags = nil
	}
	if len(removeTags) == 0{
		removeTags = nil
	}
	return
}

