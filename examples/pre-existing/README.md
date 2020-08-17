# Gemini Example: HackMD
> Note: this will not work in a KIND cluster. It has been tested on DigitalOcean.

### Install the controller
```
kubectl create ns gemini
helm repo add fairwinds-stable https://charts.fairwinds.com/stable 
helm install gemini fairwinds-stable/gemini --namespace gemini
```

### Install HackMD
> Note, we're using a fork of the chart in order to support k8s 1.16
```
reckoner plot examples/pre-existing/course.yaml
```

This will create two PVCs, one for HackMD, and one for the Postgres instance that backs it.

### Set up the Backup Schedule
```
kubectl apply -f examples/pre-existing/snapshotgroup.yaml
```

within 30 seconds or so, you should see a couple `VolumeSnapshots`:
```
$ kubectl get volumesnapshot
NAME                           READYTOUSE   SOURCEPVC                  SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS      SNAPSHOTCONTENT                                    CREATIONTIME   AGE
hackmd-1594929306              true         hackmd                                             2Gi           do-block-storage   snapcontent-2dac493e-116e-41b7-9ca2-cb797ac7c40b   17s            19s
hackmd-postgresql-1594929307   true         data-hackmd-postgresql-0                           8Gi           do-block-storage   snapcontent-300e10a1-945a-483f-a461-9b073c853ddf   16s            18s
```

### Create a document
```
kubectl port-forward svc/hackmd 3000:3000 -n notepad
```

Visit `localhost:3000` and create a new guest document. Enter some dummy text.

### Trigger a backup
Rather than waiting for Gemini to create the next backup, you can delete existing
backups, and Gemini will create a new one.

```
kubectl delete volumesnapshot --all -n notepad
```

Within 30 seconds, you should see new snapshots appear. Make sure to wait until `READYTOUSE` is true
```
$ kubectl get volumesnapshot -n notepad
NAME                           READYTOUSE   SOURCEPVC                  SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS      SNAPSHOTCONTENT                                    CREATIONTIME   AGE
hackmd-1594929516              true         hackmd                                             2Gi           do-block-storage   snapcontent-e75421c6-c4ca-4bbf-81f4-a2fb0706b957   5s             7s
hackmd-postgresql-1594929517   true         data-hackmd-postgresql-0                           8Gi           do-block-storage   snapcontent-ad71c1f8-af7b-4cdc-85ba-e512a77095a3   4s             6s
```

### Edit your document again
```
kubectl port-forward svc/hackmd 3000:3000 -n notepad
```

Go back to your document (if you lost it, you can find it again by going to
`localhost:3000` and clicking `History`).

Make some more dummy edits. These will get deleted when we restore.

### Perform the restore
> Note that we only need to restore PostgreSQL - we didn't change anything in the core app.

First, we need to scale down our deployment. We can't swap out a PVC in-place,
so you'll necessarily incur some downtime.

```
kubectl scale all --all --replicas=0 -n notepad
```

Next, annotate the `SnapshotGroup` with the timestamp of the snapshot you want.

For example, here we'll use timestamp `1585945609`.
```
$ kubectl get volumesnapshot -n notepad
NAME                           AGE
hackmd-1585945609              15s
hackmd-postgresql-1585945609   15s
```

```
kubectl annotate snapshotgroup/hackmd-postgresql -n notepad --overwrite \
  "gemini.fairwinds.com/restore=1585945609"
```

This will:
* create a one-off backup of your PVC
* delete the PVC
* create a new PVC with the same name from your snapshot

Finally, we can scale back up:
```
kubectl scale all --all --replicas=1 -n notepad
```

### Verify the restore
```
kubectl port-forward svc/hackmd 3000:3000 -n notepad
```
Go back to your document. The second round of edits you made should be gone!
