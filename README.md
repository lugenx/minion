# Minion: AI Web Monitoring Agent

Minion is a lightweight tool for automating web research.

Instead of manually checking websites for updates, you can use the interactive Terminal User Interface (TUI) or create simple YAML scripts to act as your autonomous agents. Minion uses an explicit "Execution Graph" architecture. You command the agent step-by-step to browse websites, study the text to extract specific information, and trigger webhook alerts when it finds a match.

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

```
minion          # Open the dashboard
v               # Add your API key
n               # Create your first minion
space           # Turn it on
l               # Watch it run
```

### 1. Add your API Key
Open the TUI (run `minion`) and press `v` to edit your environment variables, or manually open `~/.config/minion/.env`. Add your OpenRouter API key so your minions can process text. You can also securely store any webhook passwords here.

### 2. Create a Minion
Press `n` in the TUI to build one visually, or write a YAML file in `~/.config/minion/minions/`. Press `e` to edit an existing one.

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

  # 3. Filter links immediately using simple keywords
  - filter: 
      keep: ["startup", "release"] # Must contain at least one of these (if defined)
      drop: ["webinar", "online"]  # Drops the link (evaluated first)

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
        # markdown: true
        # basic_auth:
        #   username: "${MY_USERNAME}"
        #   password: "${MY_PASSWORD}"
        
      # - discord: "https://discord.com/api/webhooks/..."
      
      # - minion: "my_worker_minion_filename"
```

### 3. Run It
Press `space` on a minion to turn it on (the daemon starts automatically), or press `r` to run it once immediately.

### 4. Watch It Work
Press `l` to see live step-by-step logs as the minion runs.

### 5. CLI Commands
Use these commands to manage your tasks from the terminal:

*   **`minion`** - Launches the interactive Terminal User Interface (TUI).
*   **`minion up [filename|all]`** - Schedules a minion to run in the background. Automatically starts the master daemon if needed.
*   **`minion down [filename|all]`** - Unschedules a minion. If no arguments are provided, it cleanly halts the background daemon.
*   **`minion run <filename>`** - Queues a specific minion to execute immediately in the background daemon.
*   **`minion stop <filename>`** - Safely aborts a currently running minion mid-execution.
*   **`minion ls`** - Displays a table of all your minions, their current state (Up/Down/Running), and their next scheduled run time.
*   **`minion log [filename]`** - Follows the live output logs of a specific minion, or the master daemon if no arguments are provided.
*   **`minion clear <filename>`** - Wipes the database memory for a specific minion so it will re-evaluate items it has already seen. (e.g. `minion clear price_tracker` or `minion clear --all`)

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
LLM calls cost money. The fast filter does a strict string match (case-insensitive) on the URL, title, and preview text. You can use this to drop bad URLs before you scrape them, or on the raw HTML text after you scrape them.

If you provide multiple words, the filter uses **ANY** logic—it triggers if the text contains at least one of the words as a substring. 
If both `keep` and `drop` arrays are provided, the filter evaluates `drop` first. If a `drop` word is found, the item is immediately discarded, even if it also contains a `keep` word.

```yaml
- filter: 
    keep: ["artificial intelligence", "machine learning"]
    drop: ["paywall", "subscribe to read"]
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

By default, the engine outputs structured alerts. *(Planned: `format: "plain_text"` will output raw text paragraphs instead.)*
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
      markdown: true
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

## Smart Notifications

Minion won't spam you.

- **Skips unchanged pages.** It remembers the content of every page it scrapes. If nothing meaningful changed since the last check, it moves on without calling the AI. You don't get notified about "last updated 5 mins ago" noise.
- **Blocks off-topic pages permanently.** If the AI decides a page is not relevant to your task, it marks it forever. That URL will never be scraped again.
- **You only see what matters.** The result: a notification when something actually changes or a genuine match is found. Nothing else.

---

## Legal Disclaimer

**Minion is provided for educational and personal productivity purposes.**

Users are solely responsible for ensuring their configurations and usage comply with the Terms of Service and `robots.txt` policies of the websites they interact with. The creator of this tool assumes no liability for misuse, aggressive scraping, or any legal disputes arising from the user's configuration of the engine.
