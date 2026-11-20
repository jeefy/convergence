# Skill scanning

Agent skills run with the agent's trust. `convergence skills scan` vets them with [NVIDIA SkillSpector](https://github.com/NVIDIA/skillspector) (prompt injection, data exfiltration, supply chain and more) in a container, so the host needs no Python. NVIDIA publishes no image, so build the pinned one once:

```bash
convergence skills build      # localhost/convergence-skillspector:v2.13.4 from a pinned commit
convergence skills scan       # ~/.claude/skills, ~/.config/opencode/skills, ~/.agents/skills, <hermes home>/skills
convergence skills scan ./some-skill
```

Each skill gets a score, a severity and a verdict (`SAFE`, `CAUTION`, `DO_NOT_INSTALL`). The command exits 1 when a skill is `DO_NOT_INSTALL`. The static scan runs with `--network=none` and takes seconds.

`--llm` adds SkillSpector's semantic analysis through the gateway (one to two minutes per skill). The model must honor forced tool calls, so `auto` does not work: set `skill_scan_model` in `config.yaml` or pass `--model`. If the model ignores them, the analysis is incomplete and the command says so and exits 1.

To have `sandbox run` check installed skills before starting the agent, set `skill_scan` in `config.yaml`:

| `skill_scan` | Unsafe skill | Scanner image missing |
|---|---|---|
| `off` (default) | no check | no check |
| `warn` | warning, the agent starts | warning, the agent starts |
| `block` | refuses to start | refuses to start |

It reuses the last static scan (`~/.convergence/skill-scan.json`) and rescans only when the installed skills changed. `doctor` reports the cached result.
