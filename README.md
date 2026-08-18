# Kube Components Monorepo

This repository contains Kubernetes components, custom operators, and platform infrastructure manifests managed via a **GitOps** approach using **Argo CD** and the **App-of-Apps** pattern.

---

## Monorepo Architecture

```text
.
├── bootstrap/                          # Bootstrap Manifests (App-of-Apps)
│   └── root-app.yaml                   # Root Application registering all cluster apps
├── platform/                           # Platform component declarations (Argo CD Apps)
│   ├── argocd-config/                  # Internal Argo CD configurations
│   └── cost-optimizer/                 # GitOps deployment of the Cost Optimizer operator
│       └── manifests/                  # CRDs, RBAC, ServiceAccount, and Operator Deployment
└── operators/                          # Go Source Code for Custom Operators
│   └── cost-optimizer/                 # Cost Optimizer Operator (Kubebuilder / Operator SDK)
└── scripts/                            # Helper automation scripts
    └── install-argocd.sh               # Argo CD bootstrap script
```

## Prerequisites

Ensure you have the following tools installed and configured on your machine:

* Docker (or another container runtime)
* kubectl (configured to point to your target cluster)
* Operator SDK
* Go (v1.22+)

## Bootstrap Guide

Follow these steps to build the operator image, install Argo CD, and trigger automated deployment via GitOps.

### Build and Push the Operator Image
Build the cost-optimizer operator container image and push it to your container registry:

```bash
cd operators/cost-optimizer

# Build and push the Docker image
make docker-build docker-push IMG=<your-registry>/cost-optimizer-operator:v0.1.0

# Return to the monorepo root
cd ../..
```

### Install Argo CD in the Cluster

Run the bootstrap script to ensure Argo CD is installed and ready in your target cluster:

```bash
./scripts/install-argocd.sh
```

Wait until all Argo CD pods reach the Running state:

```bash
kubectl get pods -n argocd --watch
```

### Trigger the Bootstrap (App-of-Apps)

Apply the root Argo CD application manifest to register all applications in the platform/ directory:

```bash
kubectl apply -f bootstrap/root-app.yaml
```

Argo CD will detect the Root Application and automatically deploy:

* Internal Argo CD configuration (platform/argocd-config). 
* The cost-optimizer operator and its CRDs (platform/cost-optimizer).

## Operator Development Workflow

When modifying the cost-optimizer operator code, use these commands within operators/cost-optimizer/:

### Run Tests and Generate Code

```bash
# Run unit tests
make test

# Regenerate Go deepcopy code and CRD manifests after modifying api/v1alpha1/*_types.go
make generate
make manifests
```

### Apply CRDs and Run Controller Directly

Run the controller against your active Kubernetes cluster context:

```bash
# Install CRDs into the target cluster
make install

# Run the controller locally pointing to your active kubeconfig
make run
```

## Accessing the Argo CD Web UI

* Retrieve the initial admin password:

```bash
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d; echo
```

* Port-forward the UI server:

```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

* Open your browser at https://localhost:8080 and log in as admin

## Verifying the Cost Optimizer Operator

Once synced by Argo CD, test the reconciliation logic in your cluster:

```bash
# Create a test deployment with 3 replicas
kubectl create deployment nginx-test --image=nginx --replicas=3

# Apply a schedule sample
kubectl apply -f operators/cost-optimizer/config/samples/ops_v1alpha1_environmentschedule.yaml

# Inspect status and verify replicas scale to 0 if within shutdown window
kubectl get environmentschedules.ops.gouster.io
kubectl get deploy nginx-test
```