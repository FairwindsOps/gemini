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
to create a snapshot of the data on your `PersistentVolumes` on a regular schedule,
retire old snapshots, and restore snapshots with minimal downtime.

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

### Upgrading to V2
Version 2.0 of Gemini updates the CRD from `v1beta1` to `v1`. There are no substantial
changes, but `v1` adds better support for PersistentVolumeClaims on Kubernetes 1.25.

If you want to keep the v1beta1 CRD available, you can run:
```
kubectl apply -f https://raw.githubusercontent.com/FairwindsOps/gemini/main/pkg/types/snapshotgroup/v1beta1/crd-with-beta1.yaml
```
before upgrading, and add `--skip-crds` when running `helm install`.

## Usage

### Snapshots
Gemini can schedule snapshots for an existing PVC, or create a new PVC to back up.

#### Schedules

The `schedule` parameter tells Gemini how often to create snapshots, and how many historical snapshots to keep.

For example, the following schedule tells Gemini to create a snapshot every day, keeping two weeks worth of history:
```yaml
apiVersion: gemini.fairwinds.com/v1beta1
kind: SnapshotGroup
metadata:
  name: test-volume
spec:
  persistentVolumeClaim:
    claimName: postgres
  schedule:
    - every: day
      keep: 14
```

For a more complex example, Gemini can create new snapshots every 10 minutes,
always keep the last 3 snapshots, and preserve historical hourly, daily, monthly, and yearly snapshots.

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

Note that `keep` specifies how many historical snapshots you want, _in addition_ to the most recent snapshot.
This way the schedule
```yaml
- every: 10 minutes
  keep: 3
```
will always give you _at least_ 30 minutes of snapshot coverage. But you will see four snapshots at any given time.
E.g. right after a new snapshot is created, you'll see snapshots for
* 0m ago
* 10m ago
* 20m ago
* 30m ago


#### Using an Existing PVC
> See the [extended example](/examples/codimd/README.md)
The following example schedules snapshots every 10 minutes for a pre-existing PVC named `postgres`.

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

Then, copy the timestamp from the first step, and use that to annotate the `SnapshotGroup`:
```bash
$ kubectl annotate snapshotgroup/test-volume --overwrite \
  "gemini.fairwinds.com/restore=1585945609"
```

Finally, you can scale your Pods back up:
```bash
$ kubectl scale all --all --replicas=1
```

## End-to-End Example
To see gemini working end-to-end, check out [the CodiMD example](examples/codimd)

## Caveats
* Like the VolumeSnapshot API it builds on, Gemini is **currently in beta**
* Be sure to test out both the snapshot and restore process to ensure Gemini is working properly
* VolumeSnapshots simply grab the current state of the volume, without respect for things like in-flight database transactions. You may find you need to stop the application in order to get a consistently usable VolumeSnapshot.

<!-- Begin boilerplate -->
## Join the Fairwinds Open Source Community

The goal of the Fairwinds Community is to exchange ideas, influence the open source roadmap,
and network with fellow Kubernetes users.
[Chat with us on Slack](https://join.slack.com/t/fairwindscommunity/shared_invite/zt-e3c6vj4l-3lIH6dvKqzWII5fSSFDi1g)
[join the user group](https://www.fairwinds.com/open-source-software-user-group) to get involved!

<a href="https://www.fairwinds.com/t-shirt-offer?utm_source=gemini&utm_medium=gemini&utm_campaign=gemini-tshirt">
  <img src="https://www.fairwinds.com/hubfs/Doc_Banners/Fairwinds_OSS_User_Group_740x125_v6.png" alt="Love Fairwinds Open Source? Share your business email and job title and we'll send you a free Fairwinds t-shirt!" />
</a>

## Other Projects from Fairwinds

Enjoying Gemini? Check out some of our other projects:
* [Polaris](https://github.com/FairwindsOps/Polaris) - Audit, enforce, and build policies for Kubernetes resources, including over 20 built-in checks for best practices
* [Goldilocks](https://github.com/FairwindsOps/Goldilocks) - Right-size your Kubernetes Deployments by compare your memory and CPU settings against actual usage
* [Pluto](https://github.com/FairwindsOps/Pluto) - Detect Kubernetes resources that have been deprecated or removed in future versions
* [Nova](https://github.com/FairwindsOps/Nova) - Check to see if any of your Helm charts have updates available
* [rbac-manager](https://github.com/FairwindsOps/rbac-manager) - Simplify the management of RBAC in your Kubernetes clusters
