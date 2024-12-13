#!/bin/sh
function setup() {
    helm repo add minio-operator https://operator.min.io/
    helm repo add telemetry https://grafana.github.io/helm-charts
    helm repo update

    helm install \
      --namespace minio-operator \
      --create-namespace \
      minio-operator minio-operator/operator

    helm install \
        --namespace minio-tenant \
        --create-namespace \
        --values minio-tenant/values.yaml \
        minio-tenant minio-operator/tenant

    helm install loki grafana/loki-stack \
      --namespace telemetry\
      --create-namespace \
      --values telemetry/loki-stack.yaml

    echo ">> Waiting 180s for Loki to be ready..."
    kubectl wait --for=condition=ready pod -l app=loki -n telemetry --timeout 180s || return $?

    echo ">> Waiting 180s for MinIO to be ready..."
    kubectl wait --for=condition=ready pod -l app=minio -n minio-tenant --timeout 180s || return $?
}

function deploy() {
    kubectl apply -f ./orbit/solo-orbit.yaml
    until kubectl get namespace solo-orbit &> /dev/null; do
        echo ">> Waiting for solo-orbit namespace to be ready..."
        sleep 5
    done
    kubectl apply -k .
}

function destroy() {
    kubectl delete -k .
    kubectl delete -f ./orbit/solo-orbit.yaml
}

function cleanup() {
    helm uninstall minio-operator -n minio-operator
    helm uninstall minio-tenant -n minio-tenant
    helm uninstall loki -n telemetry
    kubectl delete namespace minio-tenant
    kubectl delete namespace minio-operator
    kubectl delete namespace telemetry
}
