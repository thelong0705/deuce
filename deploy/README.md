# Deploying deuce

`gke.yaml` is the whole deployment. `STEPS.md` walks through deploying it from
nothing, with a note on what each command is for. This file is the shape of it:
what runs where, what it costs, and what is still missing.

## What runs where

| | |
|---|---|
| API and sweeper | GKE Autopilot, one image, different commands |
| Postgres | AlloyDB, private IP in the VPC |
| Redis | Memorystore, private IP in the VPC |
| Migrations | a Job, before the API rolls out |

Nothing connects through a proxy. AlloyDB and Memorystore both hand out private
addresses on the VPC, and an Autopilot cluster on the same network reaches them
directly — so `DB_URL` and `REDIS_ADDR` are addresses and that is the whole of
it. No Auth Proxy sidecar, no Workload Identity, no service account.

Worth knowing, because most of the GKE documentation shows the proxy. It buys
IAM authentication instead of a password, which is the better answer for
something real. It is not needed to connect.

## What it costs

The part to decide before running it.

| | |
|---|---|
| AlloyDB | smallest instance is 2 vCPU, on the order of **$100/month** |
| Memorystore | 1 GB basic, on the order of **$35/month** |
| GKE Autopilot | what the pods request, about **$20/month** here |
| Load balancer | about **$18/month** |

Call it **$170 a month**, so $300 of credit is about seven weeks. Check the
pricing calculator rather than trusting these: they are the right order of
magnitude, not a quote.

AlloyDB has no small tier — 2 vCPU is the floor. Cloud SQL's `db-f1-micro` is
around $10 a month and speaks the same protocol, so if the point is having the
thing running rather than having AlloyDB specifically, that swap is one line in
the Secret.

Delete all of it when you have finished looking:

    gcloud container clusters delete deuce --region=asia-southeast1
    gcloud alloydb clusters delete deuce --region=asia-southeast1 --force
    gcloud redis instances delete deuce --region=asia-southeast1

## What is still missing

**There is no HTTPS.** A `LoadBalancer` Service gives out a bare IP and speaks
plain HTTP. The session cookie is `Secure`, so a browser will not store it over
that IP: sign-in will appear to succeed and every request after it will look
signed out. The API answers; the web app does not work. Fixing it needs a
domain, an Ingress and a `ManagedCertificate`.

**The migration Job runs once.** `kubectl apply` will not re-run it, because a
completed Job is immutable. Use `kubectl replace --force -f` when the migrations
change.

**Nothing serves the web app.** It builds to static files, which want a bucket
and a CDN rather than a pod.

## If something looks wrong

`kubectl logs job/migrate` and `kubectl logs -l app=deuce` say why.

`connection refused` or a timeout means the pods cannot see the private
addresses. The usual cause is the GKE cluster sitting on a different network
from the peering in step 1.

`extension "citext" is not available` would mean AlloyDB will not create it. The
first migration needs it, so it would be the first thing to fail. AlloyDB
carries the standard Postgres extensions, so it should not happen — but it is
unverified here, and it is where I would look first.
