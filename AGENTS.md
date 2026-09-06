# Minion agent entry point

For any request to install, configure, run, automate, update, or troubleshoot Minion, read and follow [`skills/minion/SKILL.md`](skills/minion/SKILL.md) before acting.

If your environment supports persistent skills, install the `skills/minion` directory using its normal skill mechanism. Do this yourself when permitted; do not make the user translate repository instructions into agent configuration. If persistent skills are unavailable, follow the repository skill directly.

Minion is a standalone application, not an extension of any particular agent. Use the current repository code and live `minion ... --help` output as the source of truth.