---
description: "Start the dashboard server and mock-oauth-proxy via the ship-status-dev MCP tool"
---

# Ship Status dev — serve

Use the **`dashboard_serve`** MCP tool (server: **`ship-status-dev`**). Do not run the dashboard or mock-oauth-proxy manually — the MCP tool handles migrations, HMAC secret generation, background process management, log routing, and duplicate detection.

The tool starts:
- **Dashboard** on port **8080** (public, no auth)
- **Mock OAuth Proxy** on port **8443** (protected, credentials: `developer:password`)

If the server is already running, the tool will report it. Ask the user if they want to restart, and if so call again with **`restart=True`**.

Logs: **`ship-status-dev-logs/dashboard_serve.log`** and **`ship-status-dev-logs/mock_oauth_proxy.log`**.

See **`mcp/server.py`** for all parameters.
