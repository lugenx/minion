# Minion: AI Web Monitoring Agent

Minion is a lightweight tool for automating web research.

Instead of manually checking websites for updates, you create simple YAML scripts to act as your autonomous agents. Minion uses an explicit "Execution Graph" architecture. You command the agent step-by-step to browse websites, study the text to extract specific information, and trigger webhook alerts when it finds a match.

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

### 2. Create a Mission
Every file you put in `~/.config/minion/minions/` is a new minion agent. 

Minion operates on a linear stream. It gathers all URLs from your search and browse blocks, and then passes them one-by-one through the rest of the pipeline. Here is a complete example:

```yaml
name: "Product Release Tracker"
enabled: true 

mission:
  # 1. The Trigger
  - schedule: "daily @ 09:00"

  # 2. Gather URLs
  - search: 
      - "latest open source AI models"
      - "AI startup news"
    limit: 3

  - browse:
      # Just grab this exact URL
      - url: "https://example.com/news"
      # Browse the page and return sub-links matching the Regex pattern
      - url: "https://example.com/events"
        match: "/events/"

  # 3. Filter out bad links immediately
  - filter: 
      drop_if_contains: ["webinar", "online"]

  # 4. Download the webpage HTML
  - scrape: true

  # 5. Have the minion study the pages
  - study: true
    task: |
      Looking for official software release announcements for version 2.0 or higher.
      Must be released within the next 7 days.

  # 6. Deliver the results
  - deliver:
      - ntfy: "https://ntfy.sh/mytopic"
        # basic_auth:
        #   username: "${MY_USERNAME}"
        #   password: "${MY_PASSWORD}"
        
      # - discord: "https://discord.com/api/webhooks/..."
      
      # - minion: "my_worker_minion_filename"
```

### 3. CLI Commands
Use these commands to manage your tasks:

*   **`minion test <filename>`** - Instantly runs a specific minion, ignoring its schedule. Outputs a step-by-step execution log.
*   **`minion run -d`** - Starts the engine silently in the background. It will run your active minions on their designated schedules.
*   **`minion list`** - Displays a table of all your minions, their current state (Running/Stopped/Disabled), and their next scheduled run time.
*   **`minion clear <filename>`** - Wipes the database memory for a specific minion so it will re-evaluate items it has already seen.
*   **`minion stop`** - Halts the background daemon.

---

## Pipeline Reference Guide

Minion allows you to build modular pipelines. You can skip steps, duplicate steps, or change the order.

### The Trigger
```yaml
# Groups: "daily @ 09:00", "weekdays @ 18:00", "weekends @ 12:00"
# Specific: "mon, wed, fri @ 17:30"
# Interval: "every 30m", "every 12h"
# Raw Cron: "*/15 * * * *"
- schedule: "daily @ 09:00"
```

### Data Generators
You can hardcode specific URLs, instruct the minion to browse homepages for sub-links, or search the web dynamically.
```yaml
- search: 
    - "latest open source AI models"
  limit: 3

- browse:
    - url: "https://example.com/news"
    - url: "https://example.com/products"
      match: "/releases/"
```

### Fast Filtering (Optional)
LLM calls cost money. The fast filter does a strict string match. You can use this to drop bad URLs before you scrape them, or on the raw HTML text after you scrape them.
```yaml
- filter: 
    drop_if_contains: ["paywall", "subscribe to read"]
```

### Scraping (Optional)
Downloads the raw HTML of the gathered URLs and strips away formatting, scripts, and styling to leave only readable text.
```yaml
- scrape: true
```

### Study
The core of the engine. The minion reads the data and extracts matches based on your plain-English task. 
Minions are inherently aware of the current date and time. You can safely use natural instructions like *"Must happen tomorrow"* or *"Drop events in the past"* in your tasks.

By default, the engine outputs structured alerts. You can also use `format: "plain_text"` to output raw text paragraphs (like essays or poems).
```yaml
- study: true
  task: "Find mentions of Apple Inc. Ignore hardware releases."
```

### Delivery
Minion uses an agnostic HTTP engine. It will POST the summary to any URL. 
It natively supports Environment Variable Expansion so you never have to hardcode passwords in your YAML files.
```yaml
- deliver:
    - ntfy: "https://ntfy.sh/mytopic"
    - discord: "https://discord.com/api/webhooks/123"
    
    # Advanced Power User HTTP Requests
    - http_request: "https://notify.example.com/alerts"
      method: "POST"
      headers:
        X-Priority: "High"
      payload_template: |
        {"custom_title": "{{.Title}}", "desc": "{{.Summary}}"}
      basic_auth:
        username: "${WEBHOOK_USER}"
        password: "${WEBHOOK_PASS}"
```

### Handing Data to Other Minions
Minions can deliver data directly to other minions. This is useful if you want to split up your work—for example, one minion can gather 50 links from the web, and hand those links off to two different minions that study the text for completely different things.

**The Sending Minion:**
Use the exact filename of the minion you want to send data to.
```yaml
- deliver:
    - minion: "event_reader"
```

**The Receiving Minion:**
The receiving minion doesn't need a schedule or a search block. It uses `receive` to verify exactly which minion is allowed to hand it data.
```yaml
name: "Event Reader"

mission:
  - receive: "My Link Gatherer"
  
  - scrape: true
  - study: true
    task: "Look for events on these pages."
```

---

## Smart Caching Architecture

Minion uses a highly robust, two-tier SQLite database to prevent notification spam and save you money on AI API calls.

1. **The AI Firewall (`dropped_urls`):** When the minion studies a page, it decides if the page is fundamentally irrelevant (e.g., a "Cooking Class" when you asked for "Tech Events"). If it is, the minion permanently drops it. The engine logs that URL to a firewall database and will **never** scrape or evaluate it again, saving massive amounts of time on future runs.
2. **Notification Deduplication (`sent_notifications`):** It generates a cryptographic hash of the specific `Title` the minion found. If it has sent you an alert for that specific Title before, it silently drops it. This allows Minion to accurately track rolling date windows and rolling lists without spamming you!

---

## Legal Disclaimer

**Minion is provided for educational and personal productivity purposes.**

Users are solely responsible for ensuring their configurations and usage comply with the Terms of Service and `robots.txt` policies of the websites they interact with. The creator of this tool assumes no liability for misuse, aggressive scraping, or any legal disputes arising from the user's configuration of the engine.
