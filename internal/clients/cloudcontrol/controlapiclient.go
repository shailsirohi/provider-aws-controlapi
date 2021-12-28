package cloudcontrol

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
)
type Client interface {
	CreateResource(ctx context.Context, params *cloudcontrol.CreateResourceInput, optFns ...func(*cloudcontrol.Options)) (*cloudcontrol.CreateResourceOutput, error)
	GetResource(ctx context.Context, params *cloudcontrol.GetResourceInput, optFns ...func(*cloudcontrol.Options)) (*cloudcontrol.GetResourceOutput, error)
	UpdateResource(ctx context.Context, params *cloudcontrol.UpdateResourceInput, optFns ...func(*cloudcontrol.Options)) (*cloudcontrol.UpdateResourceOutput, error)
	ListResources(ctx context.Context, params *cloudcontrol.ListResourcesInput, optFns ...func(*cloudcontrol.Options)) (*cloudcontrol.ListResourcesOutput, error)
	DeleteResource(ctx context.Context, params *cloudcontrol.DeleteResourceInput, optFns ...func(*cloudcontrol.Options)) (*cloudcontrol.DeleteResourceOutput, error)
	GetResourceRequestStatus(ctx context.Context, params *cloudcontrol.GetResourceRequestStatusInput, optFns ...func(*cloudcontrol.Options)) (*cloudcontrol.GetResourceRequestStatusOutput, error)
	ListResourceRequests(ctx context.Context, params *cloudcontrol.ListResourceRequestsInput, optFns ...func(*cloudcontrol.Options)) (*cloudcontrol.ListResourceRequestsOutput, error)
	CancelResourceRequest(ctx context.Context, params *cloudcontrol.CancelResourceRequestInput, optFns ...func(*cloudcontrol.Options)) (*cloudcontrol.CancelResourceRequestOutput, error)
}

func GetClient(c aws.Config) Client {
	client := cloudcontrol.NewFromConfig(c)
	return client
}