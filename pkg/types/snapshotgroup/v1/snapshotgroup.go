package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core "k8s.io/kubernetes/pkg/apis/core"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=snapshotgroup

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SnapshotGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              SnapshotGroupSpec   `json:"spec"`
	Status            SnapshotGroupStatus `json:"status"`
}

type SnapshotGroupSpec struct {
	Claim    SnapshotClaim      `json:"claim"`
	Schedule []SnapshotSchedule `json:"schedule"`
}

type SnapshotClaim struct {
	Existing string                    `json:"existing"`
	Spec     core.PersistentVolumeSpec `json:"spec"`
}

type SnapshotSchedule struct {
	Every string `json:"every"`
	Keep  int    `json:"keep"`
}

type SnapshotGroupStatus struct{}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=snapshotgroup

// SnapshotGroupList is the list of SnapshotGroups.
type SnapshotGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SnapshotGroup `json:"items"`
}
