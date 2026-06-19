package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	libutils "github.com/Diaphteiros/kw/pluginlib/pkg/utils"
)

type Selector[T any] interface {
	// Matches returns true if the selector matches the given object.
	Matches(obj T) bool
	// IsEmpty returns true if the selector can be considered empty.
	// This usually means that it matches all objects, but it can be implemented differently depending on the selector type.
	IsEmpty() bool
}

type ObjectIdentitySelector struct {
	// Name specifies the name of the object to match.
	// If empty, matches all objects.
	Name string `json:"name,omitempty"`
}

var _ Selector[client.Object] = &ObjectIdentitySelector{}

// IsEmpty implements [Selector].
func (o *ObjectIdentitySelector) IsEmpty() bool {
	return o == nil || o.Name == ""
}

// Matches implements [Selector].
func (o *ObjectIdentitySelector) Matches(obj client.Object) bool {
	if o.IsEmpty() {
		return true
	}
	return o.Name == obj.GetName()
}

type ObjectLabelSelector metav1.LabelSelector

var _ Selector[client.Object] = &ObjectLabelSelector{}

// IsEmpty implements [Selector].
func (o *ObjectLabelSelector) IsEmpty() bool {
	return o == nil || (len(o.MatchLabels) == 0 && len(o.MatchExpressions) == 0)
}

// Matches implements [Selector].
func (o *ObjectLabelSelector) Matches(obj client.Object) bool {
	if o.IsEmpty() {
		return true
	}
	selector, err := metav1.LabelSelectorAsSelector((*metav1.LabelSelector)(o))
	if err != nil {
		libutils.Fatal(1, "invalid label selector: %w", err)
	}
	return selector.Matches(labels.Set(obj.GetLabels()))
}

type ObjectIdentitiesSelector struct {
	// Names specifies the names of the objects to match.
	// If empty, matches all objects.
	Names []string `json:"names,omitempty"`
}

var _ Selector[client.Object] = &ObjectIdentitiesSelector{}

// IsEmpty implements [Selector].
func (o *ObjectIdentitiesSelector) IsEmpty() bool {
	if o == nil || len(o.Names) == 0 {
		return true
	}
	for _, name := range o.Names {
		if name != "" {
			return false
		}
	}
	return true
}

// Matches implements [Selector].
func (o *ObjectIdentitiesSelector) Matches(obj client.Object) bool {
	if o.IsEmpty() {
		return true
	}
	for _, name := range o.Names {
		if name == obj.GetName() {
			return true
		}
	}
	return false
}

// ObjectIdentityLabelSelector is a selector that combines identity and label selectors.
// If multiple selectors are specified, an object must match all of them to be selected.
// If the selector is nil or all of its sub-selectors are empty, it is considered empty and matches all objects.
type ObjectIdentityLabelSelector struct {
	*ObjectIdentitySelector
	*ObjectIdentitiesSelector
	*ObjectLabelSelector
}

var _ Selector[client.Object] = &ObjectIdentityLabelSelector{}

// IsEmpty implements [Selector].
func (o *ObjectIdentityLabelSelector) IsEmpty() bool {
	return o == nil || (o.ObjectIdentitySelector.IsEmpty() && o.ObjectIdentitiesSelector.IsEmpty() && o.ObjectLabelSelector.IsEmpty())
}

// Matches implements [Selector].
func (o *ObjectIdentityLabelSelector) Matches(obj client.Object) bool {
	if o.IsEmpty() {
		return true
	}
	return o.ObjectIdentitySelector.Matches(obj) &&
		o.ObjectIdentitiesSelector.Matches(obj) &&
		o.ObjectLabelSelector.Matches(obj)
}
