# Move relay DNS resolution out of `models.init()` into `croc.New`

## Context / root cause

On every crocson launch (any platform; captured on Android), the app issues DNS
queries for `croc.schollz.com` (A + AAAA) and `croc6.schollz.com` (A + AAAA)
**before** `main()` runs.

Root cause: `abakCroc/croc/src/models/constants.go` — the package `init()` used to
call `lookup(DEFAULT_RELAY)` then `lookup(DEFAULT_RELAY6)`. crocson imports the
`models` package (via `cli`/`croc`/`tcp`/`utils`), so this `init()` ran at program
start. `lookup()` → `localLookupIP` → `net.Resolver.LookupHost` issues A + AAAA
per host. (Besides the privacy/leak concern, this is also fragile on Android — a
potential unrecovered panic / init-time block in sandboxed test environments.)

The pre-resolution exists only to support `--internal-dns` (croc's built-in DNS
stub that bypasses the OS resolver). `--internal-dns` is **not** part of
`croc.Options`, is never passed to clients, and crocson never uses it. So the
startup lookup is pure overhead for crocson.

## Decisions

- Remove the `lookup(...)` calls from `models.init()` (DONE). `DEFAULT_RELAY` /
  `DEFAULT_RELAY6` stay as hostnames permanently.
- Do the relay resolution **lazily in `croc.New`**, the single client constructor.
  `NewCtx` calls `New(ops)` (`ctx.go:49`), so this uniformly covers the CLI
  (`croc.New`, cli.go) **and** crocson (`NewCtx` → `New`).
- Gate the resolution on `models.INTERNAL_DNS` (covers both the `--internal-dns`
  flag and the config file persisted via `--remember` — confirmed desired).
- Export a helper `models.ResolveRelay(addr)` that wraps the private `lookup` +
  port handling (DONE). Keep `lookup`/`localLookupIP`/`remoteLookupIP` private.
- **No changes in crocson.** crocson never sets `INTERNAL_DNS`, so no startup DNS
  occurs; its relay hostname is resolved by `net.Dial` at connect time
  (`croc.go:1081-1101`).

## Consistency check

- `New` runs after the CLI's option munging (comparison `cli.go:341` and
  remembered-options happen before `croc.New` is called), so `New` receives the
  already-finalized `RelayAddress`/`RelayAddress6`.
- `Send()` (connection loop at `croc.go:816`) and `Receive()` (loop at
  `croc.go:1081`) do **not** wipe `RelayAddress` before the loop in the normal
  relay case: `Send` sets `127.0.0.1` only inside its local-relay helper;
  `Receive` clears `RelayAddress` only when `OnlyLocal`/`IP` is set
  (`croc.go:985-991`). So a value resolved in `New` reaches both loops.
- Comparisons `X != models.DEFAULT_RELAY`/`DEFAULT_RELAY6`
  (`cli.go:341,343,634,636,752,757`; `croc.go:766`) now compare
  hostname-vs-hostname (flag default `Value: models.DEFAULT_RELAY` at
  `cli.go:134-135` is a hostname too) — still correct.
- The connection loop `croc.go:1081-1101` already handles a bare hostname
  (`SplitHostPort` fails → host=addr, port defaults to `9009`, then `net.Dial`
  resolves). With `--internal-dns`, `New` pre-resolves to `ip:port` so `net.Dial`
  connects to the IP without OS DNS — same end result as the old `init()`.

## Status (fork `../abakCroc/croc`)

Implemented & verified:
- `src/models/constants.go` — `init()` no longer resolves; `ResolveRelay(addr)`
  exported helper added.
- `src/cli/cli.go` — no resolution injected (calls `croc.New` directly at both
  send/recv sites).
- `src/croc/croc.go` — resolution lives in `New` right after `c.Options = ops`,
  gated on `models.INTERNAL_DNS`.
- Verified: `go build ./...`, `go vet`, `gofmt`, `go test ./src/models/` (fork);
  `go build ./...` (crocson).

## Remaining work

### 1. Restore the two trace logs lost in the refactor (fork, immediate)

