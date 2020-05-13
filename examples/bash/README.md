# Photon Example: bash timestamping
> Note: this is a work in progress. Adding this as a simple test case that illustrates the steps and components necessary to demonstrate a photon SnapshotGroup. 

Before you can work with Photon SnapshotGroups, you need to ensure that you have an appropriate CSI driver installed for your particular environment, as well as the CSI snapshotter and associated APIs. DigitalOcean clusters seem to have this included. Kops clusters do not.



### Installation on clusters that already have CSI configured

```
kubectl create ns photon
helm install photon deploy/charts/photon --namespace photon
```

### Installation on Kops 1.17+ clusters that do not have CSI configured

Run the provided `course.yaml` file with [Reckoner](https://github.com/fairwindsops/reckoner) to install all the prerequisites for Photon and Photon itself. 

### Deploy the test resources

Apply the provided Kubenretes manifests in the number in which they're ordered: `01-create-namespace.yaml`, `02-create-storage-class.yaml`, and so on. You'll be creating the temporary test namespace, the StorageClass to be utilized by the PVC, the PVC, the Pod that mounts the PVC, and then finally the Photon SnapshotGroup that will operate in this namespace. 