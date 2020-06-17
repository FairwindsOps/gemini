package snapshots

// GroupNameAnnotation contains the name of the SnapshotGroup associated with this VolumeSnapshot
const GroupNameAnnotation = "gemini.fairwinds.com/group"

// IntervalsAnnotation contains the intervals that the VolumeSnapshot represents
const IntervalsAnnotation = "gemini.fairwinds.com/intervals"

// TimestampAnnotation contains the timestamp of the VolumeSnapshot
const TimestampAnnotation = "gemini.fairwinds.com/timestamp"

// RestoreAnnotation contains the restore point of the SnapshotGroup
const RestoreAnnotation = "gemini.fairwinds.com/restore"

const managedByAnnotation = "app.kubernetes.io/managed-by"
const managerName = "gemini"
const intervalsSeparator = ", "
