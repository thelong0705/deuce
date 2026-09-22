# Deploying deuce

`gke.yaml` is the whole deployment, and its header has the commands. This file
is the part that does not fit in a comment: what it is not, and what to change
before it is anything more than a demo.

## What it gives you

A public IP serving the API over HTTP, with Postgres and Redis beside it in the
cluster and the sweeper releasing unpaid holds. Good enough to show someone.
Not production.

The API and the sweeper are the same image, run with different commands. One
build, one push, one tag — and `./cmd/...` in the Dockerfile means a third
binary needs no change to the build.

## What to change first

**Postgres is in the cluster.** One replica, one disk. A node upgrade takes the
database down with it, and there are no backups. Cloud SQL costs about ten
dollars a month and fixes both — the app reaches it through the Cloud SQL Auth
Proxy as a second container in the API pod, and `DB_URL` then points at
`127.0.0.1` so the database is never on the cluster network at all.

**There is no HTTPS.** A `LoadBalancer` Service gives out a bare IP and speaks
plain HTTP, which means the session cookie is sent in the clear. The cookie is
already marked `Secure`, so browsers will refuse to store it over that IP —
sign-in will appear to work and every request after it will look signed out.
Fixing that needs a domain, an Ingress and a `ManagedCertificate`.

That is the one thing that makes this a demo rather than a deployment: it is
enough to see the API answer, and not enough to actually sign in from a
browser.

**Redis is a single pod with no persistence.** That one is fine. Sessions are
cached there and read through to Postgres when missing, so a restart costs
latency rather than anyone's session.

**The migration Job runs once.** `kubectl apply` will not re-run it on the next
deploy, because a completed Job is immutable. Use
`kubectl replace --force -f` when the migrations change.

**Nothing serves the web app.** It builds to static files, which want a bucket
and a CDN rather than a pod.

## If something looks wrong

On the first apply the API and the migration Job fail a few times while
Postgres starts, then settle. `kubectl get pods` showing `CrashLoopBackOff` for
the first minute is expected.

`kubectl logs job/migrate` and `kubectl logs -l app=deuce` say why if it does
not settle.

`password authentication failed for user "deuce"` means the disk outlived the
password. Postgres only reads `POSTGRES_PASSWORD` when it first initialises its
data directory, so changing the Secret afterwards changes nothing — the
database still wants the old one. Either put the old password back, or delete
the claim and start over:

    kubectl delete deployment postgres && kubectl delete pvc postgres

## Costs

An Autopilot cluster bills for what the pods request. As written, about
900 mCPU and 1 GiB, which lands near twenty to thirty dollars a month, plus a
few dollars for the load balancer and the disk. `gcloud container clusters
delete deuce` stops all of it.
