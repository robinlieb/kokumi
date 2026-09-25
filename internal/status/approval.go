package status

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deliveryv1alpha1 "github.com/kokumi-dev/kokumi/api/v1alpha1"
	"github.com/kokumi-dev/kokumi/internal/approval"
)

// ApprovalUpdater updates the status of an Approval object.
type ApprovalUpdater struct {
	client client.Client
}

// NewApprovalUpdater returns an ApprovalUpdater backed by the given client.
func NewApprovalUpdater(c client.Client) *ApprovalUpdater {
	return &ApprovalUpdater{client: c}
}

// Counted records the verdict of the approval gate for the Approval. It is a
// no-op when the status is already up to date.
func (u *ApprovalUpdater) Counted(ctx context.Context, a *deliveryv1alpha1.Approval, v approval.Verdict) error {
	condStatus := metav1.ConditionFalse
	if v.Counted {
		condStatus = metav1.ConditionTrue
	}
	desired := a.Status.DeepCopy()
	desired.ObservedGeneration = a.Generation
	meta.SetStatusCondition(&desired.Conditions, metav1.Condition{
		Type:               deliveryv1alpha1.ConditionTypeCounted,
		Status:             condStatus,
		Reason:             v.Reason,
		Message:            v.Message,
		ObservedGeneration: a.Generation,
	})
	if apiequality.Semantic.DeepEqual(desired, &a.Status) {
		return nil
	}

	patch := client.MergeFromWithOptions(a.DeepCopy(), client.MergeFromWithOptimisticLock{})
	a.Status = *desired
	return u.client.Status().Patch(ctx, a, patch)
}
