Current issues:

* Why Orbit is clusterscoped and creates a namespace? Can it be namespace scoped and namespace created before dropping CR into cluster

* We need to add `cheetah` support

* We need to add a capsules for helm releases so we can add mirror, explorer and so on using https://github.com/fluxcd/helm-controller 
    * Do we want to use HelmReleaseCapsule or have separate capsules for each and make it use HelmReleaseCapsule type 

* We need to consider adding node count for the capsule so we have control around deployment - sequence, healthchecks and rollback

* Do we want to add a helm chart in between operator CRs and terraform/controlplane to hide CRs. It will describe hashsphere and include all the components in an opinionated way (?)

*





