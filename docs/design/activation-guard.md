# Activation Guard: Protocol Specification

Status: draft v2.0, implementation contract. Audience: panix contributors
implementing T1-T7. This document is the single source of truth for the Activation Guard. Every
decision here was verified against primary sources (systemd man pages/source, Nix/nixpkgs source,
nix-darwin source, home-manager source) or adversarially reviewed. Deviations require updating
this document first. The document describes the end state after the three migration stages (17);
Stage 1 and the Executioner duplex primitives are implemented, Stages 2 and 3 are not yet.
Revision history at the end.

## 1. Goals and non-goals

Goals:

1. Any guarded activation either commits or reverts to the pre-deploy state within a bounded
   time, even if panix dies, disconnects, or never returns ("no matter what").
2. For switch/test deployments (profile-last tiers), the bootloader default and the profile stay
   untouched until the activation is confirmed, so a reboot mid-window always lands on the old
   generation. `boot` mode is the documented exception (5).
3. Confirmation requires evidence the system actually works: exit code, SSH reachability
   (magic tier), and health checks (local and remote).
4. Target-side footprint: exactly two filesystem entries per transaction (log file + one GC-root
   symlink) plus the transient guardian binary persisted in the slot and removed by the next
   pre-start converge (10.3). No state.json, no marker files, no lock file, no pidfile, no
   systemd dependency.
5. The original activation error is never buried. A reverted deploy never looks successful.

Non-goals (v1): fleet-wide coordinated rollback; http health checks; nix-on-droid revert
(minimal tier only); bootstrap-path guarding (no previous generation); compatibility modes.

## 2. User-facing model

Attributes (flat, snake_case, fleet/flake/installable/machine levels, absence = inherit):

```
rollback: off | auto | magic        # default off; replaces auto_rollback outright
activation_timeout: 15m             # bound for the activation phase, all guarded tiers
rollback_confirm_timeout: 60s       # magic tier only: panix confirmation window
reboot_on_revert_failure: off       # last-resort reboot after failed revert (gated, 6.3)
health_checks: []                   # panix-side cmd checks, magic tier only
health_checks_local: []             # guardian-side cmd checks, auto and magic tiers
```

Validation rules (config-load time):

- `health_checks` non-empty requires `rollback: magic`: config error otherwise.
- `health_checks_local` non-empty with `rollback: off`: config error.
- `rollback_confirm_timeout` set with `rollback: off|auto`: config error.
- Legacy `auto_rollback` key in any config: hard error with migration hint
  (`auto_rollback: true` -> `rollback: magic`, `auto_rollback: false` -> `rollback: off`).
  No compatibility mode; breaking release.
- Check budget: sum of per-check timeouts (default per-check timeout: 30s) plus margin (5s)
  must fit within `rollback_confirm_timeout`; the budget is measured from the moment panix
  observes `ACTIVATED` (checks run after that), not from window start.
- Boot mode's effective gate is `auto` (5): the transaction has nothing to confirm because the
  new system is not live until reboot, so the remote `health_checks` would never run. Config
  error: `health_checks` on a boot-mode installable. `health_checks_local` remains valid there
  (the guardian-side CHECKING phase runs regardless of the gate). The validator resolves the
  mode statically from the installable's `activation_mode` (or the preset default).

Tier semantics:

- `off`: today's direct activation (profile set, activate, no guard). Also used for
  `dry-activate` mode and as the emergency path behind `--unsafe-direct-activation` (debug only).
- `auto`: guarded, self-committing. Guardian runs the activation, runs `health_checks_local`
  (CHECKING state), commits on success, reverts on any failure or timeout. No external
  confirmation; no confirm window. Deadline covers hangs only.
- `magic`: `auto` + confirmation gate. After activation success the guardian waits for panix's
  confirmation (`ctl confirm`) within `rollback_confirm_timeout`; no confirmation by deadline
  implies revert. Remote `health_checks` run before panix confirms; `health_checks_local` gate
  the commit.

Type support matrix (v1):

| Type | Tier class | Notes |
|---|---|---|
| nixosConfigurations | full | all modes (see 5) |
| darwinConfigurations | standard | verified V2: activate tolerates profile != closure; old-gen activate reverts via current-system diffing; boot daemon re-points to OLD under profile-last (mid-window reboot safe) |
| systemConfigs | standard | verified V1: `bin/activate` does NOT self-set the profile; commit via `bin/register-profile` (profile + system-manager's own gcroot) |
| homeConfigurations | standard (profile-last) for HM >= 25.11 (per-deploy gen-version >= 1 detection), self-setting for older | modern: panix sets the profile (--set the bare activationPackage), driver-1 activate performs zero profile ops; legacy: activation self-updates the profile (verified); revert = old gen activate (driver-1 when OLD is modern) |
| nixOnDroidConfigurations | minimal | detached execution + captured log only, no revert |
| maidConfigurations | minimal | same as nixOnDroid |
| packages | none | unchanged |

Bootstrap path: excluded everywhere (no previous generation). Arch without an embedded guardian
binary: fail fast at Inspect, only when the resolved tier is guarded.

## 3. Architecture overview

Components:

- **panix deployer**: pre-start orchestration, live-window relay consumption (the duplex spawn
  exec: command frames out on stdin, wire events in via the Executioner option pair `WithStdin`
  (stdin write handle) + `WithOutputTap` (wire-event callback); primitives implemented, 17),
  `ctl` confirmations, terminal-state reporting, inline post-mortem via `converge`.
- **panix-guard** (Go, stdlib-only, CGO-free, `cmd/panix-guard`): one self-detached process per
  transaction. Subcommands: `start` (relay + spawn; absorbs `follow`), `attach` (reconnect
  viewer, `--from N`), `ctl` (degraded control), `converge` (decide + sweep folded: self-locks,
  classifies, converges, truncates; `--truncate` for the pre-start caller), `inspect` (prints
  one JSON verdict: status, terminal, old/new/gen/mode, embedded step lists, rc, err excerpt,
  lock state; absorbs the `lock` verb and all deployer-side tail parsing). Built once per arch at
  panix build time, gzip-embedded, transferred per deploy, removed by the next pre-start converge
  (10.3).
- **slot**: one fixed directory per profile (not per deploy) holding the log, the GC root, and
  the persisted guardian binary (10.1).
- **post-mortem `converge`**: offline convergence invoked by panix inline (9.4); the pre-start
  convergence runs in-process under `start`'s own lock (4.1).

Fileless principle: after the spawning SSH connection dies, the only connection-independent
channels to a detached guardian are signals, the filesystem, and network ports. The live window
needs none of them: the spawn exec is a full-duplex relay over the one SSH connection (commands
in, events out; 6.1, 8). Signals remain the degraded control floor (wire dead), hardened with
cmdline+key pid forensics; the log file is the durable transaction record (records + heartbeats);
the Nix profile/generation list is the durable transaction evidence. Network channels are
rejected (port discovery needs a file, trust is broader than signal permissions, loopback
reachability is unguaranteed).

The two-file floor: GC roots are filesystem symlinks by definition, so one root symlink is
irreducible; the log file is the streaming console panix needs anyway and doubles as the
post-mortem authority.

## 4. Transaction ordering (profile-last) and per-tier classes

### 4.1 Pre-start sequence (several execs; mutations under the spawn exec's lock)

Pre-start is several execs, not one. The lock is acquired by `start`, inherited by the detached
guardian, and every mutation happens under that lock. The lock lives as long as ANY holder lives:

- invariant: flock held <=> a relay or guardian is alive (or a converge is running); flock-free
  => safe to converge/spawn;
- the relay may be killed at any time (disconnect, panix cancel): the guardian's inherited
  reference keeps the lock; panix canceling a deploy kills the relay exec, the guardian continues
  and converges on its own;
- disconnect mid-window keeps the lock via the guardian;
- both dead releases it; the next pre-start converges.

Sequence (read-only phases first, then the one mutating exec):

1. Resolve and prepare (read-only plus slot creation): absolute tool paths (`nix-env`,
   `nix-store`, activation child path via the new closure, `systemctl` NixOS only). The
   "stripped PATH" concern applies to panix's own resolution only; the child inherits the
   guardian's env (6.4). Verify su -l / secure_path PATH suffices for STC in V3. Resolve the
   platform, the slot dir per tier (10.1), and the embed (15); mkdir 0700; verify ownership is
   the executing identity.
2. Probes (read-only): KillUserProcesses probe (10.2); legacy-slot liveness probe and cleanup
   decision (10.1).
3. Transfer the guardian binary (idempotent, hash-skipped probe, atomic `cat > tmp && mv`,
   privilege wrap built into the Write argv; never exec a possibly-truncated transfer). Safe to
   overwrite a running binary: the running process keeps executing the old inode; the mv swaps
   only the directory entry. Transferring before the lock probe guarantees the probe has a
   binary to run.
4. Lock probe (HARD gate, unchanged semantics): `inspect`'s lock state (9.5); a missing log
   means no lock can exist and the probe is skipped. The probe is advisory-fast; `start`'s own
   nonblocking flock is the authoritative gate. Failure to acquire: fail fast with the slot path
   and the verdict's last status; verify the pid from the last `HELLO` is still alive
   (cmdline + key match) before printing it, else print "live deploy, slot <dir>, last record
   <status>".
5. exec `panix-guard start` with the full argv contract (6.5). `start` acquires the flock
   (`LOCK_EX|LOCK_NB` on the log fd) and, under it, performs every remaining mutation AND the
   OLD capture, in this order: the pre-start convergence (classify the previous log, converge a
   non-terminal transaction, truncate the log in place; 9.3), the GC-root creation
   (`nix-store --add-root <slot>/gc-root --indirect <TARGET>`, TARGET per class/mode in 4.3),
   the OLD capture (read-only `readlink -f` of the profile plus its generation number) as the
   rollback target for the new transaction, and — boot mode ONLY — the pre-start profile set
   (`--set NEW`; the boot installer enumerates profile generations; 5). Profile-last tiers: do
   NOT touch the profile here outside boot mode. `start` then spawns the detached guardian
   (OLD passed in its argv from the post-converge capture; panix does NOT capture OLD) and
   becomes the live relay (6.1).

Capture OLD lives INSIDE `start`, after convergence, deliberately: if a previous transaction
was interrupted before its confirm, its never-committed NEW generation still occupies the
profile until the converge reverts it. Capturing before that revert would make the interrupted
NEW generation the new transaction's rollback target (revert would then restore the broken
generation). The capture also precedes the boot-mode profile set: after `--set NEW` the
profile reads NEW, and boot mode's rollback target must be the pre-set generation. The
guardian's precondition re-check (6.2) validates against the same post-converge profile, so
capture and check stay consistent.

