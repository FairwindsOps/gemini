package snapshots

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fairwindsops/photon/pkg/kube"
	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"

	snapshotsv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

// GroupNameAnnotation contains the name of the SnapshotGroup associated with this VolumeSnapshot
const GroupNameAnnotation = "photon.fairwinds.com/group"

// IntervalsAnnotation contains the intervals that the VolumeSnapshot represents
const IntervalsAnnotation = "photon.fairwinds.com/intervals"

// TimestampAnnotation contains the timestamp of the VolumeSnapshot
const TimestampAnnotation = "photon.fairwinds.com/timestamp"

// RestoreAnnotation contains the restore point of the SnapshotGroup
const RestoreAnnotation = "photon.fairwinds.com/restore"

const managedByAnnotation = "app.kubernetes.io/managed-by"
const managerName = "photon"
const intervalsSeparator = ", "

type photonSnapshot struct {
	intervals []string
	snapshot  snapshotsv1.VolumeSnapshot
	timestamp time.Time
	restore   string
}

func listSnapshots(sg *v1.SnapshotGroup) ([]photonSnapshot, error) {
	client := kube.GetClient()
	snapshots, err := client.SnapshotClient.SnapshotV1alpha1().VolumeSnapshots(sg.ObjectMeta.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	photonSnapshots := []photonSnapshot{}
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
		intervals := strings.Split(snapshot.ObjectMeta.Annotations[IntervalsAnnotation], intervalsSeparator)
		photonSnapshots = append(photonSnapshots, photonSnapshot{
			snapshot:  snapshot,
			timestamp: time.Unix(int64(timestamp), 0),
			intervals: intervals,
			restore:   snapshot.ObjectMeta.Annotations[RestoreAnnotation],
		})
	}
	klog.Infof("Found %d snapshots for SnapshotGroup %s", len(photonSnapshots), sg.ObjectMeta.Name)
	sort.Slice(photonSnapshots, func(i, j int) bool {
		return photonSnapshots[j].timestamp.Before(photonSnapshots[i].timestamp)
	})
	return photonSnapshots, nil
}

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
			Source: &corev1.TypedLocalObjectReference{
				Name: sg.ObjectMeta.Name,
				Kind: "PersistentVolumeClaim",
			},
		},
	}
	client := kube.GetClient()
	snapClient := client.SnapshotClient.SnapshotV1alpha1().VolumeSnapshots(snapshot.ObjectMeta.Namespace)
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
	existing, err := listSnapshots(sg)
	if err != nil {
		return err
	}
	for _, snapshot := range existing {
		if snapshot.restore == restore {
			return nil
		}
	}
	klog.Infof("Creating snapshot for restore %s", restore)
	annotations := map[string]string{
		RestoreAnnotation: restore,
	}
	return createSnapshot(sg, annotations)
}

func deleteSnapshots(toDelete []photonSnapshot) error {
	klog.Infof("Deleting %d expired snapshots", len(toDelete))
	client := kube.GetClient()
	for _, snapshot := range toDelete {
		details := snapshot.snapshot.ObjectMeta
		snapClient := client.SnapshotClient.SnapshotV1alpha1().VolumeSnapshots(details.Namespace)
		err := snapClient.Delete(details.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		klog.Infof("Deleted snapshot %s", details.Name)
	}
	return nil
}
