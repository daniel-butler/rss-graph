# Rising Stars & New Additions Spec

## Overview

Add two new discovery modes to help surface interesting feeds and people:
1. **New Additions** — Highlight recently added feeds that haven't built up history yet
2. **Rising Stars** — Surface people/feeds gaining momentum (growing mention velocity)

## Data Model Changes

### New Table: `mention_snapshots`

Track mention counts over time to calculate velocity:

```sql
CREATE TABLE IF NOT EXISTS mention_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    mention_count INTEGER NOT NULL,
    snapshot_date DATE NOT NULL,
    UNIQUE(name, entity_type, snapshot_date)
);

CREATE INDEX IF NOT EXISTS idx_snapshots_date ON mention_snapshots(snapshot_date);
CREATE INDEX IF NOT EXISTS idx_snapshots_name ON mention_snapshots(name);
```

### New Table: `feed_snapshots` (optional, for link velocity)

```sql
CREATE TABLE IF NOT EXISTS feed_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id INTEGER NOT NULL,
    inbound_count INTEGER NOT NULL,
    snapshot_date DATE NOT NULL,
    FOREIGN KEY (feed_id) REFERENCES feeds(id),
    UNIQUE(feed_id, snapshot_date)
);
```

## CLI Changes

### `rss-graph rank`

Add flags:
- `--new` — Boost feeds added in the last 30 days (default), or specify `--new=7` for 7 days
- `--rising` — Sort by inbound link velocity instead of total count

### `rss-graph mentions`

Add flags:
- `--rising` — Sort by mention velocity instead of total count
- `--since=DATE` — Only count mentions since date (for velocity calc)

### `rss-graph snapshot`

New command to capture current state for velocity tracking:

```bash
rss-graph snapshot          # Save today's counts
rss-graph snapshot --list   # Show available snapshots
rss-graph snapshot --prune  # Remove snapshots older than 90 days
```

### `rss-graph discover`

New command combining multiple signals:

```bash
rss-graph discover          # Combined: new + rising + unsubscribed
rss-graph discover --json   # JSON output for scripts
```

## Velocity Calculation

### Rising Stars Formula

```
velocity = (current_count - previous_count) / max(previous_count, 1)
```

Where:
- `current_count` = mentions/links in most recent snapshot
- `previous_count` = mentions/links from 7-30 days ago (configurable)

### Thresholds

- **Rising**: velocity > 0.5 (50% growth)
- **Hot**: velocity > 1.0 (100% growth) AND current_count >= 3
- **New**: no previous snapshot OR previous_count == 0

## Implementation Plan

### Phase 1: Schema + Snapshot Command
1. Add `mention_snapshots` table to schema
2. Implement `snapshot` command
3. Add `Snapshot()` and `GetSnapshots()` methods to graph.go

### Phase 2: Rising Mentions
1. Add `GetRisingMentions()` method
2. Add `--rising` flag to `mentions` command
3. Test with real data

### Phase 3: New Feeds Boost  
1. Add `GetNewFeeds()` method (feeds where created_at > N days ago)
2. Add `--new` flag to `rank` command
3. Optionally blend new feeds into regular ranking

### Phase 4: Discover Command
1. Create combined discovery view
2. JSON output for automation
3. Consider cron job for daily snapshots

## Example Output

```
$ rss-graph mentions --rising

Rising stars (people gaining momentum):

🔥 HOT
 1. [+150%] Addy Osmani (3 → 8 mentions)
 2. [+120%] Yi Tay (5 → 11 mentions)

📈 RISING  
 3. [+75%] Harper Reed (4 → 7 mentions)
 4. [+60%] Philipp Schmid (5 → 8 mentions)

🆕 NEW (first seen this period)
 5. Amelia Wattenberger (4 mentions)
 6. Omar Sanseviero (3 mentions)
```

```
$ rss-graph rank --new

Recently added feeds (last 30 days):

 1. [3 links] Yi Tay's Blog
    https://www.yitay.net/
    Added: 5 days ago
    
 2. [2 links] Amelia Wattenberger
    https://wattenberger.com/
    Added: 12 days ago
```

## Automation

Add to cron/heartbeat:

```bash
# Weekly snapshot (Sundays)
0 0 * * 0 rss-graph snapshot
```

Or integrate with `crawl`:

```bash
rss-graph crawl --snapshot  # Take snapshot after crawl
```

---

*Created: 2026-02-13*
