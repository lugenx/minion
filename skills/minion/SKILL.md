---
name: minion
description: Use when installing, configuring, operating, or troubleshooting Minion web monitors and one-off research runs.
version: 1.0.0
author: lugenx
---

# Minion

Install and operate Minion, a standalone CLI and TUI for one-off web research and scheduled YAML monitors. Handle machine actions directly when permitted. Ask the user only for decisions, credentials, or access you cannot obtain safely.

## Source of truth

Use the current repository code and live `minion ... --help` output. Do not rely on old issues, commits, remembered flags, or a previously installed version when they disagree.

Repository: `https://github.com/lugenx/minion`

For installation, updates, daemon operation, or troubleshooting, read [`references/setup.md`](references/setup.md).

## Choose the right mode

- **One-off task:** use an inline run. It is synchronous, skips persistent deduplication, and emits a YAML document stream on stdout.
- **Recurring monitor:** create a saved YAML file, test it, then schedule it.
- **Interactive editing:** use the TUI when a human wants to inspect or edit monitors visually.

Prefer an inline run first when validating a source or prompt. Do not create a persistent monitor for a one-time request.

## Inline runs

```bash
minion run from.url="https://example.com"
minion run from.search="release announcements" from.limit=5 do="Find official releases on this page."
```

Without `do`, Minion returns sanitized source content with no LLM call. With `do`, it returns final `title`, `url`, and `summary` fields. Multiple results are separated by `---` and must be parsed as a YAML stream, not as one mapping.

Source-local options apply to the source immediately before them:

```bash
minion run from.url="https://example.com/releases" from.follow="/release/" from.render=true
```

Use live `minion run --help` and the repository README for the supported key list.

## Saved monitors

Saved configs live in `~/.config/minion/minions/*.yaml`. Build the smallest pipeline that meets the request:

```yaml
name: Release Monitor
enabled: true
when: daily @ 09:00
from:
  - url: https://example.com/releases
    follow: /release/
keep:
  - release
do: Find official product releases on this page.
tell:
  - file: ~/.config/minion/data/releases.yaml
    capacity: 100
settings:
  timeout: 15
```

Use `follow`, not a guessed URL pattern. Inspect real links before setting it. Test source retrieval before enabling delivery or scheduling.

```bash
minion run monitor_name
minion up monitor_name
minion list
minion log monitor_name
minion down monitor_name
```

`minion run monitor_name` queues the saved monitor through the daemon. Inline runs and saved monitors have different state semantics; do not use one as proof of the other's deduplication behavior.

## Credentials

Never print, paste into chat, or expose `~/.config/minion/.env`. Let the user enter secrets privately through the TUI or their local environment. If credentials already exist, use Minion without reading them. Omit `do` when no LLM is needed.

## Verification

After any setup or change, verify the exact path used:

1. `minion --version`
2. Run a harmless source test.
3. Parse stdout as YAML when using inline mode.
4. For saved monitors, confirm `minion list` state and inspect `minion log NAME`.
5. Confirm the destination received the expected record; command success alone is not delivery proof.

Do not report installation, scheduling, or delivery as successful without observable output from the corresponding command or destination.