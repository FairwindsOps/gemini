# gemini

## Design Doc
https://paper.dropbox.com/doc/Photon-Design-Doc--AxC2l~E4g1rQkAk5S4haNYtOAg-pWh0uK2eUgNGZSnuK2Lid

## Example
To see gemini working end-to-end, check out [the HackMD example](examples/hackmd)

## Quickstart
> Note: haven't managed to get KIND working with VolumeSnapshots

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
  github.com/fairwindsops/gemini/pkg/types/snapshotgroup/v1/apis \
  github.com/fairwindsops/gemini/pkg/types \
  "snapshotgroup:v1"
```

I had to manually edit
pkg/types/snapshotgroup/v1/apis/clientset/versioned/typed/snapshotgroup/v1/snapshotgroup.go
due to some complaints about a `context` argument getting passed in.

