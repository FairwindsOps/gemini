## Quickstart
> Note: haven't managed to get KIND working with VolumeSnapshots
> See https://github.com/rancher/local-path-provisioner/issues/81

```
go run main.go &
k apply -f examples/hackmd/snapshotgroup.yaml
k get volumesnapshot --watch
```

## Development
CRD generation mostly follows [this example](https://github.com/jinghzhu/KubernetesCRD)

### Generated files
```
$GOPATH/src/k8s.io/code-generator/generate-groups.sh all \
  github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1beta1/apis \
  github.com/fairwindsops/gemini/pkg/types \
  "snapshotgroup:v1beta1"
```

I had to manually edit
pkg/types/snapshotgroup/v1beta1/apis/clientset/versioned/typed/snapshotgroup/v1beta1/snapshotgroup.go
due to some complaints about a `context` argument getting passed in.

