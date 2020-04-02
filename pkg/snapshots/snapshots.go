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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const groupNameLabel = "photon.fairwinds.com/group"
const intervalsLabel = "photon.fairwinds.com/intervals"
const timestampLabel = "photon.fairwinds.com/timestamp"
const managedByLabel = "app.kubernetes.io/managed-by"
const managerName = "photon"
const intervalsSeparator = ", "

type photonSnapshot struct {
	intervals []string
	snapshot  snapshotsv1.VolumeSnapshot
	timestamp time.Time
}

// AddOrUpdateSnapshotGroup handles any changes to SnapshotGroups
func AddOrUpdateSnapshotGroup(sg *v1.SnapshotGroup) error {
	klog.Infof("Reconcile SnapshotGroup %s/%s", sg.ObjectMeta.Namespace, sg.ObjectMeta.Name)
	err := maybeCreatePVC(sg)
	if err != nil {
		return err
	}

	snapshots, err := listSnapshots(sg)
	if err != nil {
		return err
	}
	toCreate, toDelete := getSnapshotChanges(sg.Spec.Schedule, snapshots)
	if len(toCreate) > 0 {
		err = createSnapshot(sg, toCreate)
		if err != nil {
			return err
		}
	}
	err = deleteSnapshots(toDelete)
	if err != nil {
		return err
	}

	return nil
}

func maybeCreatePVC(sg *v1.SnapshotGroup) error {
	client := kube.GetClient()
	pvcClient := client.K8s.CoreV1().PersistentVolumeClaims(sg.ObjectMeta.Namespace)
	pvc, err := pvcClient.Get(sg.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		klog.Infof("pvc %s not found, creating it", pvc.ObjectMeta.Name)
		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sg.ObjectMeta.Name,
				Namespace: sg.ObjectMeta.Namespace,
				Annotations: map[string]string{
					managedByLabel: managerName,
				},
			},
			Spec: sg.Spec.Claim.Spec,
		}
		_, err := pvcClient.Create(pvc)
		if err != nil {
			return err
		}
	} else {
		klog.Infof("Found pvc %s", pvc.ObjectMeta.Name)
	}
	return nil
}

func listSnapshots(sg *v1.SnapshotGroup) ([]photonSnapshot, error) {
	client := kube.GetClient()
	snapshots, err := client.SnapshotClient.SnapshotV1alpha1().VolumeSnapshots(sg.ObjectMeta.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	photonSnapshots := []photonSnapshot{}
	for _, snapshot := range snapshots.Items {
		if managedBy, ok := snapshot.ObjectMeta.Annotations[managedByLabel]; !ok || managedBy != managerName {
			continue
		}
		if snapshot.ObjectMeta.Annotations[groupNameLabel] != sg.ObjectMeta.Name {
			continue
		}
		timestampStr := snapshot.ObjectMeta.Annotations[timestampLabel]
		timestamp, err := strconv.Atoi(timestampStr)
		if err != nil {
			klog.Errorf("Failed to parse unix timestamp %s for %s", timestampStr, snapshot.ObjectMeta.Name)
			continue
		}
		intervals := strings.Split(snapshot.ObjectMeta.Annotations[intervalsLabel], intervalsSeparator)
		photonSnapshots = append(photonSnapshots, photonSnapshot{
			snapshot:  snapshot,
			timestamp: time.Unix(int64(timestamp), 0),
			intervals: intervals,
		})
	}
	klog.Infof("Found %d snapshots for SnapshotGroup %s", len(photonSnapshots), sg.ObjectMeta.Name)
	sort.Slice(photonSnapshots, func(i, j int) bool {
		return photonSnapshots[j].timestamp.Before(photonSnapshots[i].timestamp)
	})
	return photonSnapshots, nil
}

func createSnapshot(sg *v1.SnapshotGroup, intervals []string) error {
	klog.Infof("Creating snapshot for intervals %v", intervals)
	now := time.Now().UTC()
	timestamp := now.Unix()
	timestampStr := strconv.Itoa(int(timestamp))
	snapshot := snapshotsv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: sg.ObjectMeta.Namespace,
			Name:      sg.ObjectMeta.Name + "-" + timestampStr,
			Annotations: map[string]string{
				managedByLabel: managerName,
				timestampLabel: timestampStr,
				groupNameLabel: sg.ObjectMeta.Name,
				intervalsLabel: strings.Join(intervals, intervalsSeparator),
			},
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
	}
	return nil
}