Two verified v1.3 defects this reorder fixes:

- Boot mode's pre-start `--set NEW` was specified but unimplemented (the compose step returned
  nil citing "profile set at pre-start"): a guarded boot deploy failed its precondition (safe,
  zero effects, but broken). The set now runs inside `start` under the lock.
- The v1.3 sweep recomposed the previous transaction's commit/revert step lists with the CURRENT
  deploy's mode (mode drift). The TXN record (7) removes recomposition entirely: post-mortem
  convergence reads the interrupted transaction's own embedded step lists.

### 4.2 Commit and revert transactions per class and mode

The guardian executes step lists composed by panix (spec 6.5): preset knowledge stays
panix-side. The tables below define what panix composes; `--set` is idempotent and re-runs
converge.

| Class / mode | Commit |
|---|---|
| nixos switch | `nix-env -p <profile> --set <NEW>` then `<NEW>/bin/switch-to-configuration boot` |
| nixos test | none (profile untouched per ProfileSkipModes) |
| nixos boot | none (profile set at pre-start; boot ran during activation) |
| darwin | `nix-env -p <profile> --set <NEW>` only (verified: activate is profile-agnostic) |
| systemConfigs | `<NEW>/bin/register-profile` only (sets profile AND updates system-manager's own `gcroots/system-manager-current`; verified V1: activate is profile-agnostic, register-then-activate is the canonical order) |
| self-setting (HM) | none (activation advanced the profile) |
| minimal | none |

Commit-time failure split (nixos switch): `--set` fails => nothing committed, revert = OLD
switch; `--set` ok, `boot` fails => `--set OLD` + OLD switch + delete the created generation.

Revert steps (panix-composed `--revert-argv`, spec 6.5): nixos =
`<OLD>/bin/switch-to-configuration switch` (where a bootloader exists, revert must rewrite the
boot default; never `test`); darwin = `<OLD>/activate`; systemConfigs =
`<OLD>/bin/register-profile` then `<OLD>/bin/activate` (verified V1: register restores the
profile and system-manager's own gcroot, activate diffs the state back); self-setting = OLD
generation's `activate` (re-flips the user profile); minimal tier has no revert. Whether the
revert also restores the profile (`--set OLD`) and deletes the created generation is derived at
runtime from the profile's current state (spec 6.5), covering boot mode's pre-start set and
partial commits without extra flags.

Idempotency contract for re-executed steps (converge): `--set X` idempotent;
`--delete-generations <gen>` must tolerate a missing generation (nonzero exit tolerated, logged);
STC `boot` re-run safe; OLD `activate` re-run assumed safe per tier (V1/V2 verify for
system-manager and darwin); STC lock retry with backoff wraps every activation-family step.

Post-commit invariant (profile-last tiers): profile resolves to NEW; violation => revert
transaction.

### 4.3 GC root targets (authoritative table)

| Class / mode | gc-root -> | Rationale |
|---|---|---|
| nixos switch/test (profile-last) | NEW | profile protects OLD (current); `-d` deletes non-current gens, never current |
| nixos boot (profile-first) | OLD | profile set to NEW at pre-start; OLD demoted |
| darwin (profile-last) | NEW | same as nixos profile-last |
| systemConfigs (profile-last) | NEW | profile untouched until `bin/register-profile` runs at commit |
| self-setting (HM) | OLD | activation flips the profile; OLD becomes non-current |
| minimal / none | n/a (minimal: none; packages: no slot) | |

`nix-collect-garbage -d` semantics (verified): deletes every non-current generation of every
profile including per-user profiles; current always kept; current can never be deleted via
nix-env. NixOS-native roots exist (`gcroots/current-system`, `gcroots/booted-system`) but protect
the booted/last-activated system, which equals our rollback target only in steady state; the
common fleet state (previous switch without reboot) diverges, hence our own root is mandatory.

## 5. Activation modes (nixosConfigurations)

- `switch` (default): two-phase. Worker runs `<NEW>/bin/switch-to-configuration test`; commit =
  `--set` + `boot` (4.2). Reboot mid-window boots OLD (bootloader untouched until commit).
- `test`: worker runs `test`; commit step is empty (profile untouched per `ProfileSkipModes`);
  revert = OLD `switch-to-configuration switch`... correction: revert uses OLD's activation with
  the same mode semantics as the deploy used; for test-mode deploys revert re-runs OLD's
  activation in `test` mode (runtime restoration only; no bootloader was touched by NEW's test).
- `boot`: profile-first mechanics (4.1): `--set NEW` under `start`'s lock at pre-start, worker
  runs `boot`, commit step empty, revert = `--set OLD` + OLD `boot`. Documented caveat: the
  bootloader IS touched before confirmation; reboot mid-window boots NEW. `reboot_on_revert_failure` must
  never fire in boot mode. Goal 2 exception.
- `dry-activate`: direct, no guard (non-mutating).

switch-to-configuration exit codes (verified for nixos-25.05 `.pl` and `-ng`; pin exact versions
in the test matrix, V3): 0 ok; 1 usage/pre-switch/bootloader-install failure; 2 activation
script failure; 3 daemon-reload failure; 4 unit start/stop/restart/reload failures (and any
failed unit in the final scan); 100 init-interface mismatch requiring reboot. All nonzero are
activation failures for the guard. Call STC directly, never via nixos-rebuild (it collapses exit
codes). Known -ng history: bootloader-install failure used to exit 0 (fixed in
40fb9e7c5a45af3f63fc821067f044d3d8a4befc); the post-commit invariant (4.2) is the belt-and-braces
defense.

STC global lock (`/run/nixos/switch-to-configuration.lock`, `LOCK_EX|LOCK_NB`): the guardian
retries activation-family steps with backoff when the lock is held; this is the expected
transient failure mode.

STC runtime requirements (verified V3): both implementations invoke commands by compiled-in
absolute paths and `/run/current-system/sw/bin`; the activation script hermetically sets its own
PATH. The guardian child needs no PATH massage for STC itself. Caveats for the test matrix:
`.pl` ignores stop-batch failures (a failed stop does not produce exit 4); `-ng` requires
`/run/current-system/sw/bin` to exist and `LOCALE_ARCHIVE` (set by the wrapper). The guardian
still sets a conservative PATH (`/run/current-system/sw/bin:/usr/bin:/bin`) as cheap insurance
for user-configurable `preSwitchChecks` and bootloader hooks, which inherit our env.

## 6. Guardian process

### 6.1 Spawn and detach (start = relay)

`panix-guard start ...` is one exec that does three things in order:

1. Acquire the transaction lock and mutate under it. Open/create the log, flock it
   `LOCK_EX|LOCK_NB` (failure = live transaction: exit nonzero, zero effects). Under the lock,
   `start` performs every pre-start mutation (4.1): the pre-start convergence (classify the
   previous log from the tail; converge a non-terminal transaction from its TXN record's
   embedded step lists, generation-arithmetic fallback; truncate the log in place), the GC-root
   creation, and the boot-mode pre-start profile set (`--set NEW`, idempotent, re-runs
   converge; 4.2, 5).
2. Spawn the detached guardian: re-exec/self-spawn with `SysProcAttr{Setsid: true}`, stdio
   redirected into the slot (guardian stdout and stderr to the log fd so pre-logging panics
   land in the transcript), and `cmd.ExtraFiles` carrying the lock fd, the command pipe, and
   the event pipe (`PANIX_GUARD_LOCK_FD=<n>` env or fd-passing equivalent).
   `signal.Ignore(SIGHUP)`. Handlers for SIGUSR1 (confirm), SIGUSR2 (revert-request), SIGTERM,
   SIGINT are installed before anything else runs. SIGTERM/SIGINT semantics: on a non-terminal
   state, treated as revert-request (same as SIGUSR2); on a terminal state, write the ack
   record and exit. CLOEXEC discipline: the guardian sets FD_CLOEXEC on all inherited fds
   (lock fd, command pipe, event pipe) at startup, so the activation child inherits none of
   them; a leak would break EOF-based death detection and keep the lock alive after guardian
   death.
3. Become the relay: the `start` process then IS the PTY<->pipes relay (one exec, one
   connection, full duplex: panix stdin -> command pipe, event pipe -> panix stdout; zero
   additional connections in the live window; 8, 9.1). Design-level pin: any self re-exec in
   the start chain must set Stdin/Stdout/Stderr explicitly; Go's nil maps to /dev/null, which
   is silent wire death at birth. The guardian inherits the lock fd, so the lock survives
   relay death.

Wire policy (guardian side):

- Log-then-wire-then-exit ordering invariant for every record: a dropped wire frame must never
  lose a durable record.
- Single writer goroutine with a bounded drop-oldest buffer; the state machine never blocks on
  the wire. Drops are logged as LINK_DEGRADED records. LINK_DOWN is recorded to the LOG and
  never announced on the dying channel; wire EOF/EPIPE is LINK_DOWN.
- The guardian always drains the command pipe (from before HELLO).
- Top-level `recover()` emits a terminal error record (log first, then wire) before the
  process dies.

Relay failure branches:

- Bootstrap window: if the guardian dies before the pipes are wired, the relay sees event-pipe
  EOF, reads the log tail once, and classifies (9.1); no tailing goroutine.
- Spawn failure: report a failed-precondition-shaped outcome, release the lock reference
  (process exit closes the fd), exit nonzero.

### 6.2 Lifecycle (state machine)

States: `PRE`, `ACTIVATING`, `CHECKING`, `ACTIVATED`, `COMMITTING`, `COMMITTED*`,
`REVERTING`, `REVERTED*`, `REVERT_FAILED*`, `FAILED_PRECONDITION*` (* terminal). Minimal tier:
commit = none, revert = none, terminal = `ACTIVATION_EXITED` (rc recorded); the deadline still
bounds the child. Commands arrive from two transports (8): wire frames drained from the command
pipe and signals (degraded path); both enqueue to the state machine in arrival order and the
first applicable transition wins.

```
PRE -> precondition re-check (profile current == captured expectation; mismatch =>
       FAILED_PRECONDITION, zero effects, never revert)
PRE -> capture failed-units baseline (NixOS only)
PRE/ACTIVATED -> HELLO already emitted (6.5); STATE snapshots every heartbeat
PRE -> ACTIVATING: run child (activation) with in-process timeout
ACTIVATING + child rc==0 -> CHECKING if health_checks_local non-empty else ACTIVATED
CHECKING + all pass -> ACTIVATED
CHECKING + any fail -> REVERTING
ACTIVATED + (magic) wait confirm; (auto) -> COMMITTING immediately
ACTIVATED + pending-confirm already set -> COMMITTING immediately
ACTIVATED/magic + confirm (SIGUSR1) -> CONFIRM_CONSUMED -> COMMITTING
ACTIVATED/magic + revert-request (SIGUSR2) -> REVERT_REQUESTED -> REVERTING
ACTIVATING/ACTIVATED/CHECKING + deadline -> REVERTING (deadline wins over pending-confirm;
  pending confirm discarded as CONFIRM_IGNORED)
ACTIVATING + revert-request -> kill child, REVERTING
ACTIVATING + deadline fired but child kill wedged -> SIGKILL child group, REVERTING
ACTIVATING/ACTIVATED/CHECKING + SIGTERM/SIGINT -> REVERTING (as revert-request)
COMMITTING + anything (SIGUSR2, SIGTERM, deadline) -> finish commit, emit the ack
  record with status late (8), COMMITTED (non-interruptible after COMMIT_START)
COMMITTED/REVERTED/REVERT_FAILED/FAILED_PRECONDITION + signals -> ack record, ctl exits 5
terminal -> EXIT record, exit 0 (always; outcomes live in records)
```

Internal deadline: monotonic, `activation_timeout + rollback_confirm_timeout + commit_budget +
grace`; commit_budget = max(120s, activation_timeout/2), internal constant (not a config attr in
v1; darwin/systemConfigs commit is mostly `--set`). Loop-counter and Go monotonic timer;
NTP-immune; stretches across suspend (documented).

### 6.3 Revert procedure (single implementation, in the guardian)

The guardian is the only revert authority. Record ordering is load-bearing for crash-resume:
`REVERT_START` is emitted BEFORE any revert step; `REVERTED`/`REVERT_FAILED` after.

1. Kill the activation child (group) if still alive; wait for reaping (bounded).
2. Execute the revert transaction (4.2) with STC-lock retry: up to N attempts with backoff (the
   held global STC lock is the expected transient cause).
3. Emit `REVERTED` / `REVERT_FAILED`.
4. `REVERT_FAILED` + `reboot_on_revert_failure: on` + safety gates met => emit the record,
   `reboot`.
   Gates: profile already restored (`--set OLD` succeeded) or profile untouched (profile-last
   pre-commit path); boot default verifiably OLD; never in boot mode. Otherwise status stays
   `REVERT_FAILED`, operator acts.

### 6.4 Child process management

The guardian runs the activation child directly (not through panix's executioner): pipes, not
PTY. Two goroutines drain stdout/stderr, line-buffered, into the single serialized log writer so
records never interleave mid-line and the 4MiB cap policy stays in control (never redirect the
child straight into the log fd). The child inherits none of the guardian's fds (CLOEXEC
discipline, 6.1). Group kill: child started with `Setpgid`, kill is
`kill(-pgid, SIGKILL)`; units already handed off to systemd are outside the group and are
handled by revert. Child env = guardian env, plus a conservative PATH
(`/run/current-system/sw/bin:/usr/bin:/bin`) set explicitly: the sshd non-interactive shell PATH
and sudo secure_path cannot be relied on (verified: STC itself needs none of it, user hooks do).
Log write errors are best-effort and never abort the transaction.

### 6.5 Spawn argv contract (start)

```
panix-guard start --dir <slot> --key <key> --profile <path> --nix-env <abs>
  --old <storePath> --new <storePath> --gen <N> --mode <mode> --tier <class>
  --confirmation <auto|magic> --activation-timeout <dur> --confirm-timeout <dur>
  --reboot-on-revert-failure <bool> --health-checks-local <json>
  --builtin-unit-check <bool> --activation-argv <json> --commit-argv <json>
  --revert-argv <json> --invariant-target <storePath|empty>
```

- `--profile`: the profile path the transaction owns (precondition check, commit `--set`,
  revert restore, post-commit invariant). The guardian derives the current profile state at
  runtime from this path instead of receiving flags for it.
- `--nix-env`: absolute nix-env path, resolved by panix at pre-start (transient contexts have
  stripped PATH); used by the guardian's own profile-restore and generation-delete steps.
- `--confirmation`: the confirmation gate (auto/magic, 2).
- `--activation-argv`: JSON array with exactly one step: the activation child argv.
- `--commit-argv` / `--revert-argv`: JSON arrays of argv arrays, composed by panix per type,
  class, and mode (preset knowledge stays panix-side; the guardian stays agnostic). Examples:
  nixos switch commit = `[["nix-env","-p",profile,"--set",NEW],["NEW/bin/switch-to-configuration","boot"]]`;
  systemConfigs commit = `[["NEW/bin/register-profile"]]`; systemConfigs revert =
  `[["OLD/bin/register-profile"],["OLD/bin/activate"]]`. Empty commit list = no commit step
  (test mode, self-setting); empty revert list = no revert (minimal).
- `--invariant-target`: the store path the profile must resolve to after the transaction's
  effects (switch: NEW after commit; test: OLD untouched; boot: NEW; self-setting: NEW after
  activation; empty = skip). Checked after commit and after activation for classes without a
  commit step; violation => revert transaction.
- Derived at runtime (no flags): on revert, the guardian reads the profile's current state:
  if it resolves to NEW, the revert prepends `--set OLD` (restore: boot mode pre-start set,
  or a partial commit); if it resolves to OLD, no restore is needed. Generation cleanup: if
  the profile's current generation number is greater than `--gen`, the revert deletes that
  generation (`--delete-generations <M>`), covering the gen created by pre-start (boot), by
  commit (`--set NEW`), or by a self-setting activation.
- `HELLO` carries: guardian pid, OLD, NEW, gen, mode, tier, profile, deadline. The guardian
  embeds `--activation-argv`, `--commit-argv`, `--revert-argv`, and `--invariant-target`
  verbatim in the TXN record it emits at startup (7): post-mortem consumers read them back
  instead of recomposing.

### 6.6 Commit procedure

See 4.2. Bounded by commit_budget. `COMMIT_START` then `COMMITTED` records. Post-commit
invariant: profile resolves to NEW; violation => run the revert transaction.

## 7. Record protocol

Sole writer: the guardian and (post-mortem) `converge`; writers continue the sequence: seq starts
from (last record seq in log tail) + 1, never restarts at 1. Log = free-form child output +
record lines. Record line format: newline-delimited sentinel-JSON, one JSON object per line,
written with `json.Marshal` and parsed with `json.Unmarshal` (`encoding/json` escapes control
characters, so a record is always exactly one line no matter what the payload carries):

```
@PG2 {"v":1,"k":"<key>","w":"g|d","seq":<n>,"ts":<unix-seconds>,"ev":"<EVENT>",...}
```

State is the fold of the event stream, not a scan for single markers: checkpoint fields
(status, pid, child pid, old, new, gen, err) take the newest non-empty value; terminal events
latch the terminal flag with their rc.

- The `@PG2` sentinel is the version carrier and an O(1) line discriminator, NOT anti-forgery:
  the per-deploy key inside the record remains the gate (nonce gating). The v1.3 `@PG1 k=v`
  text codec and its `EscapeExcerpt` helper are deleted.
- `key`: per-deploy random key (also the nonce; records without the deploy key are display
  noise). Guards against accidental forgery by activation output. A root-level activation script
  can read the guardian's argv and forge records; that threat is equivalent to it deleting a
  state file in any design and is documented, not engineered away.
- `seq`: single counter continuing across writers (`converge` inherits); unchanged from v1.3.
- `ts`: unix seconds.
- `w`: writer tag, `g` (guardian) or `d` (converge).
- Schema (Stage 1 floor, as implemented): required `v`, `k`, `w`, `seq`, `ts`, `ev`; optional
  `st` (status), `pid` (guardian), `cpid` (child), `old`, `new`, `gen`, `mode`, `tier`,
  `pf` (profile path), `dl` (deadline, unix seconds), `rc`, `rs` (human reason), `err`
  (bounded excerpt, <= 200 raw bytes). Unknown fields are ignored; an invalid envelope or a
  foreign key makes the line noise.
- Events: `HELLO`, `STATE` (full status snapshot: status, pid, child pid, OLD, NEW, gen, mode,
  tier, deadline), `ACTIVATED`, `CONFIRM_CONSUMED`, `CONFIRM_IGNORED`, `REVERT_REQUESTED`,
  `REVERT_START`, `REVERTED`, `REVERT_FAILED`, `COMMIT_START`, `COMMITTED`,
  `FAILED_PRECONDITION`, `ORPHAN_KILLED`, `ACK` (request ack, status `consumed`|`late`; the
  v1.3 `LATE_REQUEST_ACK` renamed: same semantic, ctl's exit 5 depends on observing it; 8),
  `TXN` (once at guardian startup: embeds the transaction's own activation/commit/revert step
  lists and the invariant target), `EXIT`.
- Terminal and revert records carry `rc=<n>` and a bounded single-line error excerpt
  (<= 200 raw bytes, visible suffix when budget-truncated); the full text stays in the log;
  panix reports excerpt + log path.
- `STATE` heartbeat: re-emits the full `STATE` snapshot every 5s; the heartbeat is the
  checkpoint. Makes cap truncation and torn final records self-healing: the last 64KiB always
  contain current state plus OLD/NEW/gen.

Record budgets, enforced at marshal time: `STATE` <= 512B, `TXN` <= ~4KiB. An over-budget
record drops optional fields first in a fixed deterministic order and truncates the err excerpt
LAST (never bury the error); required envelope fields and the status are never dropped. TXN
embedding removes recomposition: post-mortem consumers (`converge`, `inspect`) read the
transaction's own embedded step lists and never recompose them panix-side (this kills the v1.3
sweep mode-drift defect, 4.1).

Cap policy: 4MiB; front-truncation aligned to the last newline, always retaining >= 64KiB tail.
Single-writer at a time (guardian during the window; converge only when no guardian is live,
enforced by the lock). PINNED: truncation is in-place on the same fd (Truncate + WriteAt at
offset 0); rename-based truncation is PROHIBITED (a rename forks the inode out from under every
inherited fd: the lock, the guardian's log fd, and the relay's stdio would all point at the
orphaned old inode). A crash mid-truncate tears the final record; parsers ignore a torn final
record and the next 5s STATE heartbeat repairs state.

Durability: the log must live where it survives long enough for post-mortem convergence (10.1);
reboot clearing it is acceptable ONLY where the profile/booted-system evidence suffices (NixOS
profile-last) or where bias-revert is the defined default (user tiers).

## 8. Control channel (ctl)

One verb set (confirm, revert-request), two transports:

- Live window (wire): commands travel as frames on the exec stdin (the relay's PTY), acks return
  as wire frames correlated by request id; re-delivery is idempotent. Every ack is mirrored to
  the log as an `ACK` record, log-then-wire (6.1). panix writes no command frame before the
  first wire record is observed (readiness gate: the remote PTY input queue is finite, ~4KiB;
  6.1).
- Degraded (wire dead): the signal path is deliberately KEPT, unchanged mechanics:

```
panix-guard ctl --dir <slot> --key <key> confirm|revert-request [--wait <dur>]   # default 5s
```

  Signals and pid forensics stay because a recycled pid passed to `kill()` is defeated only by
  the cmdline+key check; log-appended command records were rejected because the in-place
  front-truncation window destroys concurrent appends even with O_APPEND. HELLO-pid discovery
  from the log stays: it feeds degraded ctl signaling and converge's orphan-kill.

Echo defense (panix-side, live window): the local PTY line discipline must not echo command
frames back into the inbound stream. panix disables PTY ECHO before writing frames (`SetEcho`
applied via the master fd; master and slave share one termios state, so the master-side call
covers the pair). Belt and braces: the inbound parser ignores command-kind frames, so an echoed
frame can never self-ack.

Behavior (degraded path): locate the guardian pid from the last `HELLO`/`STATE` record in the
log; verify liveness (`kill(pid, 0)` + `/proc/<pid>/cmdline` contains `panix-guard` and the key;
darwin: `ps -p <pid> -o command=`); send SIGUSR1 (confirm) or SIGUSR2 (revert-request); tail the
log until the matching record or `--wait` expiry. On the live wire the same verbs ride the relay
(frames in, ack frames out); the ack observation rules are identical.

Exit codes: per-verb table in 9.5. 0: consumed/acked. Matching records: confirm waits for
`CONFIRM_CONSUMED` or `CONFIRM_IGNORED`; revert-request waits for `REVERT_REQUESTED` or
`REVERT_START`; anything else (HEARTBEAT/STATE) keeps waiting. 5: refused (`CONFIRM_IGNORED`,
terminal-state acks, the status-late ack after COMMIT_START; also always on the auto tier, where
confirm is not part of the contract). 3: guardian dead (liveness check failed). 4: slot or log
missing. 7: ack-timeout (the request was delivered but no ack arrived within `--wait`; the
request may still take effect; v1.3 folded this into 5 with a stderr caveat).

Give-up cancel: panix's give-up path (user cancel, phase cancellation, exec timeout) may send
ONE best-effort fresh-connection `ctl revert-request --cause cancel` before reporting; cancel is
exactly revert-request (late-ack after COMMIT_START, no-op when terminal). There is no wire
cancel frame: at give-up time the exec carrying the wire is being torn down.

FreshConnection health checks stay loud and fresh: they carry the reachability proof; the
confirm itself rides the live wire.

Ordering guarantees:

- Never signal before a `HELLO` record is observed: handlers are installed before `HELLO` is
  written, closing the default-action-terminate race.
- Commands arrive on the runtime's signal goroutine (signals) or the command pipe (wire frames)
  and are enqueued to the state machine in arrival order; the first applicable transition wins,
  the loser is logged. Deadline competes in the same queue; a pending-confirm does not survive
  the deadline (deadline wins, `CONFIRM_IGNORED`).
- Confirm is idempotent: re-sending after reconnect is harmless (request-id correlation makes
  wire re-delivery idempotent too).
- panix-side rule: control decisions (commit/revert outcomes) come from `ctl` exit codes and the
  terminal state re-read from the log (via `inspect`), never from stream frames alone.

## 9. Streaming, attach, post-mortem

### 9.1 Live stream (the relay)

`follow` is deleted. The spawn exec IS the live stream: `start` becomes the PTY<->pipes relay
(6.1) and everything the guardian writes to the log (records and child output) flows to panix
over the exec's stdout in real time; panix's stdin carries command frames (8). This is the
real-time TUI narrative: activation output, check results, commit or revert steps.

Relay classification (EOF != death): on event-pipe EOF the relay reads the log tail ONCE and
classifies:

- a terminal record is present: map it to the outcome exit codes and exit;
- no terminal record: exit 6 (cancelled), so panix's existing inline-converge path (9.4) runs.

The relay NEVER converges and NEVER converges on a timer.

Outcome exit codes (the v1.3 viewer vocabulary, unchanged shape): 0 committed, 2 reverted,
3 revert_failed, 4 failed_precondition, 5 activation_exited (minimal), 6 cancelled; full
per-verb table in 9.5. panix reports accordingly and always includes the original error excerpt
and log path for failure cases. reportOutcome's exit-code switch keeps its shape; the relay maps
terminal records to the same codes.

### 9.2 Attach (after reconnect)

`panix-guard attach --dir <slot> --key <key> --from <offset>` is the reconnect viewer
(unchanged semantics): tails the log from the offset until a terminal record, with
replay-tolerant offset rules: if offset > file size (post-truncation), restart at 0 and let
seq-based dedupe suppress replays. Magic-tier confirmation after reconnect: `ctl` over a fresh
connection; the reachability proof and the confirm delivery are the same exec.

### 9.3 Converge (pre-start sweep and post-mortem decide, folded)

The v1.3 sweep/decide pair folds into one verb:
`panix-guard converge --dir <slot> [--key <key>] [--truncate]`. It self-locks (flock
nonblocking; busy => exit nonzero, no mutation), classifies from the log tail (the last writer's
key is recovered from the records themselves), converges a non-terminal transaction using the
TXN record's own embedded step lists (generation-arithmetic fallback when no usable TXN record
exists; no recomposition, 7), and truncates the log only with `--truncate` (the pre-start
caller; inline post-mortem keeps the log for reporting).

Callers:

- Pre-start: `start` runs the same convergence core in-process under its own lock, with
  truncation, before the fresh transaction begins (4.1, 6.1).
- Inline post-mortem (9.4): panix execs `converge` (no `--truncate`) when the guardian died
  mid-window.
- Standalone: the same verb serves operator debugging.

Classification and convergence behavior:

1. Parse the log tail (last 64KiB) for the last valid record.
2. Terminal state: with `--truncate`, clear the slot (truncate log, remove gc-root, remove the
   guardian binary; 10.3); without it, report and stop.
3. `COMMIT_START` without `COMMITTED`: converge the commit (re-run after a partial commit;
   idempotency contract 4.2).
4. `REVERT_START` without `REVERTED`: converge the revert.
5. `activating`/`activated`/`checking` with dead guardian: kill the logged child pid
   (`ORPHAN_KILLED` record) before converging, because an orphaned STC holds the global lock
   and the guardian's timeout died with it.
6. No log / no slot at all (pre-start death before spawn, or cleared tmpfs): generation
   arithmetic: profile current gen vs previous gen link; if the profile advanced without
   activation evidence, treat the previous generation as rollback target and revert the profile
   entry. On NixOS, additionally compare profile vs `/run/booted-system`: booted OLD + profile
   NEW => uncommitted => revert; booted NEW => committed => clear. User tiers: no booted-system
   evidence; bias-revert is the defined default (conservative direction).

The v1.3 defensive branch (non-terminal log AND a cmdline-verified live guardian pid) is
deleted: unreachable once mutations are lock-scoped, because the lock holder is by construction
the only mutator (4.1).

Mid-window reboot: NixOS boots OLD (bootloader untouched), profile untouched (profile-last),
transient slot cleared by tmpfs; the next deploy's pre-start converge sees a consistent machine.
User tiers: persistent slot; bias-revert per 9.3(6).

### 9.4 Panix-side guardian-death handling (mid-window)

When `ctl` returns 3 or the relay EOFs and the machine stalls without terminal records, panix
runs `panix-guard converge --dir <slot>` inline (synchronously over SSH, fresh connection),
reports the converged outcome, and exits; the lock releases when the dead processes' fds close
and converge self-locks (9.3). Added to the failure matrix.

`awaitTerminalOutcome` (post-disconnect outcome resolution) polls short `inspect` one-shots over
the master connection (cheap execs) instead of one long-held exec, until a terminal record
appears or the transaction's deadline passes; a still-non-terminal state converges via inline
converge and is reported from its records.

### 9.5 Exit codes (per verb)

Per-verb blocks, not one flat table: the same numeric code means different things per verb by
design. Per-verb scoping makes the verified v1.3 collisions explicit (3 = revert_failed
(viewer) / lock-held (probe) / guardian-dead (ctl); 5 = activation-exited (viewer) / refused
(ctl)). reportOutcome's give-up distinction is context-based (`panixGiveUpCause`), orthogonal to
exit codes.

start / attach (guardian outcome vocabulary; attach maps terminal records identically):

| Code | Outcome |
|---|---|
| 0 | committed |
| 2 | reverted |
| 3 | revert_failed (operator action required) |
| 4 | failed_precondition (zero effects) |
| 5 | activation_exited (minimal tier: activation ran, no commit/revert applies) |
| 6 | cancelled (no terminal state observed; outcome unknown; panix runs inline converge) |
| 64 | usage |

ctl:

| Code | Outcome |
|---|---|
| 0 | consumed/acked |
| 3 | guardian dead (liveness check failed) |
| 4 | slot or log missing |
| 5 | refused (CONFIRM_IGNORED, terminal-state ack, status-late ack after COMMIT_START; always on the auto tier) |
| 7 | ack-timeout (delivered, no ack within --wait; the request may still take effect) |

converge:

| Code | Outcome |
|---|---|
| 0 | converged to a terminal state (or the slot was already terminal and was cleared) |
| 3 | live transaction holds the lock (no mutation) |
| 1 | convergence failed; the log is left untouched |

inspect:

| Code | Outcome |
|---|---|
| 0 | verdict printed (one JSON object) |
| 4 | slot or log missing |
| 1 | log unreadable or unparseable beyond noise |

lock-probe (legacy @PG1 binary only; the v2 binary deletes the verb and `inspect` carries the
lock state):

| Code | Outcome |
|---|---|
| 0 | free |
| 3 | live |

## 10. Slot layout and platform contracts

### 10.1 Slot directories

| Tier | Slot root |
|---|---|
| system (nixos, systemConfigs on NixOS) | /run/panix-guard |
| darwin | /var/root/panix-guard (root's home; avoids world-writable parents) |
| user (home-manager etc.) | ~/.local/state/panix-guard |
| nix-on-droid | ${TMPDIR:-$HOME/.local/state}/panix-guard |

Slot-layout version bump: v2 slots live under a version component under each root:
`<root>/v2/<name>` (for example `/run/panix-guard/v2/nixos-profile-...-<hash>`). Legacy (@PG1)
slots under the plain root become inert.

Legacy handling at pre-start: if the legacy slot dir (same root and name without the `v2`
component) exists, probe liveness via the legacy guardian's own `lock` verb (a fresh open plus a
transient nonblocking flock, safe on a fresh OFD; the never-LOCK_UN rule binds only inherited
fds): held => fail fast ("live legacy transaction; old panix version active"); free => remove
the legacy slot tree (log, gc-root symlink, persisted guardian binary) and proceed. NEVER parse
@PG1 content. Stale `/nix/var/nix/gcroots/auto` entries pointing into the removed tree
self-clean when their target vanishes. Implemented as the pre-start legacy sweep (Stage 1, 17).

Slot contents (exactly): `log`, `gc-root` (symlink, GC root), `panix-guard` (the persisted
guardian binary); the latter two removed at slot clear (10.3).
One slot per profile: slot name = slug(profile path) + short hash of the profile path (two
profiles with the same basename must not collide). A NixOS guard and a home-manager guard
coexist without clashing (different dirs); same-slot concurrency fails fast via flock.
Cross-tier parallelism is state-safe; sequencing of multiple installables against one machine is
an orchestration concern (T3 pins this down). User-tier persistent slots: the next pre-start
converge clears terminal slots (tmpfs-clearing does not apply).

### 10.2 KillUserProcesses contract (verified V7)

Mechanism (logind source, `session_stop_scope`): on session close, logind issues a StopUnit on
`session-<id>.scope`; systemd terminates the entire session-scope cgroup. `Setsid` changes
session/pgid, never cgroup membership, so a Setsid'd guardian inside the session scope IS killed.
`KillUserProcesses=yes` is the upstream default since v230; NixOS defaults to `no` but users can
override, and per-user records (homectl `kill_processes`) can force it regardless.

Contract (product decision: hard fail): the probe runs on EVERY Linux target. Probe:
`busctl get-property org.freedesktop.login1 /org/freedesktop/login1 org.freedesktop.login1.Manager
KillUserProcesses` (busctl ships with systemd itself; present on NixOS under
`/run/current-system/sw/bin`). Verdicts: `no` => proceed; `yes` => hard fail guarded tiers with
remediation guidance `KillUserProcesses=no`; property/bus unavailable (dbus stopped, no systemd)
=> unknown => fail closed for guarded tiers. Config-file parsing is NOT a valid fallback (misses
drop-ins, the compile-time default, `KillOnlyUsers`/`KillExcludeUsers` layering, per-user
overrides). Lingering does not fix this (per-session-scope kill) and must not be suggested as a
remedy. Deferred option (documented, not built per the no-systemd directive): on systemd targets
a `systemd-run --scope` re-exec would escape the session cgroup and lift this restriction; it is
a platform-conditional path and stays out of v1.

### 10.3 Guardian binary contract

One binary, stdlib-only, CGO-free, per-arch (linux/darwin x amd64/arm64), gzip-embedded into
panix at build time (flake derivations), selected by target arch, transferred idempotently per
deploy (sha256 probe skips unchanged targets; atomic `cat > tmp && mv`; safe to overwrite a
running binary, 4.1), never installed. Cleanup owner: the NEXT pre-start converge after
classification (post-terminal attach/ctl still need the binary). Slot clear = truncate the log +
remove the gc-root + remove the binary (v1.3 left binary removal unassigned: nothing removed
it). Missing embed (dev builds): fail loudly at startup. Platform decisions are
runtime-detected; no systemd code paths exist in the guard.

### 10.4 Log-unwritable behavior

- At start: the log cannot be opened => no lock => hard abort before any mutation (zero
  effects).
- Mid-window: continue; the wire informs panix; the deadline converges; post-mortem falls to
  generation arithmetic with a loud "outcome unknown" (9.4).

## 11. Failure matrix

| Event | Outcome |
|---|---|
| activation exit != 0 | guardian reverts immediately (fast path), original error surfaced |
| activation hangs | in-process timeout kills child group, revert |
| activation kills sshd/network | relay dies with the exec; panix reconnects, attach; no confirm by deadline => guardian reverts; machine heals itself |
| services broken, ssh alive | local checks (unit baseline, user checks) fail => no commit => revert; reported outcome = checks failed |
| machine reboots mid-window | NixOS profile-last: boots OLD, profile untouched, next pre-start converge clears. boot mode: boots NEW (documented exception). User tiers: bias-revert per 9.3(6). darwin: profile-last makes boot-time activate-system activate OLD (safe; V2 verifies) |
| panix crashes mid-deploy | deadline revert (magic) or self-commit (auto); dead-man property |
| panix dies after sending confirm | guardian commits; next deploy's converge reports committed; deploy outcome was unknown at the time |
| wire link dead mid-window (LINK_DOWN) | degraded control only: reconnect via attach + ctl (signals); deadline sole authority |
| guardian dead, relay alive | relay classifies (9.1): terminal record => outcome mapped; none => exit 6 and panix runs inline converge (9.4) |
| relay and guardian both dead | no live window left; the next pre-start converge sweeps and converges |
| log unwritable at start | no lock => hard abort before any mutation, zero effects (10.4) |
| log unwritable mid-window | wire informs; deadline converges; post-mortem falls to generation arithmetic with a loud "outcome unknown" (10.4) |
| wire dead AND log unwritable | no control path at all; deadline sole authority; outcome unknown until generation arithmetic |
| guardian killed (OOM/admin) | child orphaned; converge kills it, converges from the TXN record or gen arithmetic; panix inline converge per 9.4 |
| guardian SIGTERM/SIGINT | revert-request semantics (6.1); orderly, tracked |
| revert itself fails | retry with backoff; then REVERT_FAILED; optional gated reboot |
| commit `--set` fails | nothing committed; revert = OLD switch |
| commit `boot` fails | --set OLD + OLD switch; failed gen deleted |
| confirm/revert-request after COMMIT_START | finish commit; ack record with status late (8); ctl exits 5 |
| panix gives up mid-window (cancel) | one best-effort fresh-connection `ctl revert-request --cause cancel` (8); late-ack after COMMIT_START, no-op when terminal; the guardian converges autonomously. The deployer reports the cancel cause contextually in its outcome; the record carries the revert's own reason: a record-level cause is unimplementable over payload-less signals without the rejected log-append design |
| GC (-d / auto-GC) during window | profile-last: OLD current (kept), NEW rooted. self-setting/boot: OLD rooted, NEW current-or-uncommitted |
| concurrent deploy to same profile | flock fail-fast with pointer |
| start dies after the boot-mode pre-start set, before the guardian detaches | the lock dies with start; the next pre-start converge's generation arithmetic restores the profile entry |
| forged/lost records | nonce gating; state re-derived from heartbeats; ctl exit codes decide |
| signal before handlers / during converge | ordering rules in 8; pending-confirm semantics |
| KUP=yes target | hard fail at pre-start (10.2) |

## 12. Security notes

- Markers are nonce-gated (deploy key). Activation output cannot forge control-relevant facts
  accidentally; deliberate root-level forgery is out of scope (such a script owns the machine).
- The confirm handshake authorizes commit from any connection that can signal the guardian
  (root on target). This is equivalent to the existing sudo/su trust model.
- The guardian binary is transferred over the operator's own SSH as content owned by the
  executing identity, 0700 slot; world-writable locations are never used (darwin slot in
  /var/root).

## 13. Test matrix

Unit (Go, no VM): guardian state machine against stub `switch-to-configuration`/`nix-env`
binaries recording invocations (scriptable fail/hang/slow); codec golden tests plus adversarial
parsing (torn/truncated/oversized/foreign-sentinel lines); fold table; relay classification
table (event-pipe EOF x log state: terminal present, non-terminal, empty, unreadable); TXN fold
(embedded step lists survive the round trip); per-verb exit-code matrix (start/attach, ctl incl.
ack-timeout, converge, inspect, lock-probe); CLOEXEC leak regression (the activation child
inherits none of the lock/command/event fds); nil-stdio re-exec guard (a re-exec with nil stdio
fails loudly, never /dev/null); same-inode truncation pin (ftruncate keeps the inode; rename is
rejected by the test); signal ordering (early signal, double signal, signal during converge,
SIGTERM semantics, deadline vs pending-confirm); cap/truncation incl. concurrent burst and wire
backpressure (bounded drop-oldest buffer: the state machine never blocks, drops logged); attach
offset rules across truncations; converge classification table (no invariant-violation branch:
deleted with lock-scoped mutations); commit/revert transaction argv per tier and mode (golden
argv tests for the full pre-start compound script per tier x mode); KUP probe; config validation
rules; exec option unit tests (Quiet, WithTimeout, FreshConnection, WithStdin, WithOutputTap).

e2e (existing VM harness + testflakes; the existing leg set re-runs at every migration stage,
plus the new legs): exit-code failure (assertions updated: generation list unchanged after
revert); hang; sshd-kill; network-break; `nix-collect-garbage -d` mid-window; panix killed
before confirm (deadline revert + converge); orphaned child kill; concurrent deploy fail-fast;
retry after revert; truncated log + attach resume; standalone `panix rollback`; home-manager
tier leg; original-error-text assertion (not "reverted"); reboot_on_revert_failure leg; KUP=yes
hard-fail leg; ctl ack latency regression guard (master vs fresh conn); panix-dies-after-confirm
convergence leg. New legs: link-down mid-window; guardian-death-with-relay-alive; graceful
cancel (the give-up path's one cancel revert-request); torn JSONL tail; mixed @PG1/@PG2 tail
(legacy upgrade); stdin forwarding through su -l/sudo (non-interactive elevation prerequisite:
guarded deploys require non-interactive elevation; sudo without a tty fails fast on a password
prompt, su -l without a remote tty would eat command frames); relay-EOF-with-terminal-record
(the false-revert guard, highest-consequence test); guardian startup panic (top-level recover
emits the terminal error record log-first); pre-readiness command frame (panix writes nothing
before the first wire record); echo self-ack never (echo defense, 8); slow-reader backpressure
(the transaction never stalls, drops logged); legacy-slot upgrade incl. gc-root and binary
cleanup; relay spawn-failure; ack-timeout code; boot-mode guarded leg (covers the previously
unimplemented boot-mode pre-start set).

CI: compile gates for all four GOOS/GOARCH targets (land with T1, not after); embed-present check
in release artifacts.

## 14. Verifications (V1-V7: CLOSED in v1.2)

- V1: CLOSED-AMENDED. `bin/activate` does NOT self-set the profile (engine `activate` touches
  etc/tmpfiles/services only; `register` does profile + gcroot). systemConfigs stays
  profile-last; commit = `bin/register-profile` (4.2); revert = OLD register-profile + OLD
  activate (the state-file diffing makes re-activation a true revert).
- V2: CLOSED. Activate is profile-agnostic; each generation ships a re-runnable activate whose
  current-system diffing is a real revert; boot daemon under profile-last re-points to OLD
  (mid-window reboot safe).
- V3: CLOSED. STC needs no inherited PATH (absolute paths, hermetic activation PATH, `-ng` talks
  D-Bus). Caveats folded into 5: `.pl` ignores stop-batch failures; `-ng` requires
  `/run/current-system/sw/bin` + `LOCALE_ARCHIVE`. Guardian sets a conservative PATH for user
  hooks (6.4). Lock path + exit-code table pinned to nixos-25.05 in the test matrix.
- V4: CLOSED. darwin flock(2) is OFD-scoped ("multiple references to a single lock"), survives
  execve with inherited fds; guardian MUST pass the locked fd via `cmd.ExtraFiles` and the child
  must never `LOCK_UN` (an explicit unlock in a forked child kills the parent's lock).
- V5: CLOSED. Non-root indirect roots are a first-class daemon op (`AddIndirectRoot`, no
  trusted-user required); auto entry = sha1(path) so re-registration rewrites the same entry;
  the client makes the direct symlink (parent-dir writability required; root on /run: moot);
  hard-fail policy: daemon-op failure aborts the window before commit.
- V6: CLOSED. Missing generation = silent exit 0 (modern nix and 2.3 alike); only
  current-generation deletion errors; never string-match error text; any nonzero is a real
  failure.
- V7: CLOSED-AMENDED. Kill = StopUnit on `session-<id>.scope` (whole session-scope cgroup;
  Setsid insufficient). Probe via busctl confirmed correct and present on systemd systems.
  Hard-fail contract stands (product decision); systemd-run `--scope` escape documented as a
  deferred option (10.2). Config parsing rejected as fallback.

## 15. Build contract (summary; details in T7)

Guardian derivations per target; flake copies them into the embed dir and gzips before `go
build`; every install channel (flake, release tarball, task build) must embed; release checklist
verifies embedded guardians; dev builds fail loudly when the embed dir is empty.

## 16. Panix-side implementation notes (verified repo facts, feeds T1/T3)

- TUI streaming: incremental by construction (PTY 8KiB chunks -> CommandLog -> 60fps throttled
  version-diffed re-render); a 16-minute streaming exec renders live. `MaxOutputLines` trimming
  is
  opt-in per exec via `Trim()` (activation execs opt in today, default cap 10000); trimming
  drops the OLDEST lines and never blocks live rendering. The relay exec must NOT opt into
  Trim (or uses a raised cap) so the narrative survives attach replays.
- Exec option gaps for T1 (verified absent today; v2 note: WithTimeout, Quiet, FreshConnection,
  WithStdin, WithOutputTap, and Pty.SetEcho have since landed, so this records the verified v1
  state they were specified from): per-command timeout override (`ExecOptions`
  has no timeout field; `shellStream` uses `conf.Timeout` only), `Quiet()` no-log path (every
  `Exec` creates a PhaseLog command entry; ctl/attach/poll execs would flood phase logs and the
  TUI), `FreshConnection()` (`controlMasterArgs` applies `ControlMaster=auto` +
  `ControlPersist=60` unconditionally; fresh args (`-o ControlMaster=no -o ControlPath=none`)
  must be appended AFTER `MaybeSSHCommandArguments` to override).
- Binary transfer: `routeDestination`/`pipeCommands` apply no privilege wrap (verified); compose
  the wrap at the PipeSpec call site like `transferCommandPipeSpec` does (`Write:` and `Probe:`
  prefixed with `WrapAsTargetUser`/`MaybeSudoFor`; elevation prefix outside `sh -c`; su -l takes
  the whole script as the single quoted `-c` string; stdin flows through unchanged).
- Env: `WithEnv` argv prefix survives direct exec, sudo env_reset, su -l login reset, and ssh
  re-parsing; use it for the guardian spawn's conservative PATH. Never rely on PATH under the
  sudo path or the raw ssh non-interactive shell; invoke everything by resolved absolute path
  (`resolveCommandPath` before elevation).
- Tier class: new type-level Preset field next to `IsBootstrappable` (precedent
  `IsBootstrappableType`); `rollback` attribute lives in config/attributes replacing AutoRollback
  (attributes.go:31), string enum with zero-value = inherit (mergo non-pointer constraint holds
  for string enums); legacy `auto_rollback` rejection is a schema/validation addition.
- Mode precedence: per-installable mode resolution already exists (installable.yaml override ->
  preset default -> CLI override) in `executeActivation`; tier routing composes with
  `NonMutatingModes`/`ProfileSkipModes` there. CLI overrides follow the RollbackFlags ->
  handler-field pattern (flags.go:60-62, workflow.go:76-79).

## 17. Migration (three stages)

- Stage 1: NDJSON @PG2 codec full-stack (IMPLEMENTED). Signals, ctl, follow, lock, and decide
  keep working on the new records; coexistence is decided here: slot-layout version bump, no
  dual-read of @PG1; the pre-start sweep gains legacy cleanup (gc-root + persisted binary
  removal, 10.1).
- Stage 2: relay duplex + event tap + TXN embedding + mutations-under-lock + pre-start reorder.
  The Executioner primitives (WithStdin pump, WithOutputTap, Pty.SetEcho) are implemented; the
  wiring is not.
- Stage 3: deletions (follow, live-path signals, text-codec remnants, deployer-side tail
  parsing, the 9.3 defensive branch, the lock verb, the decide verb) + inspect + converge +
  per-verb exit codes + the full e2e leg matrix.

## Revision history

- v1: initial contract from design dialogue.
- v1.1 (adversarial gate): flock open-file-description inheritance model (viewer may die; lock
  survives via guardian); `follow` subcommand mandated; sweep-invoked decide uses
  `--assume-locked`; REVERT_START ordered before revert steps; SIGTERM/SIGINT semantics; full
  transition table; per-class commit/revert tables replacing NixOS-hardcoded commands; gc-root
  rename and authoritative target table; child process management (pipes, serialized log writer,
  Setpgid group kill); error excerpts in terminal markers; seq continuation across writers; start
  argv contract; slot name derivation; panix inline decide on guardian death; builtin
  failed-units-delta check defined; minimal-tier state machine; KUP probe on all Linux targets
  with busctl method and fail-closed; darwin slot moved to /var/root; idempotency contract;
  failure matrix corrections and additions; validation constants (30s per-check, 5s margin,
  budget measured from ACTIVATED); commit_budget max(120s, activation_timeout/2); V1-V7 open
  verifications (V1/V2 gate tier classes); filed test issues folded into section 13.
- v1.2 (verification round): V1-V7 closed. V1: systemConfigs commit via `bin/register-profile`
  (profile + system-manager's own gcroot), tier stays profile-last. V3: STC runtime caveats
  (.pl stop-batch ignored, -ng requires /run/current-system/sw/bin + LOCALE_ARCHIVE) and
  conservative child PATH. V4: lock fd passed via ExtraFiles; child never LOCK_UN. V5: daemon
  AddIndirectRoot needs no trusted-user; sha1 entry reuse; hard-fail on registration failure.
  V6: delete-generations silently succeeds on missing gen. V7: KUP verified as session-scope
  StopUnit (Setsid insufficient), busctl probe, fail-closed, systemd-run --scope escape
  documented as deferred option. New section 16 (panix-side verified facts for T1/T3).
- v1.3 (T2b argv amendment): spawn argv gains `--profile`, `--nix-env`, `--activation-argv`,
  `--commit-argv`, `--revert-argv`, `--invariant-target`, and the runtime-derived revert rules
  (profile restore and generation cleanup derived from the profile's current state, no extra
  flags); commit/revert/revert step lists are panix-composed per class and mode (preset
  knowledge stays panix-side); 4.2 revert steps updated for systemConfigs register-profile.
- v2.0 (duplex relay amendment): NDJSON sentinel-JSON records (@PG2) replace the @PG1 text
  codec; state is the fold of the event stream. The spawn exec becomes a full-duplex control
  relay (zero additional connections in the live window): commands travel as stdin wire frames,
  events as an Executioner output tap; signals plus cmdline+key forensics are deliberately kept
  as the degraded control path (pid-reuse safety). Mutations move under the guardian's lock
  (converge + gc root + boot-mode pre-start set), fixing the unimplemented boot-mode pre-start
  set and the sweep mode-drift defect; the TXN record embeds the transaction's own step lists so
  post-mortem consumers never recompose. start absorbs follow (the relay is the live stream) and
  runs the pre-start convergence in-process; converge and inspect consolidate post-mortem
  parsing guard-side (inspect absorbs the lock verb and all deployer-side tail parsing); the 9.3
  defensive branch is deleted as unreachable. In-place log truncation pinned (rename
  prohibited); legacy slots become inert under the v2 slot layout with lock-probe-based legacy
  cleanup; guardian binary cleanup assigned to the next pre-start converge; per-verb exit-code
  tables (ack-timeout split from refused); relay EOF classification rule (EOF != death);
  log-unwritable behavior split start vs mid-window; migration staged in three stages; test
  matrix extended.
