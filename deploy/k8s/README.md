# Deploying deuce to GKE

Manifests for the API. The number prefixes are the apply order: the namespace
before anything in it, Redis before the API that needs it, the Ingress last
because it points at a Service.

    kubectl apply -f deploy/k8s/

The Secret is not here on purpose — see deploy/examples/db-url.yaml. Applying
this directory should never create one with a placeholder password in it.

Placeholders to replace: `IMAGE`, `MIGRATE_IMAGE`, `INSTANCE_CONNECTION_NAME`,
`DOMAIN`, and the service account annotation in `00-namespace.yaml`.

## What runs where

| | |
|---|---|
| API | Deployment, 2 replicas, behind a GKE Ingress |
| Postgres | Cloud SQL, reached through the Auth Proxy sidecar |
| Redis | in the cluster, one replica, no persistence |
| Migrations | a Job, run before the API rolls out |

Redis is not on Memorystore on purpose. `SessionCache` reads through to Postgres
on any miss or error, so losing it costs latency rather than data, and
Memorystore's smallest tier costs more per month than everything else here put
together. Move it if sessions ever stop being the only thing cached.

## Once, per project

```bash
gcloud sql instances create deuce --database-version=POSTGRES_16 \
  --tier=db-f1-micro --region=asia-southeast1
gcloud sql databases create deuce --instance=deuce
gcloud sql users create deuce --instance=deuce --password=...

gcloud compute addresses create deuce --global
```

Workload Identity, so the proxy can reach Cloud SQL without a key file:

```bash
gcloud iam service-accounts create deuce
gcloud projects add-iam-policy-binding PROJECT \
  --member=serviceAccount:deuce@PROJECT.iam.gserviceaccount.com \
  --role=roles/cloudsql.client
gcloud iam service-accounts add-iam-policy-binding \
  deuce@PROJECT.iam.gserviceaccount.com \
  --role=roles/iam.workloadIdentityUser \
  --member="serviceAccount:PROJECT.svc.id.goog[deuce/deuce]"
```

## Every deploy

```bash
docker build -t $IMAGE . && docker push $IMAGE
docker build -f Dockerfile.migrate -t $MIGRATE_IMAGE . && docker push $MIGRATE_IMAGE

kubectl apply -f deploy/k8s/00-namespace.yaml
kubectl -n deuce create secret generic deuce --from-literal=DB_URL='...'

kubectl apply -f deploy/k8s/20-redis.yaml
kubectl replace --force -f deploy/k8s/40-migrate-job.yaml
kubectl -n deuce wait --for=condition=complete job/migrate --timeout=120s
kubectl apply -f deploy/k8s/30-api.yaml
kubectl apply -f deploy/k8s/50-ingress.yaml
```

Migrations run as a Job rather than at API startup, where every replica would
race for them. `kubectl replace --force` because a completed Job's pod template
is immutable.

## Known gaps

The certificate takes up to fifteen minutes to provision, and the Ingress
serves 502s until the backend passes its health check — both are normal on the
first apply and alarming if you do not expect them.

`/healthz` answers without touching Postgres, so the readiness probe reports
that the process is up rather than that it can serve. A pod whose database has
gone away stays in the Service until the pool errors out. A probe that checked
the pool would be better.

There is nothing here for the web app. It builds to static files and wants a
bucket and a CDN rather than a container and a pod.
