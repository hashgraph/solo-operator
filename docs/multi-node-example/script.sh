#!/bin/sh
function setup() {
    helm repo add minio-operator https://operator.min.io/
    helm install \
      --namespace minio-operator \
      --create-namespace \
      minio-operator minio-operator/operator
    helm install \
        --namespace minio-tenant \
        --create-namespace \
        --values minio-tenant/values.yaml \
        minio-tenant minio-operator/tenant

    kubectl apply -f ./orbit/orbit-cr.yaml
}

function deploy() {
    kubectl apply -k .
}

function destroy() {
    kubectl delete -k .
}

function cleanup() {
    kubectl delete -f ./orbit/orbit-cr.yaml
    helm uninstall minio-operator -n minio-operator
    helm uninstall minio-tenant -n minio-tenant
    kubectl delete namespace minio-tenant
    kubectl delete namespace minio-operator
}
