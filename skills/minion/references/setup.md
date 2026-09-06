# Minion setup and troubleshooting

## Install or update

1. Detect the operating system and CPU architecture.
2. Select the matching asset from the latest release at `https://github.com/lugenx/minion/releases/latest`.
3. Install it in a directory on `PATH` using the user's normal privilege policy.
4. Verify with `minion --version` and `minion --help`.

Available release asset names follow this pattern:

- `minion-darwin-arm64`
- `minion-darwin-amd64`
- `minion-linux-arm64`
- `minion-linux-amd64`
- `minion-windows-amd64.exe`

Do not build from source unless the user asks or no compatible release exists. Stop a running daemon before replacing its binary, then restore only the schedules the user intended.

## Initialize

Running `minion` opens the TUI and creates `~/.config/minion/` when needed. The user may enter the API key privately in the TUI. Do not read or display the secrets file.

Key paths:

- `~/.config/minion/minions/` stores saved monitors
- `~/.config/minion/data/` stores file pipeline data
- `~/.config/minion/logs/` stores per-monitor logs

## Operate

```bash
minion                  # TUI
minion run key=value    # synchronous one-off run
minion run NAME         # queue a saved monitor
minion up NAME          # schedule
minion up all
minion list --all
minion log NAME
minion stop NAME        # cancel an active run
minion down NAME        # unschedule
minion down              # stop the daemon
```

Read each command's live `--help` before using destructive or state-changing operations. `minion clear NAME` removes remembered state and can cause old items to be evaluated again; use it only when the user intends that reset.

## Troubleshoot

- **No LLM result:** verify a model is configured and the API key exists without exposing either value. Test the same source without `do` to separate retrieval from analysis.
- **No web content:** test the direct URL, then inspect real links before adding `follow`; use `render=true` only for JavaScript content.
- **No repeated alerts:** saved monitors deduplicate unchanged content by design. Inline runs bypass persistent deduplication.
- **No scheduled run:** check `enabled`, `when`, `minion list --all`, daemon state, and the monitor log.
- **No delivery:** inspect the run log and destination independently. A completed fetch is not proof of delivery.
- **File pipeline problems:** parse files as multi-document YAML streams separated by `---`.
- **Update appears ignored:** verify which binary is on `PATH`, confirm its version, and restart the daemon after replacing it.

Preserve existing configs and data during updates. Do not clear state, delete monitors, revoke destinations, or overwrite credential files as a troubleshooting shortcut.