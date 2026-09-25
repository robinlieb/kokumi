/*
Copyright 2026.

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

package v1alpha1

// Field paths used with field selectors and cache field indexes. The paths
// that are also CRD selectable fields can be used against the API server.
const (
	// FieldOrderName selects Approvals and Preparations by spec.orderName.
	FieldOrderName = "spec.orderName"

	// FieldPreparationRefName selects Approvals by spec.preparationRef.name.
	FieldPreparationRefName = "spec.preparationRef.name"

	// FieldSourcePantryRefName indexes Orders by spec.source.pantryRef.name (cache only).
	FieldSourcePantryRefName = "spec.source.pantryRef.name"

	// FieldDestinationPantryRefName indexes Orders by spec.destination.pantryRef.name (cache only).
	FieldDestinationPantryRefName = "spec.destination.pantryRef.name"
)
