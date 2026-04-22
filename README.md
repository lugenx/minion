# Minion: Autonomous Task Engine

Minion is a lightweight, zero-dependency, autonomous task engine written in Go. 

It reads simple YAML configuration files, scrapes targeted websites (or performs web searches), filters out junk using fast string matching, evaluates the surviving text using an LLM (via OpenRouter), and sends the resulting matches to any generic Webhook (like ntfy, Slack, or Discord).

It acts as your personal AI web-scraping army.

---

## Installation

Minion is a standalone binary. You do not need to install Go or any other dependencies. 
Choose the correct command for your operating system to download and install Minion globally.

### Mac (Apple Silicon / M1 / M2)
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

### Linux (ARM64 / Raspberry Pi)
```bash
curl -sSL https://github.com/lugenx/minion/releases/latest/download/minion-linux-arm64 -o /tmp/minion && chmod +x /tmp/minion && sudo mv /tmp/minion /usr/local/bin/minion
```

### Windows (PowerShell)
```powershell
Invoke-WebRequest -Uri "https://github.com/lugenx/minion/releases/latest/download/minion-windows-amd64.exe" -OutFile "$env:USERPROFILE\minion.exe"
# Ensure your user directory is in your PATH to run 'minion' from anywhere
```

---

## Directory Structure

When you run `minion` for the first time, it will automatically scaffold its required directories and files in your standard OS user folder (`~/.config/minion/` on Mac/Linux).

```text
~/.config/minion/
├── .env              # Store your OpenRouter API key and webhook secrets here
├── minion.db         # A pure-Go SQLite DB that tracks notifications to prevent spam
├── minion.log        # Beautifully formatted logs for the background daemon
└── minions/          # Place your .yaml task configuration files in this folder!
    └── example.yaml  
```

---

## CLI Commands

Minion is designed to be incredibly simple to operate.

*   `minion list` - Prints a beautifully formatted table of all your minions, showing if they are Running, Stopped, or Disabled, and calculates their exact next run time.
*   `minion test <filename>` - Instantly runs a specific minion, skipping its schedule. It outputs a colorful, step-by-step execution log so you can debug your AI instructions and filters.
*   `minion run -d` - Starts the engine as a background daemon. It will silently load all enabled minions and execute them on their precise schedules.
*   `minion stop` - Safely halts the background daemon.
*   `minion clear <filename>` - Wipes the database memory for a specific minion. It will "forget" what it has already notified you about, and will re-send alerts if it sees those events again.

---

## Configuration Guide

Every file in `~/.config/minion/minions/` represents a single task. Here is a breakdown of every feature available.

### The Basics
```yaml
name: "Product Release Tracker"
enabled: true # Set to false to pause the minion without deleting the file
```

### Scheduling
The engine supports a highly flexible, custom scheduling syntax, and automatically falls back to raw Cron for power users.

```yaml
# Groups
schedule: "daily @ 09:00"
schedule: "weekdays @ 18:00"
schedule: "weekends @ 12:00"

# Specific Days (Comma separated)
schedule: "mon, wed, fri @ 17:30"

# Loop Intervals
schedule: "every 30m"
schedule: "every 12h"

# Raw Cron (e.g., every 15 minutes)
schedule: "*/15 * * * *"
```

### Sources (Gathering Data)
You can point Minion directly at URLs, or have it automatically crawl a homepage for sub-links.

```yaml
sources:
  # 1. Simple direct scraping
  - "https://example.com/news"
  
  # 2. Automated Link Following
  # It visits the URL, finds every href containing "/releases/", and scrapes them all!
  - url: "https://example.com/products"
    follow_links: "/releases/"
```

### Web Search (Optional)
Don't have a specific URL? Have Minion search the web for you. It silently queries DuckDuckGo, extracts the top organic links, and scrapes them automatically.

```yaml
web_search:
  queries:
    - "latest open source AI models"
  max_results_per_query: 3
```

### Fast Filtering (Optional)
LLM calls cost money. The Fast Filter does a dumb, lowercase string match on the scraped HTML text. If it finds a match, it drops the page immediately before sending it to the AI.

```yaml
skip_if_contains:
  - "paywall"
  - "subscribe to read"
  - "women only"
```

### AI Instructions
This is the core of the engine. Tell the AI exactly what you are looking for.
*   *Temporal Awareness:* Minion automatically injects your system's current Date and Time into the prompt. You can safely use instructions like "Must happen tomorrow" or "Drop events in the past."
*   *List Parsing:* Minion is designed to extract multiple independent matches from a single page.

```yaml
ai_instructions:
  - "Must be an official software release announcement."
  - "Looking for version 2.0 or higher."
  - "Must be released within the next 7 days."
```

### Generic Webhook Notifications
Minion features a 100% agnostic HTTP Webhook engine. It will POST the AI's summary to any URL. It is perfectly optimized for services like ntfy.sh.

Minion supports Environment Variable Expansion, so you never have to hardcode passwords in your YAML files.

**1. Add your secrets to `~/.config/minion/.env`**
```env
WEBHOOK_USER=admin
WEBHOOK_PASS=my_super_secret_password
```

**2. Configure the YAML**
```yaml
webhook:
  # Example: A private self-hosted notification server
  url: "https://notify.example.com/alerts"
  
  # Inject standard HTTP Basic Authentication safely
  basic_auth:
    username: "${WEBHOOK_USER}"
    password: "${WEBHOOK_PASS}"
    
  # You can also pass custom headers
  headers:
    Title: "Minion Alert!"
    X-Priority: "High"
```

---

## Database Deduplication Architecture
Minion uses a highly robust, pure-Go SQLite database to prevent notification spam.

It does not deduplicate by URL (because a URL like `example.com/daily-news` changes daily). Instead, it deduplicates at the very end of the pipeline. It generates a cryptographic hash of the specific `Title` the AI found. If it has sent you an alert for that specific Title before, it silently drops it. This allows Minion to accurately track rolling date windows and rolling lists!

---

## Legal Disclaimer

**Minion is a generic HTTP automation tool provided for educational and personal productivity purposes.**

Users are solely responsible for ensuring their configurations and usage comply with the Terms of Service and `robots.txt` policies of the websites they interact with. The creator of this tool assumes no liability for misuse, aggressive scraping, or any legal disputes arising from the user's configuration of the engine.