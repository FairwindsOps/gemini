package snapshots

import (
	"fmt"
	"github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1"
)

func AddOrUpdateSnapshotGroup(sg *v1.SnapshotGroup) error {
	fmt.Println("Reconcile sg", sg)
	return nil
}
