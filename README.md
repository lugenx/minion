# Minion: AI Web Monitoring Agent

Minion is a lightweight tool for automating web research.

Instead of manually checking websites for updates, you create simple YAML files (called "minions") to act as your autonomous agents. A minion will browse your target websites, use AI to extract the specific information you asked for, and trigger a webhook alert to let you know when it finds a match.

---

## Installation

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
```

---

## Getting Started

When you run `minion` for the first time, it automatically creates a `~/.config/minion/` folder. This is where your minions live.

### 1. Add your API Keys
Open `~/.config/minion/.env` and add your OpenRouter API key so your minions can process text. You can also securely store any Webhook passwords here.

### 2. Create a Minion
Every file you put in `~/.config/minion/minions/` is a new minion agent. Here is a basic example of a task configuration:

```yaml
name: "Product Release Tracker"
enabled: true 
schedule: "daily @ 09:00"

# Where should the minion look?
sources:
  - "https://example.com/news"

# If the page contains these exact words, the minion skips it.
skip_if_contains:
  - "paywall"
  - "subscribe to read"

# Tell the AI exactly what you want it to extract.
task: |
  Looking for official software release announcements for version 2.0 or higher.
  Must be released within the next 7 days.

# Where to send the alert when a match is found
webhook:
  url: "https://ntfy.sh/mytopic"
  basic_auth:
    username: "${WEBHOOK_USER}"
    password: "${WEBHOOK_PASS}"
```

### 3. CLI Commands
Use these commands to manage your tasks:

*   **`minion test <filename>`** - Instantly runs a specific minion, ignoring its schedule. Outputs a step-by-step execution log.
*   **`minion run -d`** - Starts the engine silently in the background. It will run your active minions on their designated schedules.
*   **`minion list`** - Displays a table of all your minions, their current state (Running/Stopped/Disabled), and their next scheduled run time.
*   **`minion clear <filename>`** - Wipes the database memory for a specific minion so it will re-evaluate items it has already seen.
*   **`minion stop`** - Halts the background daemon.

---

## Features

### Smart Spam Protection
Minion remembers what it has already alerted you about. If an event or news article remains on a homepage for several days, you only receive one notification. 

### Automated Link Clicking
If a search page only shows titles without full descriptions, you can instruct your minion to automatically click and read specific sub-links.
```yaml
sources:
  - url: "https://example.com/products"
    follow_links: "/releases/" # Evaluates every link that contains this string
```

### Web Searching
Minion can query DuckDuckGo, extract the top organic links, and evaluate them automatically.
```yaml
web_search:
  queries:
    - "latest open source AI models"
  max_results_per_query: 3
```

### Time Awareness
Minions are inherently aware of the current date and time. You can safely use natural instructions like *"Must happen tomorrow"* or *"Drop events in the past"* in your tasks.

---

## Legal Disclaimer
**Minion is provided for educational and personal productivity purposes.**
Users are solely responsible for ensuring their configurations comply with the Terms of Service and `robots.txt` policies of the websites they interact with. The creator of this tool assumes no liability for misuse or aggressive scraping.