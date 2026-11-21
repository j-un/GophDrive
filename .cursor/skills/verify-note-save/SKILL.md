---
name: verify-note-save
description: Check that two DynamoDB saves of one GophDrive note cannot both commit against the same ETag. Use when a change touches Adapter.SaveFile, Adapter.GetFile, NoteHandler.UpdateNote, If-Match, or the note etag.
---

# Verify note save

A save that names an ETag may commit only when that ETag is the one its own read returned, and two such saves may not both commit against the same stored ETag. If the second promise breaks, both clients are told the save worked and one writer's text is gone. The conflict dialog never opens.

The check is a TLA+ model of the production DynamoDB path, run with TLC 2.19 (tla2tools 1.7.4) on Java 25. Ordering of the read and the unconditional write is the question, so a model checker is the tool. No verifier in this repo's toolchain checks Go. The in-memory `saveFileMap` is a different function: it holds `mu` across the check and the write, and the unit tests call that path, so they do not reach this interleaving.

## Run

From the repo root:

```bash
.cursor/skills/verify-note-save/scripts/check.sh
```

The first run needs the network. `mise` installs `java@temurin-25` and the TLC 1.7.4 jar. Later runs use that cache.

Exit codes from this TLC:

- `0` — every invariant holds
- `12` — an invariant failed. The output names it. This is a result, not a crashed tool
- `11` — deadlock
- `150`, `151`, or `255` — TLC could not check the spec. Not a finding about the note

Read every result with `/statechart`. The view module is `SaveFile.view.mjs` beside `SaveFile.tla`.

## What is checked

One note. Two writers, `w1` and `w2`. Each writer does one `GetNote` and then one `SaveFile`. ETags are a counter starting at 1, not UUIDs. DynamoDB calls do not fail. The presented etag is the non-empty value `GetNote` returned.

| Action | One step in the code |
| --- | --- |
| `GetNote` | `NoteHandler.GetNote` returns `file.ETag`. The etag on a successful `UpdateNote` response is the same kind of observation |
| `GetFile` | `Adapter.GetFile` on the DynamoDB path, one `GetItem` |
| `RejectStale` | `Adapter.SaveFile`: `etag != "" && f.ETag != etag` returns `ErrPreconditionFailed`. `UpdateNote` maps that to HTTP 412. No write |
| `RejectMovedEtag` | `PutItem`'s `ConditionExpression` `etag = :expected` fails. `:expected` is the etag `GetFile` returned. `SaveFile` returns `ErrPreconditionFailed`. No write |
| `PutItem` | That condition holds, so `PutItem` commits. The stored etag still equals the `GetFile` snapshot |

`StaleSaveRejected` — a committed save presented the ETag its own `GetFile` returned.

`NoLostUpdate` — two committed saves did not both snapshot the same stored ETag.

`TypeOK` — the model state has the shape above.

## Known finding

At the sync commit, TLC exits 0. `TypeOK`, `StaleSaveRejected`, `NoLostUpdate`, and `NoDeadlock` hold. Do not weaken `NoLostUpdate`. Do not drop `RejectMovedEtag` while the Go `PutItem` still has `ConditionExpression` `etag = :expected`.

Replay the two saves against the adapter:

```bash
mise x go -- bash -c 'cd .cursor/skills/verify-note-save/scripts/repro && go run .'
```

Exit 0 means both `SaveFile` calls returned success and one body is gone. Exit 1 means that did not happen. At the sync commit the replay exits 1 and prints `bug did not reproduce: saves=2 unconditional puts=0`. The module path of that program stays under `github.com/jun/gophdrive/backend/internal/...` so Go allows the import of `internal/adapter/dynamo`. The directory is not inside `backend/`.

## Tell a bug from a modeling error

Walk the counterexample against `Adapter.SaveFile` and `NoteHandler.UpdateNote`.

- `NoLostUpdate`: both `GetFile` calls complete before either `PutItem`, both presented the same etag, both `PutItem` run. If the replay exits 0, it is the bug. If one `SaveFile` returns `ErrPreconditionFailed`, the model is behind the code.
- `StaleSaveRejected`: a writer commits whose presented etag differs from the etag `GetFile` just returned. If the `etag != "" && f.ETag != etag` branch is gone from the DynamoDB `SaveFile`, it is the bug. If that branch is still there, the model dropped `RejectStale` or the `presented[w] = snapshot[w]` conjunct by mistake. Put them back.
- A step in the trace that `SaveFile` cannot take (a third writer, an empty etag, a failed `GetItem`) is a modeling error. Fix the model. Do not drop the invariant.

## When the covered code changes

Diff the covered files from the sync commit. A rename or a move of these functions is still covered code.

- **No effect** (comments, formatting, the in-memory `saveFileMap` body, a path that returns before `GetFile`): leave the model. You may move the sync commit after a run that still matches this section's known result.
- **The compare branch changed.** If `etag != "" && f.ETag != etag` is gone from the DynamoDB `SaveFile`, delete `RejectStale` and drop `presented[w] = snapshot[w]` from `PutItem`. Leave `StaleSaveRejected` in the spec and in `SaveFile.cfg`. A failure of `StaleSaveRejected` is the promise breaking. Delete that invariant only when the change itself says the promise changed: the `StorageAdapter.SaveFile` comment, the handler tests, or the commit message. Record the old wording, the new wording, and that evidence.
- **The write became conditional.** If `PutItem` now has a `ConditionExpression` that requires the stored etag to equal the etag `GetFile` returned, `PutItem` in the model must be able to fail when `storedEtag` has moved since that `GetFile`. Do not keep an unconditional write in the model. `NoLostUpdate` should then hold. The replay must exit 1 before you treat the finding as fixed.
- **A new step reaches the row.** A new writer of `etag` or `content` on this item (another handler calling `SaveFile`, a second `PutItem`) needs an action. A step that does not change `etag` or `content`, such as `TouchViewed`, stays out of the model.

Move the sync commit only after the spec describes the code at the new commit and the check has been run. Do not move it when TLC exits `150`, `151`, or `255`.

## Coverage

Sync commit: `9a365f69a1c653fd2a4d128417602ac1ecd4b1ee`

Confirmed against that commit. The line numbers below are from that commit.

- `backend/internal/adapter/dynamo/dynamo.go` — `Adapter.SaveFile` when `inMemory` is false (`GetFile` at line 408, the etag compare at line 413, `etag = :expected` at line 476, `ConditionalCheckFailedException` mapped to `ErrPreconditionFailed` at lines 484-485). `Adapter.GetFile` at line 308.
- `backend/internal/handler/note.go` — `NoteHandler.UpdateNote` reads `If-Match` and passes it to `SaveFile`. It does not consult the editing lock.
- `backend/internal/adapter/storage.go` — `StorageAdapter.SaveFile` documents that an empty etag forces an overwrite. That path is not a step in the model.

Not covered: `saveFileMap`; an empty `If-Match`; `LockManager` acquire, heartbeat, and release; header-name case on `If-Match`; a delete between `GetFile` and `PutItem`; more than two writers; more than one note; a DynamoDB error; the UUID form of an etag.
