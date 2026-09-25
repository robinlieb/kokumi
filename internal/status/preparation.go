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

// PreparationUpdater updates the status of a Preparation object.
type PreparationUpdater struct {
	client client.Client
}

// NewPreparationUpdater returns a PreparationUpdater backed by the given client.
func NewPreparationUpdater(c client.Client) *PreparationUpdater {
	return &PreparationUpdater{client: c}
}

// Ready marks the Preparation as ready for serving.
func (u *PreparationUpdater) Ready(ctx context.Context, preparation *deliveryv1alpha1.Preparation, msg string) error {
	return u.set(ctx, preparation, metav1.ConditionTrue, "Ready", msg)
}

// Failed marks the Preparation as failed.
func (u *PreparationUpdater) Failed(ctx context.Context, preparation *deliveryv1alpha1.Preparation, err error) error {
	return u.set(ctx, preparation, metav1.ConditionFalse, "ProcessingFailed", err.Error())
}

// Pending marks the Preparation as pending.
func (u *PreparationUpdater) Pending(ctx context.Context, preparation *deliveryv1alpha1.Preparation, msg string) error {
	return u.set(ctx, preparation, metav1.ConditionUnknown, "Pending", msg)
}

func (u *PreparationUpdater) set(ctx context.Context, preparation *deliveryv1alpha1.Preparation, condStatus metav1.ConditionStatus, reason, msg string) error {
	return SetCondition(ctx, u.client, preparation, func(latest *deliveryv1alpha1.Preparation) {
		meta.SetStatusCondition(&latest.Status.Conditions, NewCondition(latest.Generation, condStatus, reason, msg))
	})
}

// Approval writes the aggregated approval status and the Approved condition.
func (u *PreparationUpdater) Approval(ctx context.Context, preparation *deliveryv1alpha1.Preparation, res approval.Result) error {
	return u.patch(ctx, preparation, func(s *deliveryv1alpha1.PreparationStatus) {
		s.Approval = nil
		if res.Required {
			s.Approval = res.Status.DeepCopy()
		}
		condStatus := metav1.ConditionFalse
		if res.Approved {
			condStatus = metav1.ConditionTrue
		}
		meta.SetStatusCondition(&s.Conditions, newTypedCondition(deliveryv1alpha1.ConditionTypeApproved, preparation.Generation, condStatus, res.Reason, res.Message))
	})
}

// Seal locks the votes in res. The OCI attestation is recorded by Archived.
func (u *PreparationUpdater) Seal(ctx context.Context, preparation *deliveryv1alpha1.Preparation, res approval.Result, sealedTime metav1.Time) error {
	return u.patch(ctx, preparation, func(s *deliveryv1alpha1.PreparationStatus) {
		s.Approval = res.Status.DeepCopy()
		s.Approval.SealedTime = &sealedTime
		s.Approval.Attestation = nil
		meta.SetStatusCondition(&s.Conditions, newTypedCondition(deliveryv1alpha1.ConditionTypeApprovalsSealed, preparation.Generation,
			metav1.ConditionFalse, deliveryv1alpha1.ReasonArchiving, "Recording the sealed approvals in the OCI registry"))
	})
}

// Archived records the OCI attestation of the sealed votes.
func (u *PreparationUpdater) Archived(ctx context.Context, preparation *deliveryv1alpha1.Preparation, attestation deliveryv1alpha1.ApprovalAttestation) error {
	return u.patch(ctx, preparation, func(s *deliveryv1alpha1.PreparationStatus) {
		s.Approval.Attestation = &attestation
		meta.SetStatusCondition(&s.Conditions, newTypedCondition(deliveryv1alpha1.ConditionTypeApprovalsSealed, preparation.Generation,
			metav1.ConditionTrue, deliveryv1alpha1.ReasonSealed, "Approvals sealed and recorded at "+attestation.OCIRef))
	})
}

// ArchiveFailed records that the OCI attestation could not be pushed.
func (u *PreparationUpdater) ArchiveFailed(ctx context.Context, preparation *deliveryv1alpha1.Preparation, err error) error {
	return u.patch(ctx, preparation, func(s *deliveryv1alpha1.PreparationStatus) {
		meta.SetStatusCondition(&s.Conditions, newTypedCondition(deliveryv1alpha1.ConditionTypeApprovalsSealed, preparation.Generation,
			metav1.ConditionFalse, deliveryv1alpha1.ReasonArchiveFailed, err.Error()))
	})
}

// patch applies mutate to a copy of the status and patches it with an
// optimistic lock, so a stale cache read can never overwrite sealed votes.
func (u *PreparationUpdater) patch(ctx context.Context, preparation *deliveryv1alpha1.Preparation, mutate func(*deliveryv1alpha1.PreparationStatus)) error {
	desired := preparation.Status.DeepCopy()
	desired.ObservedGeneration = preparation.Generation
	mutate(desired)
	if apiequality.Semantic.DeepEqual(desired, &preparation.Status) {
		return nil
	}

	patch := client.MergeFromWithOptions(preparation.DeepCopy(), client.MergeFromWithOptimisticLock{})
	preparation.Status = *desired
	return u.client.Status().Patch(ctx, preparation, patch)
}
