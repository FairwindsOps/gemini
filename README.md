# photon
## Design Doc
https://paper.dropbox.com/doc/Photon-Design-Doc--AxC2l~E4g1rQkAk5S4haNYtOAg-pWh0uK2eUgNGZSnuK2Lid

## Development
CRD generation mostly follows [this example](https://github.com/jinghzhu/KubernetesCRD)

### Generated files
```
$GOPATH/src/k8s.io/code-generator/generate-groups.sh all \
  github.com/fairwindsops/photon/pkg/types/snapshotgroup/v1/apis \
  github.com/fairwindsops/photon/pkg/types \
  "snapshotgroup:v1"
```
