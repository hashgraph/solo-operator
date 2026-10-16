#!/bin/sh
function setup() {
    helm repo add rustfs https://charts.rustfs.com
    helm repo add telemetry https://grafana.github.io/helm-charts
    helm repo update

    helm upgrade --install \
        --namespace rustfs \
        --create-namespace \
        --values setup/rustfs-values.yaml \
        rustfs rustfs/rustfs

    helm upgrade --install loki grafana/loki-stack \
      --namespace telemetry\
      --create-namespace \
      --values setup/loki-stack.yaml

    echo ">> Waiting 180s for Loki to be ready..."
    kubectl wait --for=condition=ready pod -l app=loki -n telemetry --timeout 180s || return $?

    echo ">> Waiting 180s for RustFS to be ready..."
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=rustfs -n rustfs --timeout 180s || return $?
}

function deploy() {
    # orbit
    kubectl apply -f common/orbit.yaml
    until kubectl get namespace solo-orbit &> /dev/null; do
        echo ">> Waiting for solo-orbit namespace to be ready..."
        sleep 5
    done

    # common
    kubectl apply -f common/configs.yaml
    kubectl apply -f common/secrets.yaml

    # node1
    kubectl apply -f node1/consensus-capsule.yaml
    kubectl apply -f node1/secrets.yaml

    # node2
    kubectl apply -f node2/consensus-capsule.yaml
    kubectl apply -f node2/secrets.yaml
    kubectl apply -f node2/pvcs.yaml

    # node3
    kubectl apply -f node3/consensus-capsule.yaml
    kubectl apply -f node3/secrets.yaml

    # node4
    kubectl apply -f node4/consensus-capsule.yaml
    kubectl apply -f node4/secrets.yaml
}

function destroy() {
    kubectl delete -f node4/consensus-capsule.yaml
    kubectl delete -f node4/secrets.yaml
    kubectl delete -f node3/consensus-capsule.yaml
    kubectl delete -f node3/secrets.yaml
    kubectl delete -f node2/consensus-capsule.yaml
    kubectl delete -f node2/secrets.yaml
    kubectl delete -f node2/pvcs.yaml
    kubectl delete -f node1/consensus-capsule.yaml
    kubectl delete -f node1/secrets.yaml
    kubectl delete -f common/configs.yaml
    kubectl delete -f common/secrets.yaml
    kubectl delete -f common/orbit.yaml
    kubectl delete namespace solo-orbit
}

function cleanup() {
    helm uninstall rustfs -n rustfs
    helm uninstall loki -n telemetry
    kubectl delete namespace rustfs
    kubectl delete namespace telemetry
}
