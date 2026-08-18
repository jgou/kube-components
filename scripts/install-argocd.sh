#!/usr/bin/env bash
set -euo pipefail

ARGOCD_VERSION="stable"
ARGOCD_MANIFEST_URL="https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml"

echo "=== Checking Argo CD installation ==="

if ! kubectl get namespace argocd >/dev/null 2>&1; then
  echo "Argo CD not found. Creating namespace and applying manifests..."
  kubectl create namespace argocd
  kubectl apply -n argocd -f "${ARGOCD_MANIFEST_URL}"
  echo "Waiting for Argo CD pods to be ready..."
  kubectl rollout status deployment/argocd-server -n argocd --timeout=180s
else
  echo "Argo CD namespace already exists. Skipping installation."
fi