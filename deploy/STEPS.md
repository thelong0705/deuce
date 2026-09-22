# Deploying deuce to GKE, step by step

Written for someone doing this the first time. Every step says what it is for,
because most of these commands are not guessable and the errors when you skip
one rarely point back at what you missed.

Run the whole thing in **one terminal session** — several steps set shell
variables that later steps use. If you close it, re-export `PGPASS`,
`ALLOYDB_IP`, `REDIS_IP` and `REPO` before step 9.

Roughly 40 minutes, most of it waiting for AlloyDB.

---

## 0. gcloud

```bash
brew install --cask google-cloud-sdk
```

```bash
gcloud init
```

Signs you in, makes or picks a project, sets it as the default so later commands
do not need `--project`. Link a billing account in the console afterwards
(**Billing → Link a billing account**) or everything below fails on quota.

## 1. Turn the APIs on

```bash
gcloud services enable container.googleapis.com artifactregistry.googleapis.com alloydb.googleapis.com redis.googleapis.com servicenetworking.googleapis.com
```

Every Google service is off in a new project. This switches on the five we use:
GKE, the image registry, AlloyDB, Memorystore, and the service networking that
step 3 needs.

## 2. Pick a database password

```bash
export PGPASS=$(openssl rand -hex 24) && echo "$PGPASS"
```

Keep what it prints. Step 4 sets it on AlloyDB and step 9 puts it in a
connection URL.

Hex rather than base64 on purpose: base64 emits `/` and `+`, and a `/` in a
`postgres://` URL makes the parser read the password as part of the address.
The error it gives says *invalid port*, which is not a fun hour.

## 3. Reserve a private range

```bash
gcloud compute addresses create google-managed --global --purpose=VPC_PEERING --prefix-length=16 --network=default
```

```bash
gcloud services vpc-peerings connect --service=servicenetworking.googleapis.com --ranges=google-managed --network=default
```

AlloyDB and Memorystore do not run inside your network — they run in a
Google-owned one, peered to yours. For that to work you have to set aside a
slice of your own address space for their instances to live in. The first
command reserves it, the second connects the two networks.

One-time per VPC, and free: reserved *external* IPs are billed when idle,
internal peering ranges are not.

Skip it and AlloyDB creation fails, because there is nowhere to put its IP.

## 4. AlloyDB — about 10 minutes

```bash
gcloud alloydb clusters create deuce --region=asia-southeast1 --network=default --password="$PGPASS"
```

```bash
gcloud alloydb instances create deuce --cluster=deuce --region=asia-southeast1 --instance-type=PRIMARY --cpu-count=2
```

```bash
export ALLOYDB_IP=$(gcloud alloydb instances describe deuce --cluster=deuce --region=asia-southeast1 --format='value(ipAddress)') && echo "$ALLOYDB_IP"
```

Two objects, not one: the *cluster* holds the data and the *instance* is the
machine that serves it. 2 vCPU is the floor — AlloyDB has no small tier.

The third command prints the private IP. Nothing outside the VPC can reach it,
which is the point.

## 5. Memorystore — about 5 minutes

```bash
gcloud redis instances create deuce --region=asia-southeast1 --size=1 --network=default --connect-mode=PRIVATE_SERVICE_ACCESS
```

```bash
export REDIS_IP=$(gcloud redis instances describe deuce --region=asia-southeast1 --format='value(host)') && echo "$REDIS_IP"
```

`--connect-mode=PRIVATE_SERVICE_ACCESS` reuses the range from step 3 rather than
asking for another one.

Sessions are cached here and read through to Postgres when missing, so this
holds nothing that cannot be rebuilt.

## 6. The cluster — about 5 minutes

```bash
gcloud container clusters create-auto deuce --region=asia-southeast1 --network=default
```

Autopilot means Google runs the nodes: you ask for pods, it finds machines for
them, and you are billed for what the pods request rather than for VMs.

`--network=default` matters. On another network the pods cannot see AlloyDB or
Memorystore, and the failure looks like a timeout with nothing to suggest why.

This also points `kubectl` at the cluster.

## 7. The kubectl auth plugin

```bash
gcloud components install gke-gcloud-auth-plugin
```

```bash
gcloud container clusters get-credentials deuce --region=asia-southeast1
```

