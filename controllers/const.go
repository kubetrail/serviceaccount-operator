package controllers

const (
	finalizer                     = "serviceaccount.kubetrail.io/finalizer"
	reasonObjectInitialized       = "objectInitialized"
	reasonObjectMarkedForDeletion = "objectMarkedForDeletion"
	reasonFinalizerAdded          = "finalizerAdded"
	reasonCreatedToken            = "createdToken"
	reasonDeletedToken            = "deletedToken"
	phasePending                  = "pending"
	phaseReady                    = "ready"
	phaseTerminating              = "terminating"
	conditionTypeObject           = "object"
	conditionTypeInfluxdb         = "influxdb"
)
