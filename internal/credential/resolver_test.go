package credential

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deliveryv1alpha1 "github.com/kokumi-dev/kokumi/api/v1alpha1"
)

const testNamespace = "team-a"

func testResolver(t *testing.T, objs ...runtime.Object) *KubeResolver {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, deliveryv1alpha1.AddToScheme(scheme))
	return NewKubeResolver(fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build())
}

func testOrder(name string, dest *deliveryv1alpha1.OCIDestination) *deliveryv1alpha1.Order {
	return &deliveryv1alpha1.Order{
		Name:      name,
		Namespace: testNamespace,
		Spec:      deliveryv1alpha1.OrderSpec{Destination: dest},
	}
}

func TestDestinationClient(t *testing.T) {
	ctx := context.Background()
	secret := &corev1.Secret{
		Name:      "registry-creds",
		Namespace: testNamespace,
		Data:      map[string][]byte{".dockerconfigjson": []byte(`{"auths":{"registry.example":{"auth":"dXNlcjpwYXNz"}}}`)},
	}
	pantry := &deliveryv1alpha1.Pantry{
		Name:      "private",
		Namespace: testNamespace,
		Spec: deliveryv1alpha1.PantrySpec{
			URL:       "oci://registry.example/team-a",
			SecretRef: &corev1.LocalObjectReference{Name: "registry-creds"},
		},
	}
	withPantry := &deliveryv1alpha1.OCIDestination{PantryRef: &deliveryv1alpha1.PantryRef{Name: "private"}}

	r := testResolver(t,
		secret, pantry,
		testOrder("default-dest", nil),
		testOrder("pantry-dest", withPantry),
		testOrder("missing-pantry", &deliveryv1alpha1.OCIDestination{PantryRef: &deliveryv1alpha1.PantryRef{Name: "nope"}}),
	)

	c, err := r.DestinationClient(ctx, testNamespace, "gone")
	require.NoError(t, err)
	assert.Nil(t, c, "a deleted Order falls back to the default client")

	c, err = r.DestinationClient(ctx, testNamespace, "default-dest")
	require.NoError(t, err)
	assert.Nil(t, c)

	c, err = r.DestinationClient(ctx, testNamespace, "pantry-dest")
	require.NoError(t, err)
	assert.NotNil(t, c, "a Pantry with credentials yields an authenticated client")

	_, err = r.DestinationClient(ctx, testNamespace, "missing-pantry")
	require.Error(t, err)
}
