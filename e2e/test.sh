#! /bin/bash
set -eo pipefail

ls -lah /project

helm install /project/deploy/charts/photon --namespace photon

kubectl apply -f /project/e2e/snapshotgroup.yaml
kubectl get pvc
kubectl apply -f /project/e2e/pod.yaml
sleep 10
kubectl get po
pod_name=$(kubectl get po -oname)

# first text - 'hello world'
kubectl exec $pod_name -- /bin/bash -c 'echo -ne "hello world" > /files/hello.txt'
text=$(kubectl exec $pod_name -- /bin/bash -c 'cat /files/hello.txt')
if [ $text != "hello world" ]; then
  echo "Expected 'hello world', but got $text"
  exit 1
fi

# backup the first text
kubectl delete volumesnapshot --all
sleep 35

# modify the text to 'hello world!!!'
kubectl exec $pod_name -- /bin/bash -c 'echo -ne "!!!" >> /files/hello.txt'
text=$(kubectl exec $pod_name -- /bin/bash -c 'cat /files/hello.txt')
if [ $text != "hello world!!!" ]; then
  echo "Expected 'hello world!!!', but got $text"
  exit 1
fi

# restore
kubectl delete po $pod_name
backup_time=$(kubectl get volumesnapshot -oname | head -n 1 | sed 's/.*-\([[:digit:]]\+\)$/\1/')
kubectl annotate snapshotgroup/my-files --overwrite "photon.fairwinds.com/restore=$backup_time"
kubectl apply -f /project/e2e/pod.yaml
sleep 10
kubectl get po
pod_name=$(kubectl get po -oname)

# check that first text is back
text=$(kubectl exec $pod_name -- /bin/bash -c 'cat /files/hello.txt')
if [ $text != "hello world" ]; then
  echo "Expected 'hello world', but got $text"
  exit 1
fi

