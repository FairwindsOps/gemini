# Photon Example: HackMD
> Note: this will not work in a KIND cluster. It has been tested on DigitalOcean.

### Install the controller
```
kubectl create ns photon
helm install photon deploy/charts/photon --namespace photon
```

### Create the `snapshotgroup`
```
kubectl create ns notepad
kubectl apply -f examples/hackmd/snapshotgroup.yaml -n notepad
```

This will create two PVCs to be used by HackMD. They will stay in `Pending` status until we create the application.
```
$ k get pvc
NAME                STATUS    VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS   AGE
hackmd              Pending                                      gp2            44s
hackmd-postgresql   Pending                                      gp2            44s
```


### Install HackMD
> Note, we're using a fork of the chart in order to support k8s 1.16
```
reckoner plot examples/hackmd/course.yaml
```

Note that in `course.yaml`, we specify `existingClaim` for both the HackMD
app, and the PostgreSQL database.

### Create a document
```
kubectl port-forward svc/hackmd 3000:3000 -n notepad
```

Visit `localhost:3000` and create a new guest document. Enter some dummy text.

### Trigger a backup
Rather than waiting for Photon to create the next backup, you can delete existing
backups, and Photon will create a new one.

```
kubectl delete volumesnapshot --all -n notepad
```

Within 30 seconds, you should see new snapshots appear
```
$ kubectl get volumesnapshot -n notepad
NAME                           AGE
hackmd-1585945609              15s
hackmd-postgresql-1585945609   15s
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
kubectl annotate snapshotgroup/hackmd-postgresql --overwrite \
  "photon.fairwinds.com/restore=1585945609"
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
