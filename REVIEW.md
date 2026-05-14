---
pr: openshift-eng/ship-status-dash#89
title: "TRT-2363: Add GET /api/outages/during for overlap queries"
head_sha: 0b4f795c9d4bd3940a589d2f9b2c2e8767e73c26
base: main
reviewed_at: 2026-05-14T11:09:57Z
verdict: approve
---

## Findings

### [should-fix] Deleted tests for GetComponentBySlug and GetSubComponentBySlug
- where: `pkg/types/config_test.go` (entire file rewritten)
- concern: Old direct tests for `GetComponentBySlug` and `GetSubComponentBySlug` covered edge cases (empty list, not found, case sensitivity). Replaced with `SubComponentRefsMatching` tests only. The lookup methods are still public API used in handlers. Indirect coverage exists but dedicated edge-case tests are lost.

### [should-fix] Multiple h.config() calls per request risk config-reload inconsistency
- where: `cmd/dashboard/handlers.go:658-675`, `cmd/dashboard/handlers.go:681-736`
- concern: `ListSubComponentsJSON` and `GetOutagesDuringJSON` call `h.config()` once for `SubComponentRefsMatching`, then again for each ref via `GetComponentBySlug`/`GetSubComponentBySlug`. Config reload between calls could cause refs to point to removed slugs. Nil guards prevent crashes but items silently drop. Fix: `cfg := h.config()` once at handler entry.
- excerpt: |
    refs := h.config().SubComponentRefsMatching(componentSlug, "", tag, team)
    for _, ref := range refs {
        component := h.config().GetComponentBySlug(ref.ComponentSlug)

### [nit] Unreachable test case name is misleading
- where: `pkg/utils/query_time_test.go:76-78`
- concern: `empty_both_hits_end_branch` passes empty strings for both start and end. Handler rejects this before calling `OutagesDuringQueryBounds`. Tests `ParseRFC3339OrNanoUTC("")` error path, which is fine, but name implies a reachable scenario.

### [question] Unbounded query window on a public endpoint
- where: `cmd/dashboard/handlers.go:708-727`, `pkg/repositories/outage_repository.go:163-183`
- concern: No limit on time window size or number of refs in the DB query. A caller could request a century-wide range with no component filter, scanning the full outages table. Is the table expected to stay small enough, or should there be a cap (max duration or LIMIT)?

## Checked
- SQL safety: WHERE clause uses `?` placeholders only; refs pre-validated against config slugs
- Route ordering: `/api/outages/during` registered before parameterized `/api/components/{componentName}/...` routes
- Overlap query logic: `start_time <= queryEnd AND (end_time IS NULL OR end_time > queryStart)` correct for open-ended and point-in-time queries
- StatusFromOutages: identical to deleted `determineStatusFromSeverity`, tests match 1:1
- SubComponentRefsMatching: AND semantics correct and consistent with pre-refactor behavior
- E2e tests: instant query, range query, non-overlapping window, 400 on missing params, tag+team filtering
- MCP venv fix: `rm -rf` + fresh venv with `chmod -R u+w` guard; reasonable

## Open questions
- Is the outages table expected to stay small enough that unbounded time window + all sub-components is acceptable, or should a cap be added?
- Was removal of `GetComponentBySlug`/`GetSubComponentBySlug` tests intentional or an oversight during config_test.go rewrite?
