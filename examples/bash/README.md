# Gemini Example: bash timestamping
> Note: this is a work in progress. Adding this as a simple test case that illustrates the steps and components necessary to demonstrate a Gemini SnapshotGroup. This example creates a SnapshotGroup in a namespace where no previous PersistentVolumeClaims or PersistentVolumes exist; the SnapshotGroup generates a PVC based on the details specified in the SnapshotGroup's `spec.claim.spec`, and then starts creating VolumeSnapshots of that volume every 10 minutes. An example pod mounts the PVC. 

Before you can work with Gemini and Gemini's SnapshotGroups, you need to ensure that you have an appropriate CSI driver installed for your particular environment, as well as the CSI snapshotter and associated APIs. DigitalOcean clusters seem to have this included. Kops clusters do not.



## Gemini Installation on clusters that already have CSI configured

```
kubectl create ns gemini
helm install gemini deploy/charts/gemini --namespace gemini
```

## Gemini Installation on Kops 1.17+ clusters that do not have CSI configured

Run the provided `course.yaml` file with [Reckoner](https://github.com/fairwindsops/reckoner) to install all the prerequisites for Gemini (such as the EBS CSI Driver and external-snapshotter), as well as Gemini itself.

## Deploy the demo resources

Apply the provided Kubernetes manifests in the number in which they're ordered: `01-create-namespace.yaml`, `02-create-storage-class.yaml`, and so on. You'll be creating the temporary demo namespace, the StorageClass to be utilized by the volume, the VolumeSnapshotClass to be utilized by the VolumeSnapshots, the Gemini SnapshotGroup, and a test pod that will mount the example Volume. Soon after applying, you'll be able to see that gemini is successfully generating VolumeSnapshots for you!

```kubectl get volumesnapshot
NAME                            READYTOUSE   SOURCEPVC            SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS       SNAPSHOTCONTENT                                    CREATIONTIME   AGE
ebs-gemini-testing-1591205245   true         ebs-gemini-testing                           1Gi           aws-ebs-snapclass   snapcontent-96ab9a23-0d60-413d-af0f-b01d3880f3e7   83m            83m
```

Accordingly, you'll see the associated snapshots on your cloud provider of choice. For example, with AWS:

```
aws ec2 describe-snapshots --owner-ids <account-id>
{
    "Snapshots": [
        {
            "Description": "Created by AWS EBS CSI driver for volume <vol-id>",
            "Encrypted": false,
            "OwnerId": "<account-id>",
            "Progress": "100%",
            "SnapshotId": "snap-0224fb2ee42613b56",
            "StartTime": "2020-06-03T17:27:25.362Z",
            "State": "completed",
            "VolumeId": "<vol-id>",
            "VolumeSize": 1,
            "Tags": [
                {
                    "Key": "CSIVolumeSnapshotName",
                    "Value": "snapshot-96ab9a23-0d60-413d-af0f-b01d3880f3e7"
                }
            ]
        }
    ]
}
```
