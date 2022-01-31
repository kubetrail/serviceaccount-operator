/*
Copyright 2022 kubetrail.io authors.

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
	"context"
	"errors"
	"time"

	apiv1beta1 "github.com/kubetrail/serviceaccount-operator/api/v1beta1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TokenReconciler reconciles a Token object
type TokenReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=token.kubetrail.io,resources=tokens,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=token.kubetrail.io,resources=tokens/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=token.kubetrail.io,resources=tokens/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Token object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *TokenReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)

	object := &apiv1beta1.Token{}
	if err := r.Get(ctx, req.NamespacedName, object); err != nil {
		if apimachineryerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("object not found")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "failed to get object")
		return ctrl.Result{}, err
	}

	// Check if the Object instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if object.GetDeletionTimestamp() != nil {
		if err := r.FinalizeStatus(ctx, object); err != nil {
			if errors.Is(err, ObjectUpdated) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		if err := r.FinalizeResources(ctx, object, req); err != nil {
			if errors.Is(err, ObjectUpdated) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		if err := r.RemoveFinalizer(ctx, object); err != nil {
			if errors.Is(err, ObjectUpdated) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR and update the object.
	if err := r.AddFinalizer(ctx, object); err != nil {
		if errors.Is(err, ObjectUpdated) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.InitializeStatus(ctx, object); err != nil {
		if errors.Is(err, ObjectUpdated) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.ReconcileResources(ctx, object, req); err != nil {
		if errors.Is(err, ObjectUpdated) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// requeue to maintain the state
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: time.Minute,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TokenReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1beta1.Token{}).
		Complete(r)
}
