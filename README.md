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
      # Use a headless browser to execute JavaScript for Single Page Applications (SPAs)
      - url: "https://example.com/products"
        match: "/p/"
        render: true

  # 3. Filter out bad links immediately
  - filter: 
      drop_if_contains: ["webinar", "online"]

  # 4. Download the webpage HTML
  - scrape:
      timeout: 15
      delay: 2

  # 5. Have the minion study the pages
  - study:
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
*   **`minion log <filename>`** - Follows the live step-by-step execution log for a specific minion while it runs in the background. (Leave filename blank to see master daemon log).
*   **`minion list`** - Displays a table of all your minions, their current state (Scheduled/Running/Stopped), and their next scheduled run time. (Use `-a` to show disabled minions).
*   **`minion clear <filename>`** - Wipes the database memory for a specific minion so it will re-evaluate items it has already seen. (e.g. `minion clear 23` or `minion clear --all`)
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

For heavily JavaScript-rendered websites (SPAs), add `render: true` to the browse block. This will boot an isolated Chromium headless browser to execute the JavaScript and wait for background network requests to finish before extracting links.
```yaml
- search: 
    - "latest open source AI models"
  limit: 3

- browse:
    - url: "https://example.com/news"
    - url: "https://example.com/products"
      match: "/releases/"
    - url: "https://example.com/products"
      match: "/p/"
      render: true
```

### Fast Filtering (Optional)
LLM calls cost money. The fast filter does a strict string match. You can use this to drop bad URLs before you scrape them, or on the raw HTML text after you scrape them.
```yaml
- filter: 
    drop_if_contains: ["paywall", "subscribe to read"]
```

### Scraping (Optional)
Downloads the raw HTML of the gathered URLs and strips away formatting, scripts, and styling to leave only readable text. You can optionally set a timeout and a maximum delay between requests to avoid bot detection.

If you used `render: true` in the browse step, the scraper will automatically use a headless browser to fetch the final pages as well. The `timeout` value is applied globally.
```yaml
- scrape:
    timeout: 15
    delay: 2 # Minion will pause randomly for 1 to 2 seconds before scraping
```

### Study
The core of the engine. The minion reads the data and extracts matches based on your plain-English task. 
Minions are inherently aware of the current date and time. You can safely use natural instructions like *"Must happen tomorrow"* or *"Drop events in the past"* in your tasks.

By default, the engine outputs structured alerts. You can also use `format: "plain_text"` to output raw text paragraphs (like essays or poems).
```yaml
- study:
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

### Mission Reports

If you have the daemon running silently in the background, you might want to know exactly what it did when it finishes its run. 

You can use the `report` block to get a scorecard containing the total execution time, LLM cost savings, and the number of items found.

```yaml
- report:
    # Send the mission report to your phone via Ntfy or Discord
    - ntfy: "https://ntfy.sh/my-logs"
```

The report block uses the exact same routing syntax as `deliver`, but it only executes once at the very end of the pipeline.

**Example Report Output:**
```text
Mission Report: Tech Event Tracker

Start:  2024-10-25 14:30:00
End:    14:30:04
Time:   4.2s
Errors: 0

- Search Results: 5 links
- Browse Results: 0 links
- Scraped:        5 pages
    - Cached:     4 pages
- Studied:        1 pages
    - Discarded:  0 pages
    - Skipped:    0 pages
    - Found:      2 items
- Delivered:      2 items

Cost:   $0.0012
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
  
  - scrape:
  - study:
      task: "Look for events on these pages."
```

---

## Smart Caching Architecture

Minion uses a highly robust, deterministic SQLite database to prevent notification spam and save you money on AI API calls.

1. **Page-Level Content Hashing:** When Minion scrapes a webpage, it extracts the visible text, aggressively masks out web noise (like "Updated 5 mins ago" or "100 Views"), and generates a deterministic math hash (SHA-256) of the core content. If the page hasn't meaningfully changed since the last run, the engine drops it immediately. **It does not call the LLM.** This saves you massive amounts of time and money on unchanged websites while safely capturing critical changes like price drops or updated event times.
2. **The AI Firewall (`dropped_urls`):** When the AI *does* study a page, it decides if the page is fundamentally off-topic. If it is, the AI flags it as a `permanent_drop`. The engine logs that URL to a firewall database and will never scrape it again.

---

## Legal Disclaimer

**Minion is provided for educational and personal productivity purposes.**

Users are solely responsible for ensuring their configurations and usage comply with the Terms of Service and `robots.txt` policies of the websites they interact with. The creator of this tool assumes no liability for misuse, aggressive scraping, or any legal disputes arising from the user's configuration of the engine.
