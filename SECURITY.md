# Security Policy

## Reporting a vulnerability

Please don't open a public issue. Report privately through GitHub: go to the [Security tab](https://github.com/jeefy/convergence/security/advisories/new) and choose "Report a vulnerability".

Include what you found, how to reproduce it, and the version (`convergence version`). We'll acknowledge the report, keep you updated, and credit you in the advisory unless you'd rather stay anonymous.

Issues in the bundled components belong upstream:

- Stand-In: <https://github.com/jeefy/stand-in>
- Pinball: <https://github.com/jeefy/pinball>
- agentgateway: <https://github.com/agentgateway/agentgateway>

If you're not sure where it belongs, report it here.

## Supported versions

Only the latest release gets fixes before 1.0.

## Scope notes

Some behavior is by design and documented in the README's security model: Stand-In is fail-closed by default (fail-open is opt-in), the gateway has no authentication of its own, scrubbing is pattern-based and won't catch everything, and credentials are stored in plaintext `0600` files under `~/.convergence`. Reports that show these protections failing beyond what's documented are in scope.
