---
description: "MCP server (ship-status-dev) for AI-callable dev tasks"
applyTo: "mcp/**"
---

Shared MCP server for AI-callable dev tasks (migrate, serve, test, monitor). Configuration, tool list, and extension notes are in `mcp/server.py`.

When adding or modifying MCP tools, follow existing patterns in `server.py` (`_spawn_background`, `_run_foreground`, `_find_pids`, `_repo_path`, `_ensure_dev_log_dir`). Restart the MCP server after changes.
