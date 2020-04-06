package snapshots

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
