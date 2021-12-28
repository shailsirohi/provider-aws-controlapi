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
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	snsv1alpha1 "provider-aws-controlapi/apis/sns/v1alpha1"
	awsclient "provider-aws-controlapi/internal/clients"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	awssnstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"provider-aws-controlapi/internal/clients/sns"
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

	//Check existence of the Topic
	snsAttributes, err := c.client.GetTopicAttributes(ctx,&awssns.GetTopicAttributesInput{
		TopicArn: aws.String(meta.GetExternalName(cr)),
	})

	snsAttributes.Attributes[]


	if err != nil {
		return managed.ExternalObservation{}, awsclient.Wrap(resource.Ignore(sns.IsNotFound, err), errGetTopicAttributesFailed)
	}

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
		ResourceUpToDate: true,

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*snsv1alpha1.Topic)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotTopic)
	}

	fmt.Printf("Creating: %+v", cr)

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*snsv1alpha1.Topic)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotTopic)
	}

	fmt.Printf("Updating: %+v", cr)

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*snsv1alpha1.Topic)
	if !ok {
		return errors.New(errNotTopic)
	}

	fmt.Printf("Deleting: %+v", cr)

	return nil
}
