# Minion: AI Web Monitoring Agent

Minion is a lightweight tool for automating web research.

Instead of manually checking websites for updates, you can use the interactive Terminal User Interface (TUI) or create simple YAML files to act as your autonomous agents. Minion uses an explicit step-by-step pipeline. You configure each stage: gather URLs, filter, scrape, analyze with AI, and deliver alerts.

---

## Installation

Choose the correct command for your operating system.

### Mac (Apple Silicon)
```bash
curl -sSL https://github.com/lugenx/minion/releases/latest/download/minion-darwin-arm64 -o /tmp/minion && chmod +x /tmp/minion && sudo mv /tmp/minion /usr/local/bin/minion
```

### Mac (Intel)
```bash
curl -sSL https://github.com/lugenx/minion/releases/latest/download/minion-darwin-amd64 -o /tmp/minion && chmod +x /tmp/minion && sudo mv /tmp/minion /usr/local/bin/minion
```

### Linux (AMD64)
```bash
curl -sSL https://github.com/lugenx/minion/releases/latest/download/minion-linux-amd64 -o /tmp/minion && chmod +x /tmp/minion && sudo mv /tmp/minion /usr/local/bin/minion
```

### Linux (ARM64)
```bash
curl -sSL https://github.com/lugenx/minion/releases/latest/download/minion-linux-arm64 -o /tmp/minion && chmod +x /tmp/minion && sudo mv /tmp/minion /usr/local/bin/minion
```

### Windows (PowerShell)
```powershell
Invoke-WebRequest -Uri "https://github.com/lugenx/minion/releases/latest/download/minion-windows-amd64.exe" -OutFile "$env:USERPROFILE\minion.exe"
```

---

## Getting Started

When you run `minion` for the first time, it creates a `~/.config/minion/` folder. This is where your minions live.

```
minion          # Open the dashboard
v               # Add your API key
n               # Create your first minion
space           # Turn it on
l               # Watch it run
```

### 1. Add your API Key
Open the TUI (`minion`) and press `v` to edit your environment variables, or manually edit `~/.config/minion/.env`. Add your OpenRouter API key so your minions can process text. You can also store any webhook passwords here.

### 2. Create a Minion
Press `n` in the TUI to build one visually, or write a YAML file in `~/.config/minion/minions/`. Press `e` to edit an existing one.

Minion operates on a linear pipeline. It gathers URLs, filters them, scrapes the pages, runs an AI analysis, and delivers matches. Here is a complete example:

```yaml
name: Product Release Tracker
enabled: true
when: daily @ 09:00

from:
  # 1. Search the web
  - search: latest open source AI models
    limit: 3

  # 2. Direct URL
  - url: https://example.com/news

  # 3. Crawl for sub-links matching a pattern
  - url: https://example.com/releases
    follow: /releases/

  # 4. Rendered scrape (headless browser for SPAs)
  - url: https://example.com/products
    follow: /p/
    render: true

  # 5. Run a shell command
  - command: curl -s https://api.example.com/status

keep:
  - startup
  - release
  - v2.0

ignore:
  - webinar
  - online only
  - sponsored

do: Find official release announcements for version 2.0 or higher.

tell:
  ntfy: https://ntfy.sh/mytopic
  markdown: true
  basic_auth:
    username: "${NTFY_USER}"
    password: "${NTFY_PASS}"

report:
  ntfy: https://ntfy.sh/mytopic

settings:
  timeout: 15
  delay: 2
```

### 3. Run It
Press `space` on a minion to schedule it (the daemon starts automatically), or press `r` to run it once immediately.

### 4. Watch It Work
Press `l` to see live step-by-step logs as the minion runs.

### 5. CLI Commands

*   **`minion`** - Launches the TUI.
*   **`minion up [filename|all]`** - Schedules a minion. Starts the daemon if needed.
*   **`minion down [filename|all]`** - Unschedules a minion. With no args, stops the daemon.
*   **`minion run <filename>`** - Queues a minion for immediate execution.
*   **`minion stop <filename>`** - Aborts a running minion.
*   **`minion ls`** - Lists all minions with their state and schedule.
*   **`minion log [filename]`** - Follows live logs.
*   **`minion clear <filename>`** - Wipes a minion's memory so it re-evaluates seen items.

