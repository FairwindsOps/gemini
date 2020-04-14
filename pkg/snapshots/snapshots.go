package snapshots

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fairwindsops/photon/pkg/kube"
	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"

	snapshotsv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

// PhotonSnapshot represents a VolumeSnapshot created by Photon
type PhotonSnapshot struct {
	Intervals []string
	Snapshot  snapshotsv1.VolumeSnapshot
	Timestamp time.Time
	Restore   string
}

// ListSnapshots returns all snapshots associated with a particular SnapshotGroup
func ListSnapshots(sg *v1.SnapshotGroup) ([]PhotonSnapshot, error) {
	client := kube.GetClient()
	snapshots, err := client.SnapshotClient.SnapshotV1beta1().VolumeSnapshots(sg.ObjectMeta.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	PhotonSnapshots := []PhotonSnapshot{}
	for _, snapshot := range snapshots.Items {
		if managedBy, ok := snapshot.ObjectMeta.Annotations[managedByAnnotation]; !ok || managedBy != managerName {
			continue
		}
		if snapshot.ObjectMeta.Annotations[GroupNameAnnotation] != sg.ObjectMeta.Name {
			continue
		}
		timestampStr := snapshot.ObjectMeta.Annotations[TimestampAnnotation]
		timestamp, err := strconv.Atoi(timestampStr)
		if err != nil {
			klog.Errorf("Failed to parse unix timestamp %s for %s", timestampStr, snapshot.ObjectMeta.Name)
			continue
		}
		intervals := []string{}
		intervalsStr := snapshot.ObjectMeta.Annotations[IntervalsAnnotation]
		if intervalsStr != "" {
			intervals = strings.Split(intervalsStr, intervalsSeparator)
		}
		PhotonSnapshots = append(PhotonSnapshots, PhotonSnapshot{
			Snapshot:  snapshot,
			Timestamp: time.Unix(int64(timestamp), 0),
			Intervals: intervals,
			Restore:   snapshot.ObjectMeta.Annotations[RestoreAnnotation],
		})
	}
	klog.Infof("Found %d snapshots for SnapshotGroup %s", len(PhotonSnapshots), sg.ObjectMeta.Name)
	sort.Slice(PhotonSnapshots, func(i, j int) bool {
		return PhotonSnapshots[j].Timestamp.Before(PhotonSnapshots[i].Timestamp)
	})
	return PhotonSnapshots, nil
}

// createSnapshot creates a new snappshot for a given SnapshotGroup
func createSnapshot(sg *v1.SnapshotGroup, annotations map[string]string) error {
	timestamp := strconv.Itoa(int(time.Now().Unix()))
	annotations[TimestampAnnotation] = timestamp
	annotations[managedByAnnotation] = managerName
	annotations[GroupNameAnnotation] = sg.ObjectMeta.Name

	snapshot := snapshotsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   sg.ObjectMeta.Namespace,
			Name:        sg.ObjectMeta.Name + "-" + timestamp,
			Annotations: annotations,
		},
		Spec: snapshotsv1.VolumeSnapshotSpec{
			Source: snapshotsv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &sg.ObjectMeta.Name,
			},
		},
	}
	client := kube.GetClient()
	snapClient := client.SnapshotClient.SnapshotV1beta1().VolumeSnapshots(snapshot.ObjectMeta.Namespace)
	_, err := snapClient.Create(&snapshot)
	return err
}

func createSnapshotForIntervals(sg *v1.SnapshotGroup, intervals []string) error {
	if len(intervals) == 0 {
		return nil
	}
	klog.Infof("Creating snapshot for intervals %v", intervals)
	annotations := map[string]string{
		IntervalsAnnotation: strings.Join(intervals, intervalsSeparator),
	}
	return createSnapshot(sg, annotations)
}

func createSnapshotForRestore(sg *v1.SnapshotGroup) error {
	restore := sg.ObjectMeta.Annotations[RestoreAnnotation]
	existing, err := ListSnapshots(sg)
	if err != nil {
		return err
	}
	for _, snapshot := range existing {
		if snapshot.Restore == restore {
			klog.Infof("Snapshot already exists for timestamp %s", restore)
			return nil
		}
	}
	klog.Infof("Creating snapshot for restore %s", restore)
	annotations := map[string]string{
		RestoreAnnotation: restore,
	}
	return createSnapshot(sg, annotations)
}

func deleteSnapshots(toDelete []PhotonSnapshot) error {
	klog.Infof("Deleting %d expired snapshots", len(toDelete))
	client := kube.GetClient()
	for _, snapshot := range toDelete {
		details := snapshot.Snapshot.ObjectMeta
		snapClient := client.SnapshotClient.SnapshotV1beta1().VolumeSnapshots(details.Namespace)
		err := snapClient.Delete(details.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		klog.Infof("Deleted snapshot %s", details.Name)
	}
	return nil
}
