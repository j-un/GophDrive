# Disaster Recovery

This runbook covers how to recover GophDrive note data when something has gone
wrong. It is written so a future operator (or AI agent) can act without
prior context.

## What's protected, and how

| Layer                        | Mechanism                  | Retention              |
| ---------------------------- | -------------------------- | ---------------------- |
| FileStore (notes/folders)    | DynamoDB PITR              | 35 days, any second    |
| FileStore (CFN delete)       | `RemovalPolicy.RETAIN`     | Survives `cdk destroy` |
| User-driven full backup      | `/api/export` (ZIP)        | As often as run        |
| EditingSessions (locks)      | None (intentionally)       | TTL'd, ephemeral       |

PITR is the **first** thing to reach for. The ZIP export is the **last
resort** — it bypasses AWS entirely (works even if the table is gone, the
account is locked, or you're rebuilding from scratch).

## Resolving the live table name

CloudFormation generates the FileStore table name; commands below assume
you've exported it as `$T`. Get it from the deployed stack:

```bash
T=$(aws cloudformation describe-stacks \
  --stack-name GophDriveDatabaseStack \
  --query "Stacks[0].Outputs[?OutputKey=='FileStoreTableName'].OutputValue" \
  --output text)
echo "$T"
```

For local dev (DynamoDB Local) the table is just `FileStore`.

## Verifying PITR is on

```bash
aws dynamodb describe-continuous-backups \
  --table-name "$T" \
  --query 'ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus'
```

Expected: `"ENABLED"`. If it returns `"DISABLED"`, redeploy GophDriveDatabaseStack
or run:

```bash
aws dynamodb update-continuous-backups \
  --table-name "$T" \
  --point-in-time-recovery-specification PointInTimeRecoveryEnabled=true
```

## Scenario 1: a single note was deleted or corrupted

If you can identify the moment things went wrong (deploy time, a stray
`DeleteFile` call, a bad migration), restore the table to a new name and
copy the affected rows back.

```bash
aws dynamodb restore-table-to-point-in-time \
  --source-table-name "$T" \
  --target-table-name "$T-restore-$(date +%Y%m%d-%H%M)" \
  --restore-date-time 2026-05-11T08:30:00Z
```

Then either:

- **Cherry-pick rows** — `aws dynamodb get-item` from the restored table,
  `put-item` into the live table. Cleanest for one or two items.
- **Swap tables** — only if the live table is fully corrupted. Update the
  `FILE_STORE_TABLE` env var on the Lambda to the restored name, verify,
  then rename properly via a CDK change.

Once the recovery is confirmed, delete the restored table to stop billing
for it.

## Scenario 2: the FileStore table was dropped

`RemovalPolicy.RETAIN` should make this hard, but if it happens:

1. Restore from PITR using the procedure above into a *new* table.
2. Re-create FileStore via CDK (`cdk deploy` will refuse to overwrite the
   restored table; rename or delete it first as appropriate).
3. Migrate rows by scanning the restored table and writing to FileStore.

If PITR retention has lapsed (>35 days), fall back to the most recent ZIP
export — see scenario 3.

## Scenario 3: cold restore from a ZIP export

This path needs no AWS state — it's the offline fallback when an account is
unreachable or the table is permanently gone.

1. Bring the stack back up (`./scripts/deploy-aws.sh` or equivalent).
2. Log in normally; the auto-mint flow recreates the user's base folder.
3. There is **no bulk import endpoint today**. Notes from a ZIP must be
   re-created manually (`POST /api/notes` per file). Acceptable at the
   current scale (single user, <1000 notes); revisit if data volume grows.

The user-facing "Export all notes (.zip)" button under `/settings` produces
a ZIP whose entries mirror the in-app folder hierarchy. Mtime is preserved
on each entry; folder structure is implicit in the path.

## Pre-incident checklist

- [ ] PITR is `ENABLED` on FileStore (run the verify command above).
- [ ] Most recent ZIP export is no older than your tolerance (e.g. weekly).
  Today this is a manual click; if it becomes a chore, see follow-ups below.
- [ ] CDK `RemovalPolicy.RETAIN` is set in `database-stack.ts` for FileStore.

## Phase 6 follow-up (deferred)

A weekly automated DDB→S3 export was scoped but explicitly deferred: the
manual ZIP export covers the same ground at the current single-user scale,
and a self-service automation would create an untested code path. If manual
exports become tedious, or the user count grows, revisit:

- EventBridge schedule rule (`rate(7 days)`)
- DynamoDB native S3 export to a versioned + lifecycle-managed bucket
- A bulk-import endpoint that consumes a ZIP, eliminating scenario 3 step 3
- Update this doc with the restore-from-S3-export procedure
