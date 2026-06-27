# GitOps Deployment with Argo CD

This repository deploys a small Kubernetes application stack (an app + PostgreSQL + Redis)
using the **GitOps** model powered by [Argo CD](https://argo-cd.readthedocs.io/).

> This README documents **how the GitOps/Argo CD setup works** — how to install Argo CD,
> log in for the first time, deploy this stack, and operate it. It does **not** cover the
> application's own internals.

---

## Table of Contents

1. [What is GitOps here?](#what-is-gitops-here)
2. [Repository layout](#repository-layout)
3. [How it fits together](#how-it-fits-together)
4. [Prerequisites](#prerequisites)
5. [Step 1 — Start a cluster](#step-1--start-a-cluster)
6. [Step 2 — Install Argo CD](#step-2--install-argo-cd)
7. [Step 3 — First-time login & configuration](#step-3--first-time-login--configuration)
8. [Step 4 — Deploy the application (the Argo CD `Application`)](#step-4--deploy-the-application-the-argo-cd-application)
9. [The image-update flow (Kustomize + CI)](#the-image-update-flow-kustomize--ci)
10. [Day-to-day commands](#day-to-day-commands)
11. [Troubleshooting](#troubleshooting)

---

## What is GitOps here?

The core idea: **Git is the single source of truth for what runs in the cluster.**

- You never run `kubectl apply` against the app by hand.
- Argo CD continuously watches this Git repo and makes the cluster match it.
- To change what's deployed, you change a file in Git and commit — Argo CD does the rest.

This setup uses Argo CD's `automated` sync policy, so commits to the tracked branch are
reconciled automatically, with self-healing and pruning enabled.

---

## Repository layout

```
.
├── argocd/
│   └── application.yaml        # The Argo CD "Application" — points Argo CD at k8s/
├── k8s/
│   ├── kustomization.yaml      # Kustomize entrypoint (source of truth for resources + image tag)
│   ├── namespace.yaml          # appspace namespace + ResourceQuota
│   ├── pvc.yaml                # PersistentVolumeClaims (postgres, redis)
│   ├── postgres.yaml           # PostgreSQL Deployment/Service/Secret
│   ├── redis.yaml              # Redis Deployment/Service
│   ├── app/                    # The application Deployment/Service/Config/Secret
│   └── monitoring/             # (NOT deployed — intentionally excluded from kustomization)
└── .github/workflows/ci.yml    # Builds image, pushes to registry, auto-bumps the image tag
```

Key point: **`k8s/kustomization.yaml` decides what gets deployed.** Anything not listed in its
`resources:` (such as everything under `k8s/monitoring/`) is ignored by Argo CD.

---

## How it fits together

```
                  git push
   developer ───────────────► GitHub repo (this repo)
                                   │
                                   │ Argo CD polls / webhook
                                   ▼
                            ┌──────────────┐
                            │   Argo CD    │  (runs in the "argocd" namespace)
                            └──────┬───────┘
                                   │ kustomize build k8s/  → apply
                                   ▼
                            ┌──────────────┐
                            │  appspace ns │   app + postgres + redis
                            └──────────────┘
```

The `Application` object (`argocd/application.yaml`) is the glue:

```yaml
spec:
  source:
    repoURL: https://github.com/NerojuPavan/ArgoCD.git   # where to read manifests
    targetRevision: HEAD                                  # which branch/revision
    path: k8s                                             # which folder (uses Kustomize)
  destination:
    server: https://kubernetes.default.svc               # which cluster
    namespace: appspace                                   # which namespace
  syncPolicy:
    automated:
      prune: true       # delete resources removed from Git
      selfHeal: true    # revert manual cluster drift back to Git
    syncOptions:
      - CreateNamespace=true   # create "appspace" if it doesn't exist
```

Because `k8s/` contains a `kustomization.yaml`, Argo CD automatically runs Kustomize instead
of plain directory apply.

---

## Prerequisites

| Tool | Purpose |
|------|---------|
| `kubectl` | Talk to the cluster |
| A Kubernetes cluster | e.g. `minikube`, `kind`, or a managed cluster |
| `argocd` CLI (optional) | Log in / sync from the terminal |
| `git` | Push changes (GitOps) |

Verify:

```bash
kubectl version --client
kubectl cluster-info
```

---

## Step 1 — Start a cluster

For local testing with minikube:

```bash
minikube start --cpus=4 --memory=6g
kubectl get nodes
```

> The `appspace` namespace has a `ResourceQuota`, so give the cluster enough CPU/memory.

---

## Step 2 — Install Argo CD

Install Argo CD into its own `argocd` namespace:

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

Wait for the pods to be ready:

```bash
kubectl wait --for=condition=available --timeout=300s \
  deployment/argocd-server -n argocd
kubectl get pods -n argocd
```

---

## Step 3 — First-time login & configuration

### 3a. Get the initial admin password

Argo CD auto-generates an admin password and stores it in a secret:

```bash
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d; echo
```

The username is `admin`.

### 3b. Access the Argo CD UI / API

The `argocd-server` Service is `ClusterIP` by default. Expose it with a port-forward:

```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

Now open the UI at **https://localhost:8080**
(accept the self-signed certificate warning in your browser).

> Alternatives to port-forward:
> - `minikube`: `minikube service argocd-server -n argocd`
> - Patch the service to `NodePort` or `LoadBalancer`:
>   `kubectl -n argocd patch svc argocd-server -p '{"spec":{"type":"NodePort"}}'`

### 3c. Log in with the CLI (optional but recommended)

```bash
argocd login localhost:8080 \
  --username admin \
  --password '<password-from-step-3a>' \
  --insecure
```

### 3d. Change the admin password (do this first!)

```bash
argocd account update-password
```

After changing it, you can delete the initial secret:

```bash
kubectl -n argocd delete secret argocd-initial-admin-secret
```

### 3e. (If your repo is private) register repo credentials

Public repos need nothing. For a private repo:

```bash
argocd repo add https://github.com/NerojuPavan/ArgoCD.git \
  --username <git-user> --password <git-token>
```

---

## Step 4 — Deploy the application (the Argo CD `Application`)

With Argo CD running, apply the `Application` manifest. This tells Argo CD what to deploy:

```bash
kubectl apply -f argocd/application.yaml
```

Argo CD will now:

1. Read `k8s/` from the Git repo,
2. Run `kustomize build`,
3. Create the `appspace` namespace and apply all resources,
4. Keep them in sync automatically.

Check status:

```bash
# Via kubectl
kubectl get applications -n argocd
kubectl get pods -n appspace

# Via the CLI
argocd app get argocd-application
argocd app sync argocd-application      # force an immediate sync
```

When healthy you should see the app, `postgres`, and `redis` pods `Running` in `appspace`,
and the Application showing **Synced / Healthy** in the UI.

---

## The image-update flow (Kustomize + CI)

We do **not** hand-edit image tags. The flow is fully automated:

```
push to main
   └─► CI builds Docker image tagged with the commit short-SHA
        └─► pushes image to the registry (Docker Hub)
             └─► CI runs: kustomize edit set image <repo>=<repo>:<sha>
                  └─► CI commits the updated k8s/kustomization.yaml with "[skip ci]"
                       └─► Argo CD detects the change and rolls out the new image
```

The tag is managed in one place:

```yaml
# k8s/kustomization.yaml
images:
  - name: pavankumar5/test_app
    newTag: <commit-sha>   # CI updates this line automatically
```

Why a commit-SHA instead of `latest`?

- **Immutable & traceable** — every deploy maps to an exact commit.
- **Triggers a rollout** — Argo CD only redeploys when the manifest changes; `latest` never
  changes the manifest, so it wouldn't trigger anything.
- **Easy rollback** — `git revert` the bump commit and Argo CD rolls back.

> The `[skip ci]` marker on the auto-bump commit prevents the bot's commit from re-triggering
> the pipeline (no infinite loop). The CI job needs `contents: write` permission to push back.

---

## Day-to-day commands

```bash
# See the application and its health/sync state
argocd app get argocd-application
argocd app list

# Force a refresh (re-read Git) or a sync (apply)
argocd app refresh argocd-application
argocd app sync argocd-application

# Watch what's running
kubectl get all -n appspace
kubectl get applications -n argocd

# View live diff between Git and cluster
argocd app diff argocd-application

# Roll back to a previous synced revision
argocd app history argocd-application
argocd app rollback argocd-application <history-id>

# Re-open the UI
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

---

## Troubleshooting

| Symptom | Likely cause / fix |
|---------|--------------------|
| App shows **Synced** but a resource is missing | The resource isn't listed in `k8s/kustomization.yaml`. Kustomize only deploys what's referenced (this is why `monitoring/` is excluded). |
| Pod stuck in `ErrImageNeverPull` | `imagePullPolicy: Never` with no local image. Use a real registry tag + `IfNotPresent`/`Always`. |
| Pod stuck in `ImagePullBackOff` | The tag in `kustomization.yaml` doesn't exist in the registry, or the repo is private and needs `imagePullSecrets`. |
| `Application` won't parse / apply | YAML indentation error in `argocd/application.yaml`. Validate with `kubectl apply --dry-run=client -f argocd/application.yaml`. |
| Changes in Git not deploying | Check `argocd app get` for sync errors; run `argocd app refresh`. Confirm `targetRevision`/branch is correct. |
| `exceeded quota` events | The `ResourceQuota` in `namespace.yaml` is too small for the requested pods/CPU/memory. Adjust the quota or resource requests. |
| Forgot admin password | Reset by deleting the server pod after patching the password, or re-create `argocd-initial-admin-secret`. See Argo CD docs. |

---

## Useful references

- Argo CD docs: https://argo-cd.readthedocs.io/
- Argo CD getting started: https://argo-cd.readthedocs.io/en/stable/getting_started/
- Kustomize: https://kustomize.io/
