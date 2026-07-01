# Backups with Litestream

This template stores data in a single SQLite file (`<DB_DIR>/<ProjectName>.db`).
The backup story is [Litestream](https://litestream.io): it continuously
replicates the database's WAL to S3-compatible storage, giving offsite backups
with point-in-time restore and seconds-level RPO.

Litestream runs as a **separate process/container** — it cannot live inside the
app's `scratch` image. See `litestream.yml` and `docker-compose.yml` at the repo
root for a working sidecar example that shares the `/data` volume with the app.

## Setup

1. Create an S3 (or compatible: R2, MinIO, B2) bucket.
2. Edit `litestream.yml`: set `path` to `<DB_DIR>/<ProjectName>.db`, and fill in
   the bucket, path, and region.
3. Provide credentials via `LITESTREAM_ACCESS_KEY_ID` and
   `LITESTREAM_SECRET_ACCESS_KEY` (see `docker-compose.yml`).
4. Start both services: `docker compose up -d`.

## Restore

On a fresh volume, restore the database **before** the app starts (the app runs
migrations on an existing file, so restore first):

    litestream restore -config litestream.yml /data/riostarter.db

Then start the app. The compose file documents this restore-on-boot pattern in a
comment.

## Why not a built-in snapshot job?

A scheduled in-process `VACUUM INTO` would write to the same volume as the
database, so it would not survive the volume/host loss that is the real disaster,
and it would duplicate what Litestream already does better. Litestream is the
single backup story.
