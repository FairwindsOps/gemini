package snapshots

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fairwindsops/photon/pkg/kube"
	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"

	snapshotsv1 "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
)

// PhotonSnapshot represents a VolumeSnapshot created by Photon
type PhotonSnapshot struct {
	Namespace string
	Name      string
	Intervals []string
	Timestamp time.Time
	Restore   string
}

// ListSnapshots returns all snapshots associated with a particular SnapshotGroup
func ListSnapshots(sg *v1.SnapshotGroup) ([]PhotonSnapshot, error) {
	client := kube.GetClient()
	snapshots, err := client.SnapshotClient.Namespace(sg.ObjectMeta.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	PhotonSnapshots := []PhotonSnapshot{}
	for _, snapshot := range snapshots.Items {
		snapshotMeta, err := meta.Accessor(&snapshot)
		if err != nil {
			return nil, err
		}
		annotations := snapshotMeta.GetAnnotations()
		if managedBy, ok := annotations[managedByAnnotation]; !ok || managedBy != managerName {
			continue
		}
		if annotations[GroupNameAnnotation] != sg.ObjectMeta.Name {
			continue
		}
		timestampStr := annotations[TimestampAnnotation]
		timestamp, err := strconv.Atoi(timestampStr)
		if err != nil {
			klog.Errorf("Failed to parse unix timestamp %s for %s", timestampStr, snapshotMeta.GetName())
			continue
		}
		intervals := []string{}
		intervalsStr := annotations[IntervalsAnnotation]
		if intervalsStr != "" {
			intervals = strings.Split(intervalsStr, intervalsSeparator)
		}
		PhotonSnapshots = append(PhotonSnapshots, PhotonSnapshot{
			Namespace: snapshotMeta.GetNamespace(),
			Name:      snapshotMeta.GetName(),
			Timestamp: time.Unix(int64(timestamp), 0),
			Intervals: intervals,
			Restore:   annotations[RestoreAnnotation],
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
	marshaled, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	unst := unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	err = json.Unmarshal(marshaled, &unst.Object)
	if err != nil {
		return err
	}
	client := kube.GetClient()
	unst.Object["kind"] = "VolumeSnapshot"
	unst.Object["apiVersion"] = client.VolumeSnapshotVersion

	if strings.HasSuffix(client.VolumeSnapshotVersion, "v1alpha1") {
		// There is a slight change in `source` from alpha to beta
		spec := unst.Object["spec"].(map[string]interface{})
		source := spec["source"].(map[string]interface{})
		delete(source, "persistentVolumeClaimName")
		source["name"] = sg.ObjectMeta.Name
		source["kind"] = "PersistentVolumeClaim"
		spec["source"] = source
		unst.Object["spec"] = spec
	}

	snapClient := client.SnapshotClient.Namespace(snapshot.ObjectMeta.Namespace)
	_, err = snapClient.Create(&unst, metav1.CreateOptions{})
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
		snapClient := client.SnapshotClient.Namespace(snapshot.Namespace)
		err := snapClient.Delete(snapshot.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		klog.Infof("Deleted snapshot %s/%s", snapshot.Namespace, snapshot.Name)
	}
	return nil
}
