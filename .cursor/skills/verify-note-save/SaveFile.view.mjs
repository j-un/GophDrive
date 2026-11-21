const wroteStale = (v) =>
  (v.outcome.w1 === "committed" && v.presented.w1 !== v.snapshot.w1) ||
  (v.outcome.w2 === "committed" && v.presented.w2 !== v.snapshot.w2);

const label = (v) => {
  const a = v.outcome.w1;
  const b = v.outcome.w2;
  const both = a === "committed" && b === "committed";
  const sameSnapshot = v.snapshot.w1 === v.snapshot.w2 && v.snapshot.w1 !== 0;
  if (wroteStale(v)) return "A save wrote over a different ETag";
  if (both && sameSnapshot) return "Both saves succeeded against the same ETag";
  if (a === "committed" && b === "rejected") return "First save kept, second rejected";
  if (b === "committed" && a === "rejected") return "Second save kept, first rejected";
  if (both) return "Both saves succeeded on different ETags";
  if (a === "committed" || b === "committed") return "One save stored, the other still going";
  if (v.phase.w1 === "read" || v.phase.w2 === "read") return "A save has read the note and not written";
  if (v.phase.w1 === "ready" || v.phase.w2 === "ready") return "A writer holds an ETag";
  return "Neither writer has read the note";
};

export const state = (v) => label(v);

export const step = (action, _from, to) => {
  if (action.startsWith("GetNote")) return "reads the note and keeps its ETag";
  if (action.startsWith("GetFile")) return "save reads the stored note";
  if (action.startsWith("RejectStale")) return "save sees a different ETag and stops";
  if (action.startsWith("RejectMovedEtag")) return "save finds the stored ETag changed and stops";
  if (action.startsWith("PutItem")) {
    if (label(to) === "Both saves succeeded against the same ETag") {
      return "save writes the note [the other save already read this ETag]";
    }
    return "save writes the note";
  }
  if (action === "Done" || action.startsWith("Done")) return null;
  return action;
};

export const group = (v) => {
  const name = label(v);
  if (name === "A save wrote over a different ETag") return "Stale write";
  if (name === "Both saves succeeded against the same ETag") return "Lost update";
  if (name.endsWith("rejected")) return "Stale save rejected";
  if (name === "Both saves succeeded on different ETags") return "Both writes kept";
  return "In progress";
};

export const colors = {
  "Stale write": "pink",
  "Lost update": "orange",
  "Stale save rejected": "teal",
  "Both writes kept": "blue",
  "In progress": "yellow",
};

export const title = "Two saves of one note";
