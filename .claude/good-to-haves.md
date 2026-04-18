# Good-to-haves

Deferred work items that are not part of the MVP but are worth picking up later. Each entry should describe *what* the feature is, *why* it was deferred, and *what implementing it entails*.

---

## 1. FSR explicit disable-path (rotation Step 3 & 4)

**Context**: Gemini manages AWS EBS Fast Snapshot Restore (FSR) lifecycle for `SnapshotGroup`s that opt in via `spec.fastSnapshotRestore.enabled=true`. The full rotation policy (see `.claude/feat/snapshot/hot-snapshot-scaleup-gemini.md` §4) has four steps:

1. Enable FSR on the newest `ReadyToUse` snapshot → write `fsr-state=enabling`.
2. Poll AWS; when warm in all target AZs → write `fsr-state=enabled`.
3. **[DEFERRED]** Once the new snapshot is `enabled`, call `DisableFastSnapshotRestores` on the previous FSR-enabled snapshot → write `fsr-state=disabling`.
4. **[DEFERRED]** Poll AWS; when disable completes → write `fsr-state=disabled`.

The MVP ships only Steps 1 & 2. Steps 3 & 4 are this good-to-have.

### What it does

When a newer snapshot reaches `fsr-state=enabled`, find any other snapshot in the same `SnapshotGroup` whose `fsr-state` is `enabling` or `enabled` and is older than the latest. Explicitly call AWS `DisableFastSnapshotRestores` on it (for the target AZs), annotate it `fsr-state=disabling`, then poll `DescribeFastSnapshotRestores` on subsequent reconciles until AWS reports the state has left "enabled" in all target AZs — at which point flip the annotation to `fsr-state=disabled`.

### Why it was deferred

Without explicit disable, FSR auto-disables only when the snapshot itself is deleted (by Gemini's existing `Keep` rotation). In the window between "new snapshot is warm" and "old snapshot is deleted by `Keep`," the user pays for FSR on **two** snapshots instead of one. With `keep: 1`, the window is roughly one snapshot interval — acceptable for v1 in exchange for a much smaller state machine (no `disabling`/`disabled` branches, no second polling loop, no second set of AWS calls to mock/test).

### What implementing it entails

1. **Reconciler**: extend `reconcileFSR` (in the snapshots package) with two new branches:
   - If the latest snapshot is `fsr-state=enabled` and there exists an older snapshot with `fsr-state ∈ {enabling, enabled}`, call `DisableFastSnapshotRestores(oldSnapshotID, targetAZs)` and annotate the old snapshot `fsr-state=disabling`.
   - Iterate over all snapshots in the group; for each with `fsr-state=disabling`, poll `DescribeFastSnapshotRestores`. If the state has left `enabled` in all target AZs, annotate `fsr-state=disabled`. Otherwise requeue.

2. **AWS client interface**: add `DisableFastSnapshotRestores(ctx, snapshotID, azs) error` to the FSR client interface. Implement in the `aws-sdk-go-v2` concrete client and in the fake client used for unit tests.

3. **Requeue**: the existing `AddAfter(~60s)` logic used for `enabling` must also fire for any snapshot in state `disabling`. The reconciler's "should-requeue" predicate needs to consider both transitional states.

4. **Timeouts / failure**: mirror the enable-path policy — > 2h in `disabling` → annotate `failed`, emit Event; transient AWS errors → workqueue rate-limited retry (no annotation write).

5. **Idempotency**: if `DisableFastSnapshotRestores` returns "not enabled in this AZ" for a target AZ (e.g. because the user disabled manually or AWS already cleared it), treat as success for that AZ.

6. **Unit tests**: add cases for
   - `enabled` + older `enabled` exists → older transitions to `disabling`.
   - `disabling` + AWS still shows `enabled` → requeue, state unchanged.
   - `disabling` + AWS cleared → `disabled`.
   - `disabling` > 2h → `failed`.
   - AWS error on disable → backoff, no annotation change.

7. **E2E**: after implementing, exercise the full handoff in a lower environment — create two snapshots back-to-back, verify old one transitions `enabled → disabling → disabled` and AWS billing for FSR on the old snapshot stops.

### Contract impact

The operator (`karnotxyz/karnot-madara-operator`) does not need to change. It already ignores any `fsr-state` value other than `enabled`. `disabling` and `disabled` are simply additional non-selectable values, indistinguishable from "absent" from the operator's perspective. The annotation contract in `.claude/feat/snapshot/hot-snapshot-scaleup-gemini.md` §3.2 already documents these values, so implementing this is purely an internal behavior change on the Gemini side.

### When to pick this up

- Cost of 2× FSR becomes material (large volumes, many SnapshotGroups, or long snapshot intervals where the overlap window stretches to hours).
- Finance/FinOps flags the line item.
- Operator team adds a feature (e.g. scheduled snapshot-retention changes) that widens the overlap window beyond one snapshot interval.
