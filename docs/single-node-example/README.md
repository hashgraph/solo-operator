# Deploy a single node cluster.

## Prerequisites

You will need a kubernetes cluster with the operator running. For example by running `kind` and `quarkus dev`.

## Deploy manifests

```
$ kubectl apply -f orbit.yml
orbit.operator.solo.hedera.com/consensuscapsuletest created

$ kubectl apply -f cluster-config.yml
configmap/cluster-config created

$ kubectl apply -f public-keys.yml
configmap/public-keys created

$ kubectl apply -f consensus-capsule.yml
consensuscapsule.operator.solo.hedera.com/node0 created

$ kubectl apply -f network-inistialization.yml
networkinitialization.operator.solo.hedera.com/network-initialization created

$ kubectl -n consensuscapsuletest get all
NAME                                READY   STATUS    RESTARTS   AGE
pod/node0-deploy-84cfc84bcc-bmvwr   1/1     Running   0          11s

NAME                TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)           AGE
service/node0-svc   LoadBalancer   10.96.147.15   10.89.0.201   50111:31936/TCP   18s

NAME                           READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/node0-deploy   1/1     1            1           18s

NAME                                      DESIRED   CURRENT   READY   AGE
replicaset.apps/node0-deploy-84cfc84bcc   1         1         1       18s
```

## Cleanup

```
$ kubectl delete orbits.operator.solo.hedera.com consensuscapsuletest
orbit.operator.solo.hedera.com "consensuscapsuletest" deleted
```
