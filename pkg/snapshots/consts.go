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

const managedByAnnotation = "app.kubernetes.io/managed-by"
const managerName = "gemini"
const intervalsSeparator = ", "
