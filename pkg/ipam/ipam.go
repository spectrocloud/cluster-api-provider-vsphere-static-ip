package ipam

import (
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectKey identifies a Kubernetes Object.
type ObjectKey = types.NamespacedName

// function to create a new IPAM
type NewIpamFunc func(cli client.Client, log logr.Logger) IPAddressManager

type IpamType string

const (
	IpamTypeMetal3io IpamType = "metal3io"
)

type IPAddressStr string

// CreateOption is some configuration that modifies options for a create request.
type CreateOption interface {
	// ApplyToCreate applies this configuration to the given create options.
	ApplyToCreate(*CreateOptions)
}

// CreateOptions contains options for create requests. It's generally a subset
// of metav1.CreateOptions.
type CreateOptions struct {
	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string

	// FieldManager is the name of the user or component submitting
	// this request.  It must be set with server-side apply.
	FieldManager string

	// Raw represents raw CreateOptions, as passed to the API server.
	Raw *metav1.CreateOptions
}

// AsCreateOptions returns these options as a metav1.CreateOptions.
// This may mutate the Raw field.
func (o *CreateOptions) AsCreateOptions() *metav1.CreateOptions {
	if o == nil {
		return &metav1.CreateOptions{}
	}
	if o.Raw == nil {
		o.Raw = &metav1.CreateOptions{}
	}

	o.Raw.DryRun = o.DryRun
	o.Raw.FieldManager = o.FieldManager
	return o.Raw
}

// ApplyOptions applies the given create options on these options,
// and then returns itself (for convenient chaining).
func (o *CreateOptions) ApplyOptions(opts []CreateOption) *CreateOptions {
	for _, opt := range opts {
		opt.ApplyToCreate(o)
	}
	return o
}

// ApplyToCreate implements CreateOption
func (o *CreateOptions) ApplyToCreate(co *CreateOptions) {
	if o.DryRun != nil {
		co.DryRun = o.DryRun
	}
	if o.FieldManager != "" {
		co.FieldManager = o.FieldManager
	}
	if o.Raw != nil {
		co.Raw = o.Raw
	}
}

var _ CreateOption = &CreateOptions{}

// DeleteOptions contains options for delete requests. It's generally a subset
// of metav1.DeleteOptions.
type DeleteOptions struct {
	// GracePeriodSeconds is the duration in seconds before the object should be
	// deleted. Value must be non-negative integer. The value zero indicates
	// delete immediately. If this value is nil, the default grace period for the
	// specified type will be used.
	GracePeriodSeconds *int64

	// Preconditions must be fulfilled before a deletion is carried out. If not
	// possible, a 409 Conflict status will be returned.
	Preconditions *metav1.Preconditions

	// PropagationPolicy determined whether and how garbage collection will be
	// performed. Either this field or OrphanDependents may be set, but not both.
	// The default policy is decided by the existing finalizer set in the
	// metadata.finalizers and the resource-specific default policy.
	// Acceptable values are: 'Orphan' - orphan the dependents; 'Background' -
	// allow the garbage collector to delete the dependents in the background;
	// 'Foreground' - a cascading policy that deletes all dependents in the
	// foreground.
	PropagationPolicy *metav1.DeletionPropagation

	// Raw represents raw DeleteOptions, as passed to the API server.
	Raw *metav1.DeleteOptions

	// When present, indicates that modifications should not be
	// persisted. An invalid or unrecognized dryRun directive will
	// result in an error response and no further processing of the
	// request. Valid values are:
	// - All: all dry run stages will be processed
	DryRun []string
}

// AsDeleteOptions returns these options as a metav1.DeleteOptions.
// This may mutate the Raw field.
func (o *DeleteOptions) AsDeleteOptions() *metav1.DeleteOptions {
	if o == nil {
		return &metav1.DeleteOptions{}
	}
	if o.Raw == nil {
		o.Raw = &metav1.DeleteOptions{}
	}

	o.Raw.GracePeriodSeconds = o.GracePeriodSeconds
	o.Raw.Preconditions = o.Preconditions
	o.Raw.PropagationPolicy = o.PropagationPolicy
	o.Raw.DryRun = o.DryRun
	return o.Raw
}

// DeleteOption is some configuration that modifies options for a delete request.
type DeleteOption interface {
	// ApplyToDelete applies this configuration to the given delete options.
	ApplyToDelete(*DeleteOptions)
}

// ApplyOptions applies the given delete options on these options,
// and then returns itself (for convenient chaining).
func (o *DeleteOptions) ApplyOptions(opts []DeleteOption) *DeleteOptions {
	for _, opt := range opts {
		opt.ApplyToDelete(o)
	}
	return o
}

var _ DeleteOption = &DeleteOptions{}

// ApplyToDelete implements DeleteOption
func (o *DeleteOptions) ApplyToDelete(do *DeleteOptions) {
	if o.GracePeriodSeconds != nil {
		do.GracePeriodSeconds = o.GracePeriodSeconds
	}
	if o.Preconditions != nil {
		do.Preconditions = o.Preconditions
	}
	if o.PropagationPolicy != nil {
		do.PropagationPolicy = o.PropagationPolicy
	}
	if o.Raw != nil {
		do.Raw = o.Raw
	}
	if o.DryRun != nil {
		do.DryRun = o.DryRun
	}
}