```bash
kubectl get nodes
```

`kubectl` 1.26 and later need a separate plugin to authenticate to GKE, and
nothing warns you until the first command fails. The second rewrites your
kubeconfig to use it.

`No resources found` from the third is success — Autopilot has no nodes until
something asks for capacity. An error mentioning the plugin is not.

## 8. Build and push the image

```bash
gcloud artifacts repositories create deuce --repository-format=docker --location=asia-southeast1
```

```bash
gcloud auth configure-docker asia-southeast1-docker.pkg.dev
```

```bash
export REPO=asia-southeast1-docker.pkg.dev/$(gcloud config get project)/deuce
```

```bash
docker build --platform linux/amd64 -t $REPO/api:v1 . && docker push $REPO/api:v1
```

A registry the cluster can pull from, then Docker taught to authenticate to it.

`--platform linux/amd64` is not optional on an Apple Silicon Mac. Without it you
build an ARM image, the x86 nodes cannot run it, and the pods fail with `exec
format error` — which reads like a corrupt binary rather than a wrong
architecture.

One image holds both the API and the sweeper; they run different commands.

## 9. Tell the cluster where things are

```bash
kubectl create secret generic deuce --from-literal=DB_URL="postgres://postgres:$PGPASS@$ALLOYDB_IP:5432/postgres?sslmode=require" --from-literal=REDIS_ADDR="$REDIS_IP:6379"
```

```bash
kubectl create configmap migrations --from-file=db/postgres/migration
```

The two addresses and the password, as a Secret so they are not in git. The
migrations go in as a ConfigMap so the Job in step 10 can run them without
another image to build.

`REDIS_ADDR` is not secret; it shares the Secret because one object is one
command.

## 10. Deploy

```bash
sed "s|IMAGE|$REPO/api:v1|" deploy/gke.yaml | kubectl apply -f -
```

```bash
kubectl get pods --watch
```

`sed` fills in the image tag; `deploy/gke.yaml` ships with `IMAGE` as a
placeholder so nothing in git points at your project.

**Expect a bad first minute.** The migration Job runs before the API is ready
and the API restarts a few times while it waits. `CrashLoopBackOff` at the start
is normal. Ctrl-C when the two `deuce-` pods read `1/1` and `migrate` reads
`Completed`.

Migrations run as a Job rather than on API startup, where two replicas would
both try at once.

## 11. Your public address

```bash
kubectl get service deuce --watch
```

`EXTERNAL-IP` is `<pending>` for a minute or two while Google assigns a load
balancer. Then:

    http://THAT-IP/healthz  →  {"status":"ok"}

**Signing in from a browser will not work.** The session cookie is marked
`Secure`, so no browser will store it over plain HTTP on a bare IP — sign-in
looks like it succeeds and every request after it looks signed out. The API is
fine; the web app needs a domain and HTTPS, which is a separate piece of work.

## 12. Delete it when you are done

```bash
gcloud container clusters delete deuce --region=asia-southeast1
```

```bash
gcloud alloydb clusters delete deuce --region=asia-southeast1 --force
```

```bash
gcloud redis instances delete deuce --region=asia-southeast1
```

Around **$170 a month** running — AlloyDB about $100, Memorystore about $35,
the cluster about $20, the load balancer about $18. Free credit of $300 is
roughly seven weeks of that, and it runs whether or not you are looking at it.

---

## When something fails

`kubectl logs job/migrate` and `kubectl logs -l app=deuce` say why.

**`connection refused` or a timeout** — the pods cannot see the private
addresses. Usually the cluster is on a different network from the peering in
step 3.

**`password authentication failed`** — the URL in step 9 does not match what
AlloyDB has. Reset it rather than guessing:

```bash
gcloud alloydb users set-password postgres --cluster=deuce --region=asia-southeast1 --password="$PGPASS"
```

then delete and recreate the Secret.

**`invalid port`** in a connection error — a `/` or `+` in the password. Step 2
explains; generate a hex one.

**`exec format error`** — the image is ARM. Rebuild with `--platform
linux/amd64`.

**`extension "citext" is not available`** — the first migration creates it and
AlloyDB would not. This should not happen, but it is unverified, and it is
where I would look first if step 10 fails on the migration rather than on the
connection.
