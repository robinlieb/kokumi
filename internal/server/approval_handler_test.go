package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deliveryv1alpha1 "github.com/kokumi-dev/kokumi/api/v1alpha1"
)

const (
	approvalTestDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	testApproverGroup  = "release"
	testApprover       = "alice"
	testIssuerURL      = "https://issuer"
	approveBody        = `{"decision":"Approve"}`
)

func approvalTestPreparation() *deliveryv1alpha1.Preparation {
	return &deliveryv1alpha1.Preparation{
		Name:      "shop-1",
		Namespace: "team-a",
		UID:       "prep-uid",
		Spec: deliveryv1alpha1.PreparationSpec{
			OrderName:      "shop",
			Artifact:       deliveryv1alpha1.Artifact{Digest: approvalTestDigest},
			ApprovalPolicy: &deliveryv1alpha1.ApprovalPolicy{RequiredApprovals: 1, AllowedGroups: []string{testApproverGroup}},
		},
	}
}

func submitApprovalRequest(t *testing.T, deps *apiDeps, id *Identity, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/preparations/team-a/shop-1/approvals", strings.NewReader(body))
	req.SetPathValue("namespace", "team-a")
	req.SetPathValue("name", "shop-1")
	if id != nil {
		req = req.WithContext(withIdentity(req.Context(), id))
	}
	rec := httptest.NewRecorder()
	handleSubmitApproval(deps)(rec, req)
	return rec
}

func TestHandleSubmitApprovalRequiresServerIdentity(t *testing.T) {
	deps := &apiDeps{logger: logr.Discard()}
	rec := submitApprovalRequest(t, deps, &Identity{Subject: testApprover, Provider: providerOIDC, Issuer: testIssuerURL}, approveBody)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "without the server's own client no Approval can be submitted")
}

func TestHandleSubmitApprovalRejectsInvalidRequests(t *testing.T) {
	oidc := &Identity{Subject: testApprover, Provider: providerOIDC, Issuer: testIssuerURL}
	tests := []struct {
		name string
		id   *Identity
		body string
		want int
	}{
		{name: "unauthenticated", id: nil, body: approveBody, want: http.StatusUnauthorized},
		{name: "shared admin account", id: &Identity{Subject: "admin", Provider: providerAdmin}, body: approveBody, want: http.StatusForbidden},
		{name: "oidc without issuer", id: &Identity{Subject: testApprover, Provider: providerOIDC}, body: approveBody, want: http.StatusForbidden},
		{name: "unknown decision", id: oidc, body: `{"decision":"Maybe"}`, want: http.StatusBadRequest},
		{name: "comment too long", id: oidc, body: `{"decision":"Approve","comment":"` + strings.Repeat("x", maxApprovalComment+1) + `"}`, want: http.StatusBadRequest},
		{name: "malformed body", id: oidc, body: `{`, want: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &apiDeps{logger: logr.Discard(), approvalWriter: fake.NewClientBuilder().WithScheme(newScheme()).Build()}
			rec := submitApprovalRequest(t, deps, tt.id, tt.body)
			assert.Equal(t, tt.want, rec.Code, rec.Body.String())
		})
	}
}

func TestNewApprovalTakesIdentityFromSession(t *testing.T) {
	prep := approvalTestPreparation()
	id := &Identity{
		Subject:  "alice-sub",
		Issuer:   testIssuerURL,
		Username: testApprover,
		Email:    "alice@example.com",
		Groups:   []string{"platform", testApproverGroup, testApproverGroup},
		Provider: providerOIDC,
	}

	a := newApproval(prep, nil, id, deliveryv1alpha1.ApprovalDecisionApprove, "lgtm")

	assert.Regexp(t, `^shop-1-[0-9a-f]{10}-0$`, a.Name)
	assert.Equal(t, "team-a", a.Namespace)
	assert.Equal(t, "shop", a.Spec.OrderName)
	assert.Equal(t, deliveryv1alpha1.ApprovalPreparationReference{Name: "shop-1", UID: "prep-uid", ArtifactDigest: approvalTestDigest}, a.Spec.PreparationRef)
	assert.Equal(t, deliveryv1alpha1.Approver{
		Issuer:   testIssuerURL,
		Subject:  "alice-sub",
		Username: testApprover,
		Email:    "alice@example.com",
		Groups:   []string{testApproverGroup},
	}, a.Spec.Approver, "only policy-relevant groups are recorded")
	assert.False(t, a.Spec.SubmittedTime.IsZero())
}

func TestRejectVote(t *testing.T) {
	prep := approvalTestPreparation()
	id := &Identity{Subject: testApprover, Issuer: testIssuerURL, Groups: []string{testApproverGroup}, Provider: providerOIDC}
	existing := newApproval(prep, nil, id, deliveryv1alpha1.ApprovalDecisionApprove, "")

	assert.Contains(t, rejectVote(prep, []deliveryv1alpha1.Approval{*existing}, id, deliveryv1alpha1.ApprovalDecisionApprove), "already Approve")
	assert.Empty(t, rejectVote(prep, []deliveryv1alpha1.Approval{*existing}, id, deliveryv1alpha1.ApprovalDecisionReject))

	other := &Identity{Subject: "bob", Issuer: testIssuerURL, Provider: providerOIDC}
	assert.Empty(t, rejectVote(prep, []deliveryv1alpha1.Approval{*existing}, other, deliveryv1alpha1.ApprovalDecisionApprove))
}

func TestNewApprovalTruncatesLongNames(t *testing.T) {
	prep := approvalTestPreparation()
	prep.Name = strings.Repeat("a", 250)
	a := newApproval(prep, nil, &Identity{Subject: testApprover, Issuer: testIssuerURL}, deliveryv1alpha1.ApprovalDecisionApprove, "")
	assert.LessOrEqual(t, len(a.Name), 253)
}

func TestNewApprovalNameIsUniquePerApproverVote(t *testing.T) {
	prep := approvalTestPreparation()
	alice := &Identity{Subject: testApprover, Issuer: testIssuerURL}
	bob := &Identity{Subject: "bob", Issuer: testIssuerURL}

	first := newApproval(prep, nil, alice, deliveryv1alpha1.ApprovalDecisionApprove, "")
	race := newApproval(prep, nil, alice, deliveryv1alpha1.ApprovalDecisionApprove, "")
	assert.Equal(t, first.Name, race.Name, "concurrent votes of one approver must compete for the same name")

	second := newApproval(prep, []deliveryv1alpha1.Approval{*first}, alice, deliveryv1alpha1.ApprovalDecisionReject, "")
	assert.NotEqual(t, first.Name, second.Name, "a later vote gets the next sequence number")

	other := newApproval(prep, []deliveryv1alpha1.Approval{*first}, bob, deliveryv1alpha1.ApprovalDecisionApprove, "")
	assert.NotEqual(t, first.Name, other.Name)
	assert.True(t, strings.HasSuffix(other.Name, "-0"), "votes of other approvers do not advance the sequence")
}

func TestHandleSubmitApprovalRejectsConcurrentDuplicate(t *testing.T) {
	prep := approvalTestPreparation()
	alice := &Identity{Subject: testApprover, Issuer: testIssuerURL}
	winner := newApproval(prep, nil, alice, deliveryv1alpha1.ApprovalDecisionApprove, "")
	writer := fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(winner).Build()

	loser := newApproval(prep, nil, alice, deliveryv1alpha1.ApprovalDecisionApprove, "")
	err := writer.Create(t.Context(), loser)
	require.Error(t, err)
	assert.True(t, apierrors.IsAlreadyExists(err), "the API server admits only one of two racing votes")
}
