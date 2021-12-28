package sns

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/smithy-go"
	"provider-aws-controlapi/apis/sns/v1alpha1"
)

const (
	//Error code send by AWS API if topic doesn't exist
	TopicNotFound = "NotFound"
)

type Client interface {
	CreateTopic(ctx context.Context, params *awssns.CreateTopicInput, optFns ...func(*awssns.Options)) (*awssns.CreateTopicOutput, error)
	DeleteTopic(ctx context.Context, params *awssns.DeleteTopicInput, optFns ...func(*awssns.Options)) (*awssns.DeleteTopicOutput, error)
	GetTopicAttributes(ctx context.Context, params *awssns.GetTopicAttributesInput, optFns ...func(*awssns.Options)) (*awssns.GetTopicAttributesOutput, error)
	TagResource(ctx context.Context, params *awssns.TagResourceInput, optFns ...func(*awssns.Options)) (*awssns.TagResourceOutput, error)
	UntagResource(ctx context.Context, params *awssns.UntagResourceInput, optFns ...func(*awssns.Options)) (*awssns.UntagResourceOutput, error)
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
func LateInitialize(in *v1alpha1.TopicParameters,attributes map[string]string, tags map[string]string){
	if in.Tags == nil && len(tags) > 0 {
		in.Tags = map[string]string{}
		for k, v := range tags {
			in.Tags[k] = v
		}
	}

}

