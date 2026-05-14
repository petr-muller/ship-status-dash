---
pr: openshift-eng/ship-status-dash#88
title: "TRT-2656: Resolve ongoing outage when sub-component is removed from the config"
head_sha: 871a821251c147a7040eb0df6a2f0c56d92483b5
base: main
reviewed_at: 2026-05-14T11:45:37Z
verdict: approve
---

## Summary

- On startup and each config hot-reload, active outages for sub-components no longer in the config are resolved via `ResolveActiveOutagesForMissingSubComponents`.
- `NewDBOutageManager` return type changed from `OutageManager` interface to concrete `*DBOutageManager` so the new method is callable from `main.go`.
- Config manager gains `reloadCount atomic.Uint64`, incremented after callbacks; `ConfigReloadedMessage` log moved to after callbacks complete.
- E2e reload detection switched from `--since-time` log parsing to `--tail` + monotonic `reload_count` comparison.
- New `Boskos` component added to e2e config for orphan-outage test scenario.

## Findings

### [nit] redundant summary log after per-outage logs
- where: `pkg/outage/config_reload_resolve.go:73-75`
- concern: Each resolved outage already gets an individual Info log at line 67-71. The summary log at line 73 is slightly redundant. Not harmful, but adds noise when the individual logs are already present.
- excerpt: |
    if resolved > 0 {
        entry.WithField("count", resolved).Info("Resolved active outages for sub-components removed from configuration")
    }

### [nit] ResolveActiveOutagesForMissingSubComponents not on the OutageManager interface
- where: `pkg/outage/outage_manager.go:16-28`, `pkg/outage/config_reload_resolve.go:40`
- concern: The new method lives only on the concrete `*DBOutageManager`, not on the `OutageManager` interface. This is fine today since only `main.go` calls it and holds the concrete type. If a mock-based test or another caller ever needs it, they would need the concrete type too. Consider adding it to the interface for symmetry, but not blocking.

### [nit] typo in PR description
- where: PR description body
- concern: "phatom" should be "phantom".

## Checked

- Key format in `configuredSubComponentKeys` uses `c.Slug`/`sub.Slug` which matches `Outage.ComponentName`/`Outage.SubComponentName` as populated by `component_monitoring_report.go` and `absent_report.go` and `handlers.go` -- all store slugs.
- `GetAllActiveOutages` query (`end_time IS NULL OR end_time > now()`) matches the existing `GetActiveOutagesForComponent` active-outage definition.
- `UpdateOutage` goes through audit logging -- audit trail for automated resolutions is preserved with `dashboard-config-reload` user.
- Nil-config produces empty allowed set, resolving all outages -- correct defensive behavior.
- Nil-component pointer guard in `configuredSubComponentKeys` loop.
- `reloadCount` uses `atomic.Uint64` correctly -- incremented outside the mutex, after callbacks, before the log message.
- E2e poll interval is 10s (`--config-update-poll-interval 10s`), well within the 90s timeout in the new test.
- No security concerns: no auth bypass, no injection vectors, audit trail maintained.

## Open questions

- Is there value in adding `ResolveActiveOutagesForMissingSubComponents` to the `OutageManager` interface for testability, or is keeping it on the concrete type intentional?
