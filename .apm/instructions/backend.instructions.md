---
description: "Go backend coding standards for Ship Status Dashboard"
applyTo: "**/*.go"
---

* Follow idiomatic Go practices.
* After making changes, always run `gofmt -w` on modified files to ensure proper formatting.
* Use GORM conventions for database models and queries.
* Authentication uses HMAC signature verification — never bypass `SKIP_AUTH` in production paths.
* Outage modifications must go through the audit logging system (`outage_audit_logs` table).
