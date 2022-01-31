package controllers

import (
	"context"
	"fmt"
	"time"

	apiv1beta1 "github.com/kubetrail/serviceaccount-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *TokenReconciler) FinalizeStatus(ctx context.Context, clientObject client.Object) error {
	if !controllerutil.ContainsFinalizer(clientObject, finalizer) {
		return nil
	}

	reqLogger := log.FromContext(ctx)

	object, ok := clientObject.(*apiv1beta1.Token)
	if !ok {
		err := fmt.Errorf("cientObject to object type assertion error")
		reqLogger.Error(err, "failed to get object instance")
		return err
	}

	// Update the status of the object if not terminating
	if object.Status.Phase != phaseTerminating {
		object.Status = apiv1beta1.TokenStatus{
			Phase:      phaseTerminating,
			Conditions: object.Status.Conditions,
			Message:    "object is marked for deletion",
			Reason:     reasonObjectMarkedForDeletion,
		}
		if err := r.Status().Update(ctx, object); err != nil {
			reqLogger.Error(err, "failed to update object status")
			return err
		} else {
			reqLogger.Info("updated object status")
			return ObjectUpdated
		}
	}

	return nil
}

func (r *TokenReconciler) FinalizeResources(ctx context.Context, clientObject client.Object, req ctrl.Request) error {
	if !controllerutil.ContainsFinalizer(clientObject, finalizer) {
		return nil
	}

	return nil
}

func (r *TokenReconciler) RemoveFinalizer(ctx context.Context, clientObject client.Object) error {
	if !controllerutil.ContainsFinalizer(clientObject, finalizer) {
		return nil
	}

	reqLogger := log.FromContext(ctx)

	controllerutil.RemoveFinalizer(clientObject, finalizer)
	if err := r.Update(ctx, clientObject); err != nil {
		reqLogger.Error(err, "failed to remove finalizer")
		return err
	}
	reqLogger.Info("finalizer removed")
	return ObjectUpdated
}

func (r *TokenReconciler) AddFinalizer(ctx context.Context, clientObject client.Object) error {
	if controllerutil.ContainsFinalizer(clientObject, finalizer) {
		return nil
	}

	reqLogger := log.FromContext(ctx)

	controllerutil.AddFinalizer(clientObject, finalizer)
	if err := r.Update(ctx, clientObject); err != nil {
		reqLogger.Error(err, "failed to add finalizer")
		return err
	}
	reqLogger.Info("finalizer added")
	return ObjectUpdated
}

func (r *TokenReconciler) InitializeStatus(ctx context.Context, clientObject client.Object) error {
	reqLogger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(clientObject, finalizer) {
		err := fmt.Errorf("finalizer not found")
		reqLogger.Error(err, "failed to detect finalizer")
		return err
	}

	object, ok := clientObject.(*apiv1beta1.Token)
	if !ok {
		err := fmt.Errorf("cientObject to object type assertion error")
		reqLogger.Error(err, "failed to get object instance")
		return err
	}

	// Update the status of the object if none exists
	found := false
	for _, condition := range object.Status.Conditions {
		if condition.Reason == reasonFinalizerAdded {
			found = true
			break
		}
	}

	if !found {
		object.Status = apiv1beta1.TokenStatus{
			Phase: phasePending,
			Conditions: []v12.Condition{
				{
					Type:               conditionTypeObject,
					Status:             v12.ConditionTrue,
					ObservedGeneration: 0,
					LastTransitionTime: v12.Time{Time: time.Now()},
					Reason:             reasonFinalizerAdded,
					Message:            "object initialized",
				},
			},
			Message: "object initialized",
			Reason:  reasonObjectInitialized,
		}
		if err := r.Status().Update(ctx, object); err != nil {
			reqLogger.Error(err, "failed to update object status")
			return err
		} else {
			reqLogger.Info("updated object status")
			return ObjectUpdated
		}
	}

	return nil
}

func (r *TokenReconciler) ReconcileResources(ctx context.Context, clientObject client.Object, req ctrl.Request) error {
	reqLogger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(clientObject, finalizer) {
		err := fmt.Errorf("finalizer not found")
		reqLogger.Error(err, "failed to detect finalizer")
		return err
	}

	object, ok := clientObject.(*apiv1beta1.Token)
	if !ok {
		err := fmt.Errorf("cientObject to object type assertion error")
		reqLogger.Error(err, "failed to get object instance")
		return err
	}

	var tokenCreated bool
	var found bool

	// try to get secret associated with the service account
	// if secret is found, do nothing
	// if it is not found, create one
	// report errors if not these two cases
	secret := &v1.Secret{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Namespace: req.Namespace,
			Name:      object.Spec.SecretName,
		},
		secret,
	); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(
				ctx,
				&v1.Secret{
					TypeMeta: v12.TypeMeta{},
					ObjectMeta: v12.ObjectMeta{
						Name:                       object.Spec.SecretName,
						GenerateName:               "",
						Namespace:                  object.Namespace,
						SelfLink:                   "",
						UID:                        "",
						ResourceVersion:            "",
						Generation:                 0,
						CreationTimestamp:          v12.Time{Time: time.Now()},
						DeletionTimestamp:          nil,
						DeletionGracePeriodSeconds: nil,
						Labels:                     nil,
						Annotations: map[string]string{
							v1.ServiceAccountNameKey: object.Spec.ServiceAccountName,
						},
						OwnerReferences: []v12.OwnerReference{
							{
								APIVersion:         object.APIVersion,
								Kind:               object.Kind,
								Name:               object.Name,
								UID:                object.UID,
								Controller:         nil,
								BlockOwnerDeletion: nil,
							},
						},
						Finalizers:    nil,
						ClusterName:   "",
						ManagedFields: nil,
					},
					Immutable:  nil,
					Data:       nil,
					StringData: nil,
					Type:       "kubernetes.io/service-account-token",
				},
			); err != nil {
				reqLogger.Error(err, "failed to create secret")
				return err
			}
		} else {
			reqLogger.Error(err, "failed to get secret")
			return err
		}
	} else {
		found = true
	}

	// Update the status of the object if pending
	for i, condition := range object.Status.Conditions {
		if condition.Reason == reasonCreatedToken {
			if tokenCreated {
				object.Status.Conditions[i].LastTransitionTime = v12.Time{Time: time.Now()}
			}
			found = true
			break
		}
	}
	if !found {
		condition := v12.Condition{
			Type:               conditionTypeInfluxdb,
			Status:             v12.ConditionTrue,
			ObservedGeneration: 0,
			LastTransitionTime: v12.Time{Time: time.Now()},
			Reason:             reasonCreatedToken,
			Message:            "created serviceaccount token",
		}
		object.Status = apiv1beta1.TokenStatus{
			Phase:      phaseReady,
			Conditions: append(object.Status.Conditions, condition),
			Message:    "created serviceaccount token",
			Reason:     reasonCreatedToken,
		}
		if err := r.Status().Update(ctx, object); err != nil {
			reqLogger.Error(err, "failed to update object status")
			return err
		} else {
			reqLogger.Info("updated object status")
			return ObjectUpdated
		}
	} else {
		if tokenCreated {
			if err := r.Status().Update(ctx, object); err != nil {
				reqLogger.Error(err, "failed to update object status")
				return err
			} else {
				reqLogger.Info("updated object status")
				return ObjectUpdated
			}
		}
	}

	return nil
}
