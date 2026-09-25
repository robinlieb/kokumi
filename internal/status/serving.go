package status

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deliveryv1alpha1 "github.com/kokumi-dev/kokumi/api/v1alpha1"
)

// ServingUpdater updates the status of a Serving object.
type ServingUpdater struct {
	client client.Client
}

// NewServingUpdater returns a ServingUpdater backed by the given client.
func NewServingUpdater(c client.Client) *ServingUpdater {
	return &ServingUpdater{client: c}
}

// Deploying marks the Serving as actively being deployed. It records the
// preparation that is being served so that consumers can treat it as the
// active preparation while the rollout is in progress.
func (u *ServingUpdater) Deploying(ctx context.Context, serving *deliveryv1alpha1.Serving, preparationName string) error {
	return u.set(ctx, serving, metav1.ConditionUnknown, "Deploying", "Deploying preparation "+preparationName, func(latest *deliveryv1alpha1.Serving) {
		latest.Status.ObservedPreparationName = preparationName
	})
}

// Deployed marks the Serving as successfully deployed.
func (u *ServingUpdater) Deployed(ctx context.Context, serving *deliveryv1alpha1.Serving, preparationName, deployedDigest, msg string) error {
	return SetCondition(ctx, u.client, serving, func(latest *deliveryv1alpha1.Serving) {
		latest.Status.ObservedGeneration = latest.Generation
		latest.Status.ObservedPreparationName = preparationName
		latest.Status.DeployedDigest = deployedDigest
		meta.SetStatusCondition(&latest.Status.Conditions, NewCondition(latest.Generation, metav1.ConditionTrue, "Deployed", msg))
	})
}

// Pending marks the Serving as waiting for a prerequisite.
func (u *ServingUpdater) Pending(ctx context.Context, serving *deliveryv1alpha1.Serving, msg string) error {
	return u.set(ctx, serving, metav1.ConditionUnknown, "Pending", msg, nil)
}

// Failed marks the Serving as failed with the supplied error as the message.
func (u *ServingUpdater) Failed(ctx context.Context, serving *deliveryv1alpha1.Serving, err error) error {
	return u.set(ctx, serving, metav1.ConditionFalse, "DeploymentFailed", err.Error(), nil)
}

// Gate records the target Preparation and whether it passes the approval
// gate. It is a no-op when the status is already up to date.
func (u *ServingUpdater) Gate(ctx context.Context, serving *deliveryv1alpha1.Serving, target string, condStatus metav1.ConditionStatus, reason, msg string) error {
	desired := serving.Status.DeepCopy()
	desired.TargetPreparationName = target
	meta.SetStatusCondition(&desired.Conditions, newTypedCondition(deliveryv1alpha1.ConditionTypeApproved, serving.Generation, condStatus, reason, msg))
	if apiequality.Semantic.DeepEqual(desired, &serving.Status) {
		return nil
	}

	patch := client.MergeFrom(serving.DeepCopy())
	serving.Status = *desired
	return u.client.Status().Patch(ctx, serving, patch)
}

func (u *ServingUpdater) set(ctx context.Context, serving *deliveryv1alpha1.Serving, condStatus metav1.ConditionStatus, reason, msg string, extra func(latest *deliveryv1alpha1.Serving)) error {
	return SetCondition(ctx, u.client, serving, func(latest *deliveryv1alpha1.Serving) {
		latest.Status.ObservedGeneration = latest.Generation
		if extra != nil {
			extra(latest)
		}
		meta.SetStatusCondition(&latest.Status.Conditions, NewCondition(latest.Generation, condStatus, reason, msg))
	})
}
