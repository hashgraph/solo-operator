#!/bin/sh
function setup() {
    helm repo add minio https://operator.min.io/
    helm install \
      --namespace minio-operator \
      --create-namespace \
      minio-operator minio/operator
    kubectl apply -k minio-tenant/solo-minio-tiny
}

function deploy() {
    kubectl apply -k .
}

function destroy() {
    kubectl delete -k .
}

function cleanup() {
    kubectl delete -k minio-tenant/solo-minio-tiny
    helm uninstall minio-operator -n minio-operator
    kubectl delete namespace minio-operator
}
