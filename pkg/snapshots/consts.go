// Copyright 2020 FairwindsOps Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snapshots

// GroupNameAnnotation contains the name of the SnapshotGroup associated with this VolumeSnapshot
const GroupNameAnnotation = "gemini.fairwinds.com/group"

// IntervalsAnnotation contains the intervals that the VolumeSnapshot represents
const IntervalsAnnotation = "gemini.fairwinds.com/intervals"

// TimestampAnnotation contains the timestamp of the VolumeSnapshot
const TimestampAnnotation = "gemini.fairwinds.com/timestamp"

// RestoreAnnotation contains the restore point of the SnapshotGroup
const RestoreAnnotation = "gemini.fairwinds.com/restore"

// FSRStateAnnotation reports the AWS Fast Snapshot Restore state for a VolumeSnapshot.
// Set by Gemini when the parent SnapshotGroup has fastSnapshotRestore.enabled=true.
const FSRStateAnnotation = "gemini.fairwinds.com/fsr-state"

// FSREnabledAtAnnotation records the unix timestamp at which Gemini issued
// EnableFastSnapshotRestores for a snapshot. Used to enforce the warmup timeout.
const FSREnabledAtAnnotation = "gemini.fairwinds.com/fsr-enabled-at"

// FSRDisabledAtAnnotation records the unix timestamp at which Gemini issued
// DisableFastSnapshotRestores for a snapshot. Used to enforce the cooldown timeout.
const FSRDisabledAtAnnotation = "gemini.fairwinds.com/fsr-disabled-at"

// FSR state values written to FSRStateAnnotation. The operator treats any value
// other than FSRStateEnabled as non-selectable for hot scale-up.
const (
	FSRStateEnabling  = "enabling"
	FSRStateEnabled   = "enabled"
	FSRStateDisabling = "disabling"
	FSRStateDisabled  = "disabled"
	FSRStateFailed    = "failed"
)

const managedByAnnotation = "app.kubernetes.io/managed-by"
const managerName = "gemini"
const intervalsSeparator = ", "
