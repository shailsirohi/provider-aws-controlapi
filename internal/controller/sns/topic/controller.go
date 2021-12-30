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

package topic

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/google/go-cmp/cmp"
	snsv1alpha1 "provider-aws-controlapi/apis/sns/v1alpha1"
	awsclient "provider-aws-controlapi/internal/clients"
	"provider-aws-controlapi/internal/clients/sns"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errNotTopic    				= "managed resource is not a Topic custom resource"
	errKubeUpdateFailed         = "cannot update Topic custom resource"
	errCreateFailed             = "cannot create Topic"
	errDeleteFailed             = "cannot delete Topic"
	errGetTopicAttributesFailed = "cannot get Topic attributes"
	errTag                      = "cannot tag Topic"
	errListTopicTagsFailed      = "cannot list Topic tags"
	errUpdateFailed             = "failed to update the Queue resource"
	errTrackPCUsage 			= "cannot track ProviderConfig usage"
	errGetPC        			= "cannot get ProviderConfig"
	errGetCreds     			= "cannot get credentials"
	errNewClient 				= "cannot create new Service"
)


// SetupTopic adds a controller that reconciles Topic managed resources.
func SetupTopic(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter, poll  time.Duration) error {
	name := managed.ControllerName(snsv1alpha1.TopicGroupKind)

	o := controller.Options{
		RateLimiter: ratelimiter.NewDefaultManagedRateLimiter(rl),
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(snsv1alpha1.TopicGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:        mgr.GetClient(),
			//usage:       resource.NewProviderConfigUsageTracker(mgr.GetClient(), &v1beta1.ProviderConfigUsage{}),
			newClientFn: sns.GetClient}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o).
		For(&snsv1alpha1.Topic{}).
		Complete(r)
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube        client.Client
	//usage       resource.Tracker
	newClientFn func(aws.Config) sns.Client
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*snsv1alpha1.Topic)
	if !ok {
		return nil, errors.New(errNotTopic)
	}
	/*
	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}
	*/

	cfg, err := awsclient.GetConfig(ctx, c.kube, mg, cr.Spec.ForProvider.Region)
	if err != nil {
		return nil, err
	}
	return &external{c.newClientFn(*cfg), c.kube}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client sns.Client
	kube   client.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*snsv1alpha1.Topic)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotTopic)
	}

	if strings.EqualFold(meta.GetExternalName(cr),cr.GetName()){
		return managed.ExternalObservation{
			ResourceExists: false,
			ConnectionDetails: nil,
			ResourceUpToDate: false,
		},nil
	}

	//Check existence of the Topic and if exists, get all sns attributes values
	topicAttributes, err := c.client.GetTopicAttributes(ctx,&awssns.GetTopicAttributesInput{
		TopicArn: aws.String(meta.GetExternalName(cr)),
	})
	if err != nil {
		return managed.ExternalObservation{}, awsclient.Wrap(resource.Ignore(sns.IsNotFound, err), errGetTopicAttributesFailed)
	}

	//Get all the tags on sns topic
	topicTags, err := c.client.ListTagsForResource(ctx,&awssns.ListTagsForResourceInput{
		ResourceArn: aws.String(meta.GetExternalName(cr)),
	})
	if err != nil {
		return managed.ExternalObservation{}, awsclient.Wrap(err,errListTopicTagsFailed)
	}

	current := cr.Spec.ForProvider.DeepCopy()
	// LateInitialize to update tags and topic parameters which are auto generated after topic creation
	sns.LateInitialize(&cr.Spec.ForProvider,topicAttributes.Attributes,topicTags.Tags)
	if !cmp.Equal(current, &cr.Spec.ForProvider){
		err := c.kube.Update(ctx,cr)
		if err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errKubeUpdateFailed)
		}
	}

	cr.Status.SetConditions(xpv1.Available())
	cr.Status.AtProvider = sns.GenerateObservation(topicAttributes.Attributes)

	// These fmt statements should be removed in the real implementation.
	fmt.Printf("Observing: %+v", cr)

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: true,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: sns.IsUpToDate(cr.Spec.ForProvider,topicAttributes.Attributes,topicTags.Tags),

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: sns.GetConnectionDetails(*cr),
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	fmt.Printf("Inside Create function............................")
	cr, ok := mg.(*snsv1alpha1.Topic)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotTopic)
	}

	cr.SetConditions(xpv1.Creating())

	// Check if external name annotation is used or not
	// if not object name is used as topic name
	name := meta.GetExternalName(cr)
	if name == ""{
		name = cr.GetName()
	}

	// Convert Tags map to []types.Tag as required by CreateTopicInput
	t := make([]types.Tag,len(cr.Spec.ForProvider.Tags))
	i := 0
	for k,v := range cr.Spec.ForProvider.Tags{
		t[i] = types.Tag{
			Key: aws.String(k),
			Value: aws.String(v),
		}
		i++
	}

	resp, err := c.client.CreateTopic(ctx,&awssns.CreateTopicInput{
		Attributes: sns.GenerateTopicAttributeMap(cr.Spec.ForProvider),
		Tags: t,
		Name: aws.String(name),
	})

	if err != nil{
		return managed.ExternalCreation{},awsclient.Wrap(err,errCreateFailed)
	}

	// Changing the external name to full TopicArn as
	// AWS APIs doesn't provide any option to get ARN using TopicName
	// Neither do they treat TopicName as identifier
	meta.SetExternalName(cr,*resp.TopicArn)
	conn := managed.ConnectionDetails{
		xpv1.ResourceCredentialsSecretEndpointKey: []byte(*resp.TopicArn),
	}

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: conn,
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	fmt.Printf("Inside Update function............................")
	cr, ok := mg.(*snsv1alpha1.Topic)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotTopic)
	}


	fmt.Printf("Updating: %+v", cr)

	// Check existence of the Topic and if exists, get all sns attributes values
	topicAttributes, err := c.client.GetTopicAttributes(ctx,&awssns.GetTopicAttributesInput{
		TopicArn: aws.String(meta.GetExternalName(cr)),
	})
	if err != nil {
		return managed.ExternalUpdate{}, awsclient.Wrap(resource.Ignore(sns.IsNotFound, err), errGetTopicAttributesFailed)
	}

	// Identifying changed attributes and updating them in external resource
	diffAttributes := sns.GetAttributeDiff(cr.Spec.ForProvider,topicAttributes.Attributes)
	if diffAttributes != nil{
		for k,v := range diffAttributes{
			_, err := c.client.SetTopicAttributes(ctx,&awssns.SetTopicAttributesInput{
				TopicArn: aws.String(meta.GetExternalName(cr)),
				AttributeName: &k,
				AttributeValue: &v,
			})
			if err != nil{
				return managed.ExternalUpdate{},awsclient.Wrap(err,errKubeUpdateFailed)
			}
		}
	}

	// Getting all the tags for the external resource
	topicTags, err := c.client.ListTagsForResource(ctx,&awssns.ListTagsForResourceInput{
		ResourceArn: aws.String(meta.GetExternalName(cr)),
	})
	if err != nil {
		return managed.ExternalUpdate{}, awsclient.Wrap(err,errListTopicTagsFailed)
	}

	// Identifying changes in tags and updating external resource accordingly
	addTags,removeTags := sns.GetDiffTags(cr.Spec.ForProvider,topicTags.Tags)
	if removeTags != nil{
		_, err := c.client.UntagResource(ctx,&awssns.UntagResourceInput{
			ResourceArn: aws.String(meta.GetExternalName(cr)),
			TagKeys: removeTags,
		})
		if err != nil{
			return managed.ExternalUpdate{},awsclient.Wrap(err,errKubeUpdateFailed)
		}
	}
	if addTags != nil{
		_, err := c.client.TagResource(ctx,&awssns.TagResourceInput{
			ResourceArn: aws.String(meta.GetExternalName(cr)),
			Tags: addTags,
		})
		if err != nil{
			return managed.ExternalUpdate{},awsclient.Wrap(err,errKubeUpdateFailed)
		}
	}

	conn := managed.ConnectionDetails{
		xpv1.ResourceCredentialsSecretEndpointKey: []byte(meta.GetExternalName(cr)),
	}

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: conn,
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*snsv1alpha1.Topic)
	if !ok {
		return errors.New(errNotTopic)
	}

	fmt.Printf("Deleting: %+v", cr)

	cr.SetConditions(xpv1.Deleting())

	_, err := c.client.DeleteTopic(ctx,&awssns.DeleteTopicInput{
		TopicArn: aws.String(meta.GetExternalName(cr)),
	})

	if err != nil{
		return awsclient.Wrap(resource.Ignore(sns.IsNotFound,err),errDeleteFailed)
	}

	return nil
}