In `src/croc/croc.go` `New`, inside the existing `if models.INTERNAL_DNS { ... }`
block, add the trace lines that used to live in `models.init()` (now logging the
resolved addresses):
```go
if models.INTERNAL_DNS {
    c.Options.RelayAddress = models.ResolveRelay(c.Options.RelayAddress)
    c.Options.RelayAddress6 = models.ResolveRelay(c.Options.RelayAddress6)
    log.Tracef("Default ipv4 relay: %s", c.Options.RelayAddress)
    log.Tracef("Default ipv6 relay: %s", c.Options.RelayAddress6)
}
```
Originals were `log.Tracef("Default ipv4 relay: %s", addr)` /
`log.Tracef("Default ipv6 relay: %s", addr)` in the old `init()`; `addr` maps to
`c.Options.RelayAddress` / `RelayAddress6` here. `log` is already imported in
`croc.go`. (These only emit under `INTERNAL_DNS` — i.e. CLI `--internal-dns` or
the GUI toggle below — since resolution now only happens there. If an always-on
"which relay is this client using" trace is wanted instead, log the addresses
outside the gate.)

### 2. GUI toggle to enable `models.INTERNAL_DNS` from crocson (crocson feature)

Let crocson opt into croc's internal DNS stub resolver from the GUI (replicating
`croc --internal-dns`), still only at send/receive time (not startup):

- Settings tab: add a bool preference `internal-dns` (default false) bound to a
  checkbox near the relay/network options (see `settings.go` form items, e.g.
  ~`settings.go:463`).
- Before creating the client, set the croc global from the preference:
  ```go
  models.INTERNAL_DNS = a.Preferences().Bool("internal-dns")
  ```
  at the send handler (`send.go`, just before `crocNew` ~`send.go:1382`) and the
  recv handler (`recv.go`, just before `crocNew` ~`recv.go:963`). Add
  `import "github.com/schollz/croc/v10/src/models"` to those files.
- The gate is in `croc.New` (reached via `NewCtx`→`New`), so setting the global
  makes the GUI resolve the relay via the built-in public-DNS stub at send/recv.
  `models.INTERNAL_DNS` is a process-global; crocson is single-process, so safe.
- No startup DNS introduced: resolution still runs only at `croc.New` (send/recv).
- Note: `models.init()` still parses `--internal-dns`/`--remember`/config file;
  the crocson pref overrides it for GUI clients.

## Risks / edge cases

- `--internal-dns` now resolves per `croc.New` (i.e. per send/receive) instead of
  once at package load — negligible.
- Persisted `--internal-dns` via `--remember` (config file) still works because
  the gate is `models.INTERNAL_DNS` (set from the config file in `init()`), not
  the raw flag.
- Croc CLI users of this fork who rely on the default-relay pre-resolution get
  the same runtime behavior (relay still resolves, just lazily and only under
  `--internal-dns`).
- Minor pre-existing nuance (unchanged by this move): under `--internal-dns`, the
  resolved IP differs from the hostname `DEFAULT_RELAY`, so the "receive command"
  flag check at `croc.go:766` may include `--relay <ip>`. This is identical to the
  previous `cli.go` placement; not introduced by the move.

## Validation

- Fork: `go build ./...`, `go vet ./src/croc/... ./src/cli/... ./src/models/...`,
  `gofmt -l src/croc/croc.go src/cli/cli.go src/models/constants.go`, and
  `go test ./src/models/` (lookup helpers still covered by `models_test.go`).
- crocson: `go build ./...` (its working `go.mod` replaces `croc` →
  `../abakCroc/croc`); launch on Android (or any OS) — DNS capture must show
  **no** queries to `croc.schollz.com` / `croc6.schollz.com` until a send/receive
  is started.
- crocson send/receive still works (relay resolves via `net.Dial` at connect).
- Croc CLI: `croc --internal-dns relay` resolves via the built-in public DNS list
  (now at `croc.New` time); without the flag it uses the OS resolver.

## Out of scope

- Crocson source changes other than the `internal-dns` toggle in task 2.
- Removing the now-startup-inert `--internal-dns`/`--remember` parsing in
  `models.init()` (kept so `INTERNAL_DNS` stays set for the gate).
