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

package controller

import (
	"k8s.io/client-go/util/workqueue"
	"provider-aws-controlapi/internal/controller/config"
	"provider-aws-controlapi/internal/controller/sns/topic"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Setup creates all Template controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, l logging.Logger, wl workqueue.RateLimiter, poll time.Duration) error {
	for _, setup := range []func(ctrl.Manager, logging.Logger, workqueue.RateLimiter, time.Duration) error{
		config.Setup,
		topic.SetupTopic,
	} {
		if err := setup(mgr, l, wl,poll); err != nil {
			return err
		}
	}
	return nil
}
