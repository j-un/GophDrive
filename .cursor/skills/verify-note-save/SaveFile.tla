------------------------------ MODULE SaveFile ------------------------------
\* Production Adapter.SaveFile on the DynamoDB path.
\* One note. Two writers. Each writer reads the note once, then calls SaveFile
\* once. SaveFile is two indivisible steps, matching the code:
\*   GetFile  — dynamo.go Adapter.GetFile, one GetItem
\*   then either
\*     RejectStale — the local compare in Adapter.SaveFile
\*       (etag != "" && f.ETag != etag returns ErrPreconditionFailed)
\*     or
\*       PutItem — the PutItem in Adapter.SaveFile, which sets no
\*       ConditionExpression
\* The compare uses the ETag captured by that GetFile, not a new read.
\* An empty etag is a documented force-overwrite (StorageAdapter.SaveFile)
\* and is not a step here. The in-memory saveFileMap holds mu across the
\* check and the write, so this interleaving is the DynamoDB path only.

EXTENDS Naturals

CONSTANT Writers

VARIABLES
  storedEtag, \* version of the note row; the real code stores a UUID
  storedBody, \* "origin" or the writer whose content was stored last
  phase,      \* writer -> "observe" | "ready" | "read" | "done"
  presented,  \* ETag the writer will send as If-Match
  snapshot,   \* ETag returned by this writer's SaveFile GetFile
  outcome     \* "pending" | "committed" | "rejected"

vars == <<storedEtag, storedBody, phase, presented, snapshot, outcome>>

Init ==
  /\ storedEtag = 1
  /\ storedBody = "origin"
  /\ phase = [w \in Writers |-> "observe"]
  /\ presented = [w \in Writers |-> 0]
  /\ snapshot = [w \in Writers |-> 0]
  /\ outcome = [w \in Writers |-> "pending"]

\* NoteHandler.GetNote returns file.ETag. NotePage stores it and later sends
\* it as If-Match. The etag on a successful UpdateNote response is the same
\* kind of observation: one read of the current ETag into the client's hands.
GetNote(w) ==
  /\ phase[w] = "observe"
  /\ phase' = [phase EXCEPT ![w] = "ready"]
  /\ presented' = [presented EXCEPT ![w] = storedEtag]
  /\ UNCHANGED <<storedEtag, storedBody, snapshot, outcome>>

\* Adapter.SaveFile calls GetFile before it compares or writes.
GetFile(w) ==
  /\ phase[w] = "ready"
  /\ phase' = [phase EXCEPT ![w] = "read"]
  /\ snapshot' = [snapshot EXCEPT ![w] = storedEtag]
  /\ UNCHANGED <<storedEtag, storedBody, presented, outcome>>

\* Adapter.SaveFile returns ErrPreconditionFailed when the presented etag
\* is non-empty and differs from the GetFile snapshot. UpdateNote maps that
\* to HTTP 412. This step does not write.
RejectStale(w) ==
  /\ phase[w] = "read"
  /\ presented[w] # snapshot[w]
  /\ phase' = [phase EXCEPT ![w] = "done"]
  /\ outcome' = [outcome EXCEPT ![w] = "rejected"]
  /\ UNCHANGED <<storedEtag, storedBody, presented, snapshot>>

\* The compare passed, so SaveFile builds a new ETag and PutItem with no
\* condition. A PutItem that starts after another writer's PutItem still
\* overwrites the row.
PutItem(w) ==
  /\ phase[w] = "read"
  /\ presented[w] = snapshot[w]
  /\ storedEtag' = storedEtag + 1
  /\ storedBody' = w
  /\ phase' = [phase EXCEPT ![w] = "done"]
  /\ outcome' = [outcome EXCEPT ![w] = "committed"]
  /\ UNCHANGED <<presented, snapshot>>

\* Both writers have finished. Keeps the end state from looking stuck.
Done ==
  /\ \A w \in Writers : phase[w] = "done"
  /\ UNCHANGED vars

Next ==
  \/ \E w \in Writers : GetNote(w)
  \/ \E w \in Writers : GetFile(w)
  \/ \E w \in Writers : RejectStale(w)
  \/ \E w \in Writers : PutItem(w)
  \/ Done

Spec == Init /\ [][Next]_vars

TypeOK ==
  /\ storedEtag \in Nat \ {0}
  /\ storedBody \in (Writers \cup {"origin"})
  /\ phase \in [Writers -> {"observe", "ready", "read", "done"}]
  /\ presented \in [Writers -> Nat]
  /\ snapshot \in [Writers -> Nat]
  /\ outcome \in [Writers -> {"pending", "committed", "rejected"}]

\* A save that commits presented the ETag its own GetFile returned.
\* This is the compare in Adapter.SaveFile. It says nothing about whether
\* that ETag was still stored at the PutItem.
StaleSaveRejected ==
  \A w \in Writers :
    outcome[w] = "committed" => presented[w] = snapshot[w]

\* Two committed saves did not both snapshot the same stored ETag.
\* The second PutItem would otherwise erase the first body, and both
\* SaveFile calls would still return success.
NoLostUpdate ==
  \A w1, w2 \in Writers :
    (w1 # w2 /\ outcome[w1] = "committed" /\ outcome[w2] = "committed")
      => snapshot[w1] # snapshot[w2]

=============================================================================
