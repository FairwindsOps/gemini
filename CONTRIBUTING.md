# Contributing

Issues, whether bugs, tasks, or feature requests are essential for keeping Gemini great. We believe it should be as easy as possible to contribute changes that get things working in your environment. There are a few guidelines that we need contributors to follow so that we can keep on top of things.

## Code of Conduct

This project adheres to a [code of conduct](CODE_OF_CONDUCT.md). Please review this document before contributing to this project.

## Sign the CLA
Before you can contribute, you will need to sign the [Contributor License Agreement](https://cla-assistant.io/fairwindsops/gemini).

## Quickstart
> Note: haven't managed to get KIND working with VolumeSnapshots
> See https://github.com/rancher/local-path-provisioner/issues/81

```
go run main.go &
kubectl apply -f examples/hackmd/snapshotgroup.yaml
kubectl get volumesnapshot --watch
```

## Development

### Project structure
* `pkg/controller` - watches SnapshotGroup resources
* `pkg/kube` - client for interacting with the Kubernetes API
* `pkg/snapshots` - contains most of the logic for Gemini:
* `pkg/snapshots/groups.go` - high-level logic for handling updates to SnapshotGroups
* `pkg/snapshots/snapshots.go` - create, delete, and update VolumeSnapshots based on SnapshotGroups
* `pkg/snapshots/pvc.go` - create, delete, and update PVCs based on changes to SnapshotGroups
* `pkg/snapshots/scheduler.go` - logic for scheduling creation/deletion of VolumeSnapshots

### Generated files
CRD generation mostly follows [this example](https://github.com/jinghzhu/KubernetesCRD)
```
$GOPATH/src/k8s.io/code-generator/generate-groups.sh all \
  github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1/apis \
  github.com/fairwindsops/gemini/pkg/types \
  "snapshotgroup:v1beta1"
```

I had to manually edit
pkg/types/snapshotgroup/v1beta1/apis/clientset/versioned/typed/snapshotgroup/v1beta1/snapshotgroup.go
due to some complaints about a `context` argument getting passed in.

## Releases
To release a new version of Gemini, tag the master branch with the format `x.x.x`,
and push your tags.

For minor/major upgrades, please update the
[helm chart](https://github.com/FairwindsOps/charts/tree/master/stable/goldilocks) as well