---

## Pipeline Reference

Minion has 8 pipeline steps. You can skip any step, and the builder in the TUI lets you reorder them.

### when (Schedule)
```yaml
when: "daily @ 09:00"
```
- Groups: `daily @ 09:00`, `weekdays @ 18:00`, `weekends @ 12:00`
- Specific: `mon, wed, fri @ 17:30`
- Interval: `every 30m`, `every 12h`
- Raw cron: `*/15 * * * *`

### from (Data Sources)
You can specify URLs, search queries, shell commands, or hand off data from another minion.

Add `render: true` for JavaScript-heavy pages. This boots a headless Chromium browser to execute JS and wait for network requests before extracting links.

```yaml
from:
  - url: https://example.com/news
  - url: https://example.com/products
    follow: /releases/
  - url: https://example.com/spa
    render: true
  - search: latest open source AI models
    limit: 3
  - minion: other_minion_filename
  - command: curl -s https://api.example.com/status
```

### keep / ignore (Content Filtering)
LLM calls cost money. These do a case-insensitive substring match on the full scraped page content, title, summary, and URL. Use them to drop irrelevant pages before the LLM call.

**ANY** logic: if multiple words are listed, a match on any one triggers the rule. `ignore` is evaluated first — if an ignore word matches, the item is discarded even if it also matches a keep word.

```yaml
keep:
  - artificial intelligence
  - machine learning

ignore:
  - paywall
  - subscribe to read
```

### do (AI Study Task)
The core of the engine. The minion reads the scraped text and extracts matches based on your plain-English task. Minions are aware of the current date and time — you can use natural instructions like *"Must happen tomorrow"*.

```yaml
do: Find mentions of product launches. Ignore rumors.
```

### tell (Delivery)
Minion POSTs summaries to any URL. It supports env var expansion so you never hardcode passwords.

```yaml
tell:
  ntfy: https://ntfy.sh/mytopic
  markdown: true
  basic_auth:
    username: "${NTFY_USER}"
    password: "${NTFY_PASS}"

  # discord: https://discord.com/api/webhooks/...

  # http_request: https://notify.example.com/alerts
  # method: POST
  # headers:
  #   X-Priority: High
  # payload_template: |
  #   {"title": "{{.Title}}", "body": "{{.Summary}}"}
```

### report (Mission Report)
Sends a summary of the run (duration, items found, cost) using the same routing as `tell`. Only fires once at the end.

```yaml
report:
  ntfy: https://ntfy.sh/my-logs
```

**Example output:**
```
Mission Report: Tech Event Tracker

Start:  2024-10-25 14:30:00
End:    14:30:04
Time:   4.2s
Errors: 0

- Search Results: 5 links
- Scraped:        5 pages
- Studied:        1 pages
- Found:          2 items
- Delivered:      2 items

Cost:   $0.0012
```

### settings
Controls scraping behavior globally.

```yaml
settings:
  timeout: 15   # Max seconds per page
  delay: 2      # Random delay 1-2s between requests
```

---

## Passing Data Between Minions

Minions can hand data to other minions. This is useful for splitting work — one minion gathers links, another studies them for a different topic.

**Sending minion** uses the target's filename:
```yaml
tell:
  minion: event_reader
```

**Receiving minion** uses `from` with the `minion` source:
```yaml
name: Event Reader

from:
  - minion: My Link Gatherer

do: Find event dates and locations.
```

---

## Smart Notifications

- **Skips unchanged pages.** Minion remembers page content. If nothing changed, it skips the AI call.
- **Blocks off-topic pages.** Irrelevant URLs are marked forever and never scraped again.
- **You only see what matters.** A notification fires when something actually matches.

---

## Legal Disclaimer

**Minion is provided for educational and personal productivity purposes.**

Users are solely responsible for ensuring their configurations comply with the Terms of Service and `robots.txt` policies of the websites they interact with. The creator assumes no liability for misuse or aggressive scraping.
