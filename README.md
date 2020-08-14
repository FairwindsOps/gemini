<div align="center">
  <img src="/logo.png" alt="Gemini Logo" />
  <br>

  [![Version][version-image]][version-link] [![CircleCI][circleci-image]][circleci-link] [![Go Report Card][goreport-image]][goreport-link]
</div>

[version-image]: https://img.shields.io/static/v1.svg?label=Version&message=0.0.1&color=239922
[version-link]: https://github.com/FairwindsOps/gemini

[goreport-image]: https://goreportcard.com/badge/github.com/FairwindsOps/gemini
[goreport-link]: https://goreportcard.com/report/github.com/FairwindsOps/gemini

[circleci-image]: https://circleci.com/gh/FairwindsOps/gemini.svg?style=svg
[circleci-link]: https://circleci.com/gh/FairwindsOps/gemini.svg

Gemini is a Kubernetes CRD and operator for managing `VolumeSnapshots`. This allows you
to back up your `PersistentVolumes` on a regular schedule, retire old backups, and restore
backups with minimal downtime.

## Installation
The Gemini Helm chart will install both the CRD and the operator into your cluster

```bash
# TODO: change to charts repo
git clone https://github.com/FairwindsOps/gemini
kubectl create ns gemini
helm install -n gemini gemini ./deploy/charts/gemini
```

## Usage

### Backup
Gemini can schedule backups for an existing PVC, or create a new PVC to back up.

#### Existing PVC
> See the [extended example](/examples/pre-existing/README.md)

The following example schedules backups every 10 minutes for a pre-existing PVC named `postgres`.

The `schedule` parameter tells Gemini to always keep the last 3 backups, as well as
hourly, daily, monthly, and yearly backups.

```yaml
apiVersion: gemini.fairwinds.com/v1beta1
kind: SnapshotGroup
metadata:
  name: test-volume
spec:
  persistentVolumeClaim:
    claimName: postgres
  schedule:
    - every: 10 minutes
      keep: 3
    - every: hour
      keep: 1
    - every: day
      keep: 1
    - every: month
      keep: 1
    - every: year
      keep: 1
```

#### New PVC
You can also specify an entire PVC spec inside the SnapshotGroup if you'd like Gemini to create
the PVC for you.
```yaml
apiVersion: gemini.fairwinds.com/v1beta1
kind: SnapshotGroup
metadata:
  name: test-volume
spec:
  persistentVolumeClaim:
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
  schedule:
    - every: 10 minutes
      keep: 3
```

### Restore
> Caution: you cannot alter a PVC without some downtime!

You can restore your PVC to a particular point in time using an annotation.

First, check out what `VolumeSnapshots` are available:
```bash
$ kubectl get volumesnapshot
NAME                           AGE
test-volume-1585945609         15s
```

Next, you'll need to remove any Pods that are using the PVC:
```bash
$ kubectl scale all --all --replicas=0
```

The copy the timestamp from the first step, and use that to annotate the `SnapshotGroup`:
```bash
$ kubectl annotate snapshotgroup/test-volume --overwrite \
  "gemini.fairwinds.com/restore=1585945609"
```

Finally, you can scale your Pods back up:
```bash
$ kubectl scale all --all --replicas=1
```

## End-to-End Example
To see gemini working end-to-end, check out [the HackMD example](examples/hackmd)

