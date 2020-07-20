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
