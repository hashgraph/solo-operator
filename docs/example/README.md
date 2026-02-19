# Deploy a test network with 3 nodes

## Prerequisites
- create cluster
``` 
$ kind create cluster solo
```

- start solo operator ( in another terminal ) 
```
# In another terminal go to the project root and run
$ task dev
```

## Demo
- **setup:** deploy minIO and solo orbit

```
$ source script.sh && setup
```

- **deploy:** deploy network in the orbit

```
$ source script.sh && deploy 
```

- **destroy:** destroy network in the orbit

```
$ source script.sh && destroy
```

- **cleanup:** uninstall minIO and solo orbit

```
$ source script.sh && cleanup
```
