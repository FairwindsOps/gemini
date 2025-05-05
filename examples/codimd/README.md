# Gemini Example: CodiMD
> Note: this will not work in a KIND cluster. It has been tested on DigitalOcean.

### Install the controller
```bash
helm repo add fairwinds-stable https://charts.fairwinds.com/stable
helm install gemini fairwinds-stable/gemini --namespace gemini --create-namespace
```

### Install CodiMD
```bash
helm repo add codimd https://helm.codimd.dev/
helm upgrade --install codimd codimd/codimd -n codimd --create-namespace --set codimd.imageStorePersistentVolume.enabled=false
```

This will create a PVC for the Postgres instance that drives CodiMD.

### Set up the Backup Schedule
```bash
cat <<EOF | kubectl apply -f -
apiVersion: gemini.fairwinds.com/v1beta1
kind: SnapshotGroup
metadata:
  name: codimd-postgresql
  namespace: codimd
spec:
  persistentVolumeClaim:
    claimName: data-codimd-postgresql-0
  schedule:
    - every: "10 minutes"
      keep: 3
    - every: hour
      keep: 1
EOF
```

within 30 seconds or so, you should see a `VolumeSnapshot`:
```bash
$ kubectl get volumesnapshot -n codimd
NAME                           READYTOUSE   SOURCEPVC                  SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS      SNAPSHOTCONTENT                                    CREATIONTIME   AGE
codimd-postgresql-1677262979   true         data-codimd-postgresql-0                           8Gi           do-block-storage   snapcontent-3aff5b51-e0c1-4154-bd93-4b396bf0c16e   12m            12m
```

### Create a document
```bash
kubectl port-forward svc/codimd 3000:80 -n codimd
```

Visit `localhost:3000` and sign up for an account. (You can use a dummy email. Make sure to click `Register` instead of hitting enter when first signing up.) Create a new note and enter some text.

### Trigger a backup
Rather than waiting for Gemini to create the next backup, you can delete existing
backups, and Gemini will create a new one.

```bash
kubectl delete volumesnapshot --all -n codimd
```

Within 30 seconds, you should see new snapshots appear. Make sure to wait until `READYTOUSE` is true
```bash
$ kubectl get volumesnapshot -n codimd
NAME                           READYTOUSE   SOURCEPVC                  SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS      SNAPSHOTCONTENT                                    CREATIONTIME   AGE
codimd-1594929516              true         codimd                                             2Gi           do-block-storage   snapcontent-e75421c6-c4ca-4bbf-81f4-a2fb0706b957   5s             7s
codimd-postgresql-1594929517   true         data-codimd-postgresql-0                           8Gi           do-block-storage   snapcontent-ad71c1f8-af7b-4cdc-85ba-e512a77095a3   4s             6s
```

### Edit your document again
```bash
kubectl port-forward svc/codimd 3000:80 -n codimd
```

Go back to your document (if you lost it, you can find it again by going to
`localhost:3000` and clicking `History`).

Delete what you wrote and replace it with something else. These changes will get reverted when we restore.

### Perform the restore
First, we need to scale down our deployment. We can't swap out a PVC in-place,
so you'll necessarily incur some downtime.

```bash
kubectl scale all --all --replicas=0 -n codimd
```

Next, annotate the `SnapshotGroup` with the timestamp of the snapshot you want.

For example, here we'll use timestamp `1585945609`.
```bash
$ kubectl get volumesnapshot -n codimd
NAME                           AGE
codimd-1585945609              15s
codimd-postgresql-1585945609   15s
```

```bash
kubectl annotate snapshotgroup/codimd-postgresql -n codimd --overwrite \
  "gemini.fairwinds.com/restore=1585945609"
```

This will:
* create a one-off backup of your PVC
* delete the PVC
* create a new PVC with the same name from your snapshot

Note: If your PVC gets stuck in `Terminating`, this might be related to rate-limiting from the DO API (check [this issue](https://github.com/FairwindsOps/gemini/issues/29) for more info) You can force destroy the PVC by running:

```bash
kubectl -n codimd patch pvc data-codimd-postgresql-0 -p '{"metadata":{"finalizers": []}}' --type=merge
```

Wait until you see that your PVC is back in `Bound` state.

Finally, we can scale back up:
```bash
kubectl scale all --all --replicas=1 -n codimd
```

### Verify the restore
```bash
kubectl port-forward svc/codimd 3000:80 -n codimd
```
Go back to your document. The second round of edits you made should be gone!
