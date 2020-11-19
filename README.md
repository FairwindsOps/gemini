<div align="center">
  <img src="/logo.png" alt="Gemini Logo" />
  <br>

  [![Version][version-image]][version-link] [![CircleCI][circleci-image]][circleci-link] [![Go Report Card][goreport-image]][goreport-link] [![Codecov][codecov-image]][codecov-link]
</div>

[version-image]: https://img.shields.io/static/v1.svg?label=Version&message=0.0.1&color=239922
[version-link]: https://github.com/FairwindsOps/gemini

[goreport-image]: https://goreportcard.com/badge/github.com/FairwindsOps/gemini
[goreport-link]: https://goreportcard.com/report/github.com/FairwindsOps/gemini

[circleci-image]: https://circleci.com/gh/FairwindsOps/gemini.svg?style=svg
[circleci-link]: https://circleci.com/gh/FairwindsOps/gemini

[codecov-image]: https://codecov.io/gh/FairwindsOps/gemini/branch/master/graph/badge.svg?token=7C20K7SYNR
[codecov-link]: https://codecov.io/gh/FairwindsOps/gemini

Gemini is a Kubernetes CRD and operator for managing `VolumeSnapshots`. This allows you
to back up your `PersistentVolumes` on a regular schedule, retire old backups, and restore
backups with minimal downtime.


**Want to learn more?** Reach out on [the Slack channel](https://fairwindscommunity.slack.com/messages/gemini) ([request invite](https://join.slack.com/t/fairwindscommunity/shared_invite/zt-e3c6vj4l-3lIH6dvKqzWII5fSSFDi1g)), send an email to `opensource@fairwinds.com`, or join us for [office hours on Zoom](https://fairwindscommunity.slack.com/messages/office-hours)

> Note: Like the VolumeSnapshot API it builds on, Gemini is **currently in beta**.

## Installation
The Gemini Helm chart will install both the CRD and the operator into your cluster

```bash
kubectl create ns gemini
helm repo add fairwinds-stable https://charts.fairwinds.com/stable
helm install gemini fairwinds-stable/gemini --namespace gemini
```

### Prerequisites
You'll need to have the `VolumeSnapshot` API available in your cluster. This API is in
[beta as of Kubernetes 1.17](https://kubernetes.io/docs/concepts/storage/volume-snapshots/),
and was introduced as alpha in 1.12.

To check if your cluster has `VolumeSnapshots` available, you can run
```bash
kubectl api-resources | grep volumesnapshots
```

* To enable on v1.12-16, set the flag `--feature-gates=VolumeSnapshotDataSource=true` on the API server binary [source](https://kubernetes.io/blog/2018/10/09/introducing-volume-snapshot-alpha-for-kubernetes/#kubernetes-snapshots-requirements)
* To enable VolumeSnapshots on kops, see our [instructions here](/examples/bash)
* Depending on your environment, you may need to configure the VolumeSnapshot API as well as the CSI. Fortunately, some managed Kubernetes providers like DigitalOcean support VolumeSnapshots by default, even on older versions

Before getting started with Gemini, it's a good idea to make sure you're able to
[create a VolumeSnapshot manually](https://kubernetes.io/docs/concepts/storage/volume-snapshots/#volumesnapshots).


## Usage

### Backup
Gemini can schedule backups for an existing PVC, or create a new PVC to back up.

#### Schedules

The `schedule` parameter tells Gemini how often to run backups, and how many historical backups to keep.

For example, the following schedule tells Gemini to create new backups every 10 minutes,
always keep the last 3 backups, and keep historical hourly, daily, monthly, and yearly backups.

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

Note that `keep` specifies how many historical backups you want, _in addition_ to the most recent backup.
This way the schedule
```yaml
- every: 10 minutes
  keep: 3
```
will always give you _at least_ 30 minutes of backup coverage. But you will see four snapshots at any given time.
E.g. right after a new snapshot is created, you'll see backups for
* 0m ago
* 10m ago
* 20m ago
* 30m ago


#### Using an Existing PVC
> See the [extended example](/examples/hackmd/README.md)

The following example schedules backups every 10 minutes for a pre-existing PVC named `postgres`.

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
```

#### Creating a New PVC
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

The PVC will have the same name as the SnapshotGroup, (in this example, `test-volume`)

#### Snapshot Spec
You can use the `spec.template` field to set the template for any `VolumeSnapshots` that get created,
most notably the name of the [snapshot class](https://kubernetes.io/docs/concepts/storage/volume-snapshot-classes/)
you want to use.

```yaml
apiVersion: gemini.fairwinds.com/v1beta1
kind: SnapshotGroup
metadata:
  name: test-volume
spec:
  persistentVolumeClaim:
    claimName: postgres
  schedule:
    - every: "10 minutes"
      keep: 3
  template:
    spec:
      volumeSnapshotClassName: test-snapshot-class      
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

## Caveats
* Like the VolumeSnapshot API it builds on, Gemini is **currently in beta**
* Be sure to test out both the backup and restore process to ensure Gemini is working properly
* VolumeSnapshots simply grab the current state of the volume, without respect for things like in-flight database transactions. You may find you need to stop the application in order to get a consistently usable VolumeSnapshot.
