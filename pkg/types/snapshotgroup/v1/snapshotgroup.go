package v1

import (
	snapshotsv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Template SnapshotTemplate   `json:"template"`
	Schedule []SnapshotSchedule `json:"schedule"`
}

type SnapshotClaim struct {
	Spec corev1.PersistentVolumeClaimSpec `json:"spec"`
}

type SnapshotTemplate struct {
	Spec snapshotsv1.VolumeSnapshotSpec `json:"spec"`
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
