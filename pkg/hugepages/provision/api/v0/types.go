package v0

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	// Conditions represents the latest available observations of current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=performanceprofiles,scope=Cluster
// +kubebuilder:storageversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HugePageProvision is the Schema for the hugepageprovision API
type HugePageProvision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HugePageProvisionSpec   `json:"spec,omitempty"`
	Status HugePageProvisionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HugePageProvisionList contains a list of HugePage
type HugePageProvisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HugePageProvision `json:"items"`
}
