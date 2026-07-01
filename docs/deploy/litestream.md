# Backups with Litestream (Dokku host daemon)

This template stores data in a single SQLite file (`<DB_DIR>/<ProjectName>.db`).
The backup story is [Litestream](https://litestream.io): it continuously
replicates the database's WAL to S3-compatible storage, giving offsite backups
with point-in-time restore and seconds-level RPO.

The target deployment is a single DigitalOcean droplet running
[Dokku](https://dokku.com), hosting several projects as containers that each
keep their SQLite file on Dokku persistent storage. In that setup the cleanest
place for Litestream is **one host-level daemon** on the droplet — a systemd
service that watches every project's database file and replicates each to a
Cloudflare R2 bucket.

Nothing about Litestream lives in the app image or the app code. The `scratch`
image stays untouched; the daemon reads the SQLite files directly off the host
disk, out-of-band from the containers.

## Why a host daemon (not a sidecar)

Dokku is built around one main container per app, so the alternatives scale
badly across many projects:

- **A Litestream sidecar per app** would mean an extra Dokku app plus a
  shared-host-path mount for every project.
- **Litestream inside each container** (via `litestream replicate -exec`) would
  force every project off its minimal `scratch` image onto one bundling the
  Litestream binary.

All Dokku persistent storage lives under one predictable tree
(`/var/lib/dokku/data/storage/<app>/…`), so a single host process can see every
database. Adding a new project is a few lines in one config file.

## Setup

**1. Mount each app's DB directory to host storage.** The app writes
`<DB_DIR>/<ProjectName>.db`; map `/data` in the container to a host directory:

    dokku storage:ensure-directory myapp
    dokku storage:mount myapp /var/lib/dokku/data/storage/myapp:/data

The database then lives at
`/var/lib/dokku/data/storage/myapp/<ProjectName>.db` on the host.

**2. Install Litestream on the droplet** (on the host, not in a container). The
`.deb` package ships a `litestream.service` systemd unit that reads
`/etc/litestream.yml`:

    wget https://github.com/benbjohnson/litestream/releases/latest/download/litestream-<ver>-linux-amd64.deb
    dpkg -i litestream-*.deb

**3. Configure `/etc/litestream.yml`** with one `dbs:` entry per project, each
replicating to R2 (S3-compatible, so `type: s3` with an `endpoint`). Give each
app a distinct `path:` prefix so replicas don't collide in the bucket:

    dbs:
      - path: /var/lib/dokku/data/storage/myapp/riostarter.db
        replicas:
          - type: s3
            bucket: my-r2-bucket
            path: myapp
            endpoint: https://<ACCOUNT_ID>.r2.cloudflarestorage.com
            region: auto

      - path: /var/lib/dokku/data/storage/otherapp/other.db
        replicas:
          - type: s3
            bucket: my-r2-bucket
            path: otherapp
            endpoint: https://<ACCOUNT_ID>.r2.cloudflarestorage.com
            region: auto

**4. Provide R2 credentials via a systemd env file**, keeping secrets out of the
yaml. Create an R2 API token (S3 access key + secret) in the Cloudflare
dashboard, then write `/etc/litestream.env` (`chmod 600`):

    LITESTREAM_ACCESS_KEY_ID=<r2-access-key-id>
    LITESTREAM_SECRET_ACCESS_KEY=<r2-secret-access-key>

Wire it in with a drop-in via `systemctl edit litestream`:

    [Service]
    EnvironmentFile=/etc/litestream.env

Litestream reads those two env vars automatically.

**5. Enable the daemon:**

    systemctl enable --now litestream
    systemctl status litestream      # confirm it found each db and is replicating

## Restore

On a fresh droplet (or disaster recovery), restore **before** the app's first
boot — the app runs migrations against an existing file, so restore first, per
app:

    litestream restore -config /etc/litestream.yml \
      /var/lib/dokku/data/storage/myapp/riostarter.db

Then start the app (`dokku ps:start myapp`). On an existing droplet the file is
already present, so no restore is needed — the daemon just resumes.

## Deploys

`git push dokku` / `dokku ps:rebuild` stops and replaces the app container, but
Litestream watches the **host file**, not the container, so replication keeps
running across deploys. The only coordination point is a brand-new app: create
the storage mount and, if restoring, run the restore before the first start.

## Local dev

You do not need Litestream in local development — it is purely a production
backup concern. The app opens and migrates its SQLite file with or without
Litestream present, so just run the binary against a local database.

## Why not a built-in snapshot job?

A scheduled in-process `VACUUM INTO` would write to the same disk as the
database, so it would not survive the droplet/disk loss that is the real
disaster, and it would duplicate what Litestream already does better. Litestream
is the single backup story.
