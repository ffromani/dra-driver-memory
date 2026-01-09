package v0

// HugePageSize defines size of huge pages
// The allowed values for this depend on CPU architecture
// For x86/amd64, the valid values are 2M and 1G.
// For aarch64, the valid huge page sizes depend on the kernel page size:
// - With a 4k kernel page size: 64k, 2M, 32M, 1G
// - With a 64k kernel page size: 2M, 512M, 16G
//
// Reference: https://docs.kernel.org/mm/vmemmap_dedup.html
type HugePageSize string

// HugePageProvisionSpec defines a set of huge pages that we want to allocate
type HugePageProvisionSpec struct {
	// DefaultHugePagesSize defines huge pages default size under kernel boot parameters.
	DefaultHugePagesSize *HugePageSize `json:"defaultHugepagesSize,omitempty"`
	// Pages defines huge pages that we want to allocate at boot time.
	Pages []HugePage `json:"pages,omitempty"`
}

// HugePageProvisionStatus defines the observed state of Hugepages.
type HugePageProvisionStatus struct {
}

// HugePage defines the number of allocated huge pages of the specific size.
type HugePage struct {
	// Size defines huge page size, maps to the 'hugepagesz' kernel boot parameter.
	Size HugePageSize `json:"size,omitempty"`
	// Count defines amount of huge pages, maps to the 'hugepages' kernel boot parameter.
	Count int32 `json:"count,omitempty"`
	// Node defines the NUMA node where hugepages will be allocated,
	// if not specified, pages will be allocated equally between NUMA nodes
	// +optional
	Node *int32 `json:"node,omitempty"`
}

// HugePageProvision is the Schema for the hugepageprovision API
type HugePageProvision struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec   HugePageProvisionSpec   `json:"spec,omitempty"`
	Status HugePageProvisionStatus `json:"status,omitempty"`
}

// HugePageProvisionList contains a list of HugePage
type HugePageProvisionList struct {
	TypeMeta `json:",inline"`
	Items    []HugePageProvision `json:"items"`
}

// borrowed types from kube. Once we will move to a real API (if ever) we will
// just use the standard metav1 k8s packages

type TypeMeta struct {
	// Kind is a string value representing the REST resource this object represents.
	// Servers may infer this from the endpoint the client submits requests to.
	// Cannot be updated.
	// In CamelCase.
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`

	// APIVersion defines the versioned schema of this representation of an object.
	// Servers should convert recognized schemas to the latest internal value, and
	// may reject unrecognized values.
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,2,opt,name=apiVersion"`
}

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create.
type ObjectMeta struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names#names
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// Namespace defines the space within which each name must be unique. An empty namespace is
	// equivalent to the "default" namespace, but "default" is the canonical representation.
	// Not all objects are required to be scoped to a namespace - the value of this field for
	// those objects will be empty.
	//
	// Must be a DNS_LABEL.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
}
