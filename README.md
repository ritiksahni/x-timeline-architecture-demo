# The Twitter Timeline Problem

**Elon tweets. 100 million followers need to see it. You have 5 seconds. What do you do?**

This project implements and benchmarks the three strategies Twitter considered for this exact problem — straight from Chapter 1 of [Designing Data-Intensive Applications](https://dataintensive.net/):

| Strategy | Write Cost | Read Cost | The Trade-off |
|----------|------------|-----------|---------------|
| **Fan-Out on Write** | O(followers) | O(1) | Fast reads, but celebrities break it |
| **Fan-Out on Read** | O(1) | O(following) | Fast writes, but timeline loads crawl |
| **Hybrid** | Varies | Varies | What Twitter actually uses |

Built by [Ritik Sahni](https://github.com/ritiksahni) as an interactive deep dive into distributed systems trade-offs.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Clients                                    │
│                    ┌─────────┐  ┌─────────┐                         │
│                    │   CLI   │  │ Web UI  │                         │
│                    └────┬────┘  └────┬────┘                         │
└─────────────────────────┼───────────┼───────────────────────────────┘
                          │           │
                          ▼           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Go API Server                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │ Fan-Out Write│  │ Fan-Out Read │  │    Hybrid    │               │
│  │   Strategy   │  │   Strategy   │  │   Strategy   │               │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘               │
│         │                 │                 │                        │
│         └─────────────────┼─────────────────┘                        │
│                           │                                          │
│              ┌────────────┴────────────┐                             │
│              ▼                         ▼                             │
│     ┌─────────────────┐       ┌─────────────────┐                   │
│     │   PostgreSQL    │       │     Redis       │                   │
│     │   (Persistent)  │       │    (Cache)      │                   │
│     └─────────────────┘       └─────────────────┘                   │
└─────────────────────────────────────────────────────────────────────┘
```

## How Each Strategy Works

### Fan-Out on Write (Push)

Post a tweet → Push to every follower's cache immediately.

```
Tweet posted → Find 10,000 followers → Update 10,000 caches
```

**Result:** Reading your timeline is instant. It's already waiting for you. But if Elon tweets, that's 100M cache writes.

### Fan-Out on Read (Pull)

Post a tweet → Store it once. Build timelines on-demand.

```
Open timeline → Query 500 accounts you follow → Merge and sort
```

**Result:** Posting is instant. But every timeline refresh queries hundreds of users.

### Hybrid (What Twitter Uses)

Push for regular users. Pull for celebrities. Best of both.

```
Regular tweet → Push to followers
Celebrity tweet → Fetch and merge when timeline loads
```

**Result:** 99% of users get instant timelines. Celebrities don't melt the servers.

## Run It Yourself

**Prerequisites:** Go 1.21+, Docker, Node.js 18+, pnpm

```bash
# 1. Start PostgreSQL and Redis
make docker-up

# 2. Start the API server
make run

# 3. Seed test data (10k users, realistic follow graph)
make build-cli
./bin/fanout seed --users 10000 --avg-followers 150 --celebrities 50

# 4. Open the interactive dashboard
cd web && pnpm install && pnpm dev
```

Then open **http://localhost:3000** and start experimenting.

### Run Benchmarks

```bash
# Compare all strategies head-to-head
./bin/fanout benchmark --strategy all --tweets 1000 --concurrent 50

# Test just the hybrid approach
./bin/fanout benchmark --strategy hybrid --tweets 500

# Export results
./bin/fanout benchmark --output results.json
```

## CLI Commands

```bash
# Configuration
fanout config show                              # Show all config
fanout config get celebrity-threshold           # Get specific value
fanout config set celebrity-threshold 10000     # Set value

# Data seeding
fanout seed --users 10000 --avg-followers 150 --celebrities 50 --tweets-per-user 10

# Benchmarking
fanout benchmark --strategy all --tweets 1000 --concurrent 50
fanout benchmark --strategy hybrid --duration 60s

# View results
fanout results --format table
fanout results --format json --input results.json
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/tweet` | Post a new tweet |
| GET | `/api/timeline/{user_id}` | Get user's timeline |
| GET | `/api/config` | Get configuration |
| PUT | `/api/config` | Update configuration |
| GET | `/api/metrics` | Get metrics summary |
| GET | `/api/metrics/recent` | Get recent metrics |
| DELETE | `/api/metrics` | Clear metrics |
| GET | `/health` | Health check |

### Example: Post a Tweet

```bash
curl -X POST http://localhost:8080/api/tweet \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "content": "Hello, world!",
    "strategy": "hybrid"
  }'
```

### Example: Get Timeline

```bash
curl "http://localhost:8080/api/timeline/1?strategy=hybrid&limit=50"
```

## Project Structure

```
twitter-fan-out/
├── cmd/
│   ├── server/main.go          # API server entrypoint
│   └── cli/                    # CLI tool
│       ├── main.go
│       ├── config.go
│       ├── seed.go
│       ├── benchmark.go
│       └── results.go
├── internal/
│   ├── config/                 # Configuration management
│   ├── models/                 # Data models
│   ├── repository/             # PostgreSQL operations
│   ├── cache/                  # Redis operations
│   ├── timeline/               # Timeline strategies
│   │   ├── common.go
│   │   ├── fanout_write.go
│   │   ├── fanout_read.go
│   │   └── hybrid.go
│   ├── api/                    # HTTP handlers
│   └── benchmark/              # Benchmark engine
├── web/                        # Svelte dashboard
├── migrations/                 # SQL migrations
├── docker-compose.yml
├── Makefile
└── README.md
```

## Key Metrics

The system tracks and reports:

- **Write Latency** - Time to post a tweet (P50, P95, P99)
- **Read Latency** - Time to fetch timeline (P50, P95, P99)
- **Throughput** - Operations per second
- **Fan-Out Count** - Number of cache updates per write
- **Cache Hit Rate** - Percentage of reads served from cache

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `celebrity_threshold` | 10000 | Follower count above which user is a celebrity |
| `timeline_cache_size` | 800 | Max tweets in timeline cache |
| `timeline_page_size` | 50 | Default tweets per page |

## What You'll See

| Strategy | Write P95 | Read P95 | Why |
|----------|-----------|----------|-----|
| Fan-Out Write | 10-50ms | 1-5ms | Writes touch many caches; reads are instant |
| Fan-Out Read | 1-5ms | 20-100ms | Writes are cheap; reads query many users |
| Hybrid | 5-20ms | 5-20ms | The sweet spot for real social graphs |

P95 = 95th percentile latency. This shows tail performance, not just averages.

*Results vary with hardware, data size, and network. Run it yourself to see.*

## Learning Resources

- [Designing Data-Intensive Applications](https://dataintensive.net/) by Martin Kleppmann
- Chapter 1, Section "Twitter's Timeline" discusses this exact problem

## Tech Stack

- **Backend:** Go 1.21+ with chi router
- **Database:** PostgreSQL 15
- **Cache:** Redis 7
- **Web UI:** Svelte 4 + TailwindCSS
- **Containerization:** Docker Compose

## Use This Project

MIT License. Use it for learning, system design prep, or as a foundation for your own work.

**Questions?** Open an issue, email [ritik@ritiksahni.com](mailto:ritik@ritiksahni.com), or find me on [X](https://x.com/ritiksahni22).
