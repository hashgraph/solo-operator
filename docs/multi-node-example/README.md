# Deploy a test network with 3 nodes

## Prerequisites

You will need a kubernetes cluster with the operator running. 

``` 
# Create cluster
$ kind create cluster solo

# In another terminal go to the project root and run
$ quarkus dev
```
For example by running `kind` and `quarkus dev`.

## Setup cluster 

```
$ source script.sh && setup
```

## Deploy Network

```
$ source script.sh && deploy 
```

## Destroy Network

```
$ source script.sh && destroy
```

## Cleanup

```
$ source script.sh && cleanup
```
