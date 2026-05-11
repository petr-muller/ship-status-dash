---
description: Run e2e tests only
---

# Ship Status dev — e2e

Run e2e tests directly:

```bash
make local-e2e
```

**Do not modify the command or tail it.** The e2e script manages its own PostgreSQL, dashboard, mock-oauth-proxy, component-monitor, and test execution. It auto-detects whether to use native binaries or containers.

**Never run e2e more than once per request.** The tests take several minutes. Run once, capture the output, and read it for results.

## Presenting results

After the run completes, read the full output and present every test with its result and timing. Format as a list:

- ✓ TestName (1.23s)
- ✗ TestName (0.45s)

Show all tests, not just failures. Include the total pass/fail count and overall duration at the end.