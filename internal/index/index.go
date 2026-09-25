// Package index registers the field indexes shared by the kokumi controllers
// and provides typed list helpers on top of them.
package index

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	deliveryv1alpha1 "github.com/kokumi-dev/kokumi/api/v1alpha1"
)

// Setup registers all field indexes with the manager's cache.
func Setup(ctx context.Context, indexer client.FieldIndexer) error {
	indexes := []struct {
		obj     client.Object
		field   string
		extract client.IndexerFunc
	}{
		{&deliveryv1alpha1.Approval{}, deliveryv1alpha1.FieldPreparationRefName, approvalPreparation},
		{&deliveryv1alpha1.Preparation{}, deliveryv1alpha1.FieldOrderName, preparationOrder},
		{&deliveryv1alpha1.Order{}, deliveryv1alpha1.FieldSourcePantryRefName, orderSourcePantry},
		{&deliveryv1alpha1.Order{}, deliveryv1alpha1.FieldDestinationPantryRefName, orderDestinationPantry},
	}
	for _, idx := range indexes {
		if err := indexer.IndexField(ctx, idx.obj, idx.field, idx.extract); err != nil {
			return err
		}
	}
	return nil
}

// ApprovalsForPreparation returns the Approvals referencing prep by name.
func ApprovalsForPreparation(ctx context.Context, r client.Reader, prep *deliveryv1alpha1.Preparation) ([]deliveryv1alpha1.Approval, error) {
	list := &deliveryv1alpha1.ApprovalList{}
	if err := r.List(ctx, list,
		client.InNamespace(prep.Namespace),
		client.MatchingFields{deliveryv1alpha1.FieldPreparationRefName: prep.Name},
	); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// PreparationsForOrder returns the Preparations of the named Order.
func PreparationsForOrder(ctx context.Context, r client.Reader, namespace, orderName string) ([]deliveryv1alpha1.Preparation, error) {
	list := &deliveryv1alpha1.PreparationList{}
	if err := r.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingFields{deliveryv1alpha1.FieldOrderName: orderName},
	); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// OrdersForPantry returns the Orders whose source or destination references
// the named Pantry. It requires the cache indexes registered by Setup.
func OrdersForPantry(ctx context.Context, r client.Reader, namespace, pantryName string) ([]deliveryv1alpha1.Order, error) {
	var orders []deliveryv1alpha1.Order
	seen := map[string]struct{}{}
	for _, field := range []string{deliveryv1alpha1.FieldSourcePantryRefName, deliveryv1alpha1.FieldDestinationPantryRefName} {
		list := &deliveryv1alpha1.OrderList{}
		if err := r.List(ctx, list, client.InNamespace(namespace), client.MatchingFields{field: pantryName}); err != nil {
			return nil, err
		}
		for _, o := range list.Items {
			if _, dup := seen[o.Name]; dup {
				continue
			}
			seen[o.Name] = struct{}{}
			orders = append(orders, o)
		}
	}
	return orders, nil
}

func approvalPreparation(obj client.Object) []string {
	a, ok := obj.(*deliveryv1alpha1.Approval)
	if !ok {
		return nil
	}
	return []string{a.Spec.PreparationRef.Name}
}

func preparationOrder(obj client.Object) []string {
	p, ok := obj.(*deliveryv1alpha1.Preparation)
	if !ok {
		return nil
	}
	return []string{p.Spec.OrderName}
}

func orderSourcePantry(obj client.Object) []string {
	o, ok := obj.(*deliveryv1alpha1.Order)
	if !ok || o.Spec.Source == nil || o.Spec.Source.PantryRef == nil {
		return nil
	}
	return []string{o.Spec.Source.PantryRef.Name}
}

func orderDestinationPantry(obj client.Object) []string {
	o, ok := obj.(*deliveryv1alpha1.Order)
	if !ok || o.Spec.Destination == nil || o.Spec.Destination.PantryRef == nil {
		return nil
	}
	return []string{o.Spec.Destination.PantryRef.Name}
}
