package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

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

	secrets := &v1.SecretList{}
	if err := r.List(ctx, secrets, client.InNamespace(object.Namespace)); err != nil {
		reqLogger.Error(err, "failed to list secrets")
		return err
	}

	// scan through all secrets, find the ones for which owner reference matches, then
	// delete those for which time has expired
	for _, secret := range secrets.Items {
		secret := secret
		for _, ownerReference := range secret.OwnerReferences {
			if ownerReference.UID == object.UID &&
				object.Spec.RotationPeriodSeconds != nil &&
				object.Spec.DeletionGracePeriodSeconds != nil {
				if time.Since(
					secret.CreationTimestamp.Time.Add(
						time.Second*time.Duration(
							(*object.Spec.RotationPeriodSeconds)+(*object.Spec.DeletionGracePeriodSeconds),
						),
					),
				) > 0 {
					if err := r.Delete(ctx, &secret); err != nil {
						reqLogger.Error(err, "failed to delete secret", "name", secret.Name)
						return err
					} else {
						reqLogger.Info("deleted secret", "name", secret.Name)
					}
				}
			}
		}
	}

	id := uuid.New().String()
	secretName := fmt.Sprintf("%s-%s-%s", object.Name, "token", id[:5])
	createSecret := func() error {
		var deletionTimestamp *v12.Time
		if object.Spec.RotationPeriodSeconds != nil {
			deletionTimestamp = &v12.Time{
				Time: time.Now().Add(
					time.Second * time.Duration(*object.Spec.RotationPeriodSeconds),
				),
			}
		}
		return r.Create(
			ctx,
			&v1.Secret{
				TypeMeta: v12.TypeMeta{},
				ObjectMeta: v12.ObjectMeta{
					Name:                       secretName,
					GenerateName:               "",
					Namespace:                  object.Namespace,
					SelfLink:                   "",
					UID:                        "",
					ResourceVersion:            "",
					Generation:                 0,
					CreationTimestamp:          v12.Time{Time: time.Now()},
					DeletionTimestamp:          deletionTimestamp,
					DeletionGracePeriodSeconds: object.Spec.DeletionGracePeriodSeconds,
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
		)
	}

	var tokenCreated bool
	// try to get secret associated with the service account
	// if secret is found, do nothing
	// if it is not found, create one
	// report errors if not these two cases
	secret := &v1.Secret{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Namespace: req.Namespace,
			Name:      object.Status.SecretName,
		},
		secret,
	); err != nil {
		if errors.IsNotFound(err) {
			if err := createSecret(); err != nil {
				reqLogger.Error(err, "failed to create secret")
				return err
			}
			tokenCreated = true
		} else {
			reqLogger.Error(err, "failed to get secret")
			return err
		}
	} else {
		if object.Spec.RotationPeriodSeconds != nil {
			if time.Since(
				secret.CreationTimestamp.Time.Add(
					time.Second*time.Duration(*object.Spec.RotationPeriodSeconds),
				),
			) > 0 {
				if err := createSecret(); err != nil {
					reqLogger.Error(err, "failed to create secret")
					return err
				}
				tokenCreated = true
				if object.Spec.DeletionGracePeriodSeconds == nil {
					if err := r.Delete(ctx, secret); err != nil {
						reqLogger.Error(err, "failed to delete secret")
						return err
					}
				}
			}
		}
	}

	if tokenCreated {
		condition := v12.Condition{
			Type:               conditionTypeInfluxdb,
			Status:             v12.ConditionTrue,
			ObservedGeneration: 0,
			LastTransitionTime: v12.Time{Time: time.Now()},
			Reason:             reasonCreatedToken,
			Message:            "created serviceaccount token",
		}

		conditions := object.Status.Conditions

		index := -1
		for i, condition := range object.Status.Conditions {
			if condition.Type == conditionTypeInfluxdb &&
				condition.Status == v12.ConditionTrue &&
				condition.Reason == reasonCreatedToken {
				index = i
				break
			}
		}

		if index >= 0 {
			conditions[index] = condition
		} else {
			conditions = append(conditions, condition)
		}

		object.Status = apiv1beta1.TokenStatus{
			Phase:      phaseReady,
			Conditions: conditions,
			Message:    "created serviceaccount token",
			Reason:     reasonCreatedToken,
			SecretName: secretName,
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
