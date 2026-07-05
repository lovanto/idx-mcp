# idx-mcp

[![CI](https://github.com/lovanto/idx-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/lovanto/idx-mcp/actions/workflows/ci.yml)

An [MCP](https://modelcontextprotocol.io) server that exposes **official Bursa Efek Indonesia
(idx.co.id)** market data to AI assistants — daily prices with official foreign flow, company
profiles, and financial-statement fundamentals parsed straight from IDX's XBRL filings.

Unlike yfinance-based alternatives, `idx-mcp` reads IDX's own endpoints, so it returns the
fundamentals and official foreign buy/sell figures that yfinance leaves empty for Indonesian
tickers. It ships as a single static Go binary (no cgo, no headless browser) with an embedded
SQLite cache.

> **Data usage / legal.** This is a tool, not a data set. It ships no IDX data; each user fetches
> for their own personal use. IDX's Terms of Use prohibit commercial use and redistribution of the
> data — respect them. The server rate-limits itself conservatively by default.

## Tools

| Tool | Arguments | Returns |
|---|---|---|
| `get_trading_info` | `code` (e.g. `BBCA`), `length` (1–365, default 30) | Daily OHLCV + official foreign buy/sell/net, most recent first |
| `get_company_profile` | `code` | Listing/sector metadata + recent dividends |
| `get_dividends` | `code` | Most recently declared dividend(s): cash per share, cum/ex/record/payment dates, bonus-share ratio |
| `get_shareholders` | `code` | Ownership structure: each holder's name, category, share count, percentage, and controlling-shareholder flag (largest first) |
| `get_subsidiaries` | `code` | Consolidated subsidiaries: name, line of business, location, ownership %, total assets, operation status (largest first) |
| `get_management` | `code` | Board: directors (with affiliation flag) and commissioners (with independence flag), plus positions |
| `get_market_index` | `code` (optional, e.g. `COMPOSITE`, `LQ45`) | Latest EOD index summary: OHLC-style values, change, change %, volume, value, market cap. Omit `code` for all ~45 indices |
| `get_market_summary` | _(none)_ | Exchange-wide roll-up for the latest trading day: market breadth (advancing/declining/unchanged), total volume/value/frequency, net foreign flow, and top gainers/losers |
| `get_sector_summary` | _(none)_ | Per-sector roll-up of the latest trading day: for each of the ~11 IDX sectors, stocks traded, market breadth (advancing/declining/unchanged), total value, and net foreign flow; sorted by value |
| `get_broker_summary` | _(none)_ | Per-broker trading activity for the latest day (all ~88 brokers): firm code, name, volume, value, frequency; sorted by traded value |
| `list_companies` | `query` (optional, matches code/name), `sector` (optional) | IDX listed-company directory (~957) for ticker discovery: code, name, listing board/date, sector, industry. Capped at 100 results (`truncated` flag when more matched) |
| `get_financial_report` | `code`, `year` (e.g. `2026`), `period` (`tw1`/`tw2`/`tw3`/`audit`, default `tw1`) | Key accounts (assets, liabilities, equity, revenue, profit, …) parsed from the official XBRL filing |
| `get_valuation_ratios` | `code`, `year`, `period` (`tw1`/`tw2`/`tw3`/`audit`, default `tw1`) | Market cap, PER, PBV, book value per share, annualized EPS, and dividend yield — computed from the latest official close, listed shares, and the XBRL filing for that period |
| `get_financial_growth` | `code`, `year`, `period` | YoY growth from a single XBRL filing: revenue/profit vs the same period a year earlier, balance sheet vs prior year-end, and net-margin trend |
| `get_foreign_flow_trend` | `code`, `days` (default 60) | Foreign accumulation/distribution: net flow (shares + approx IDR), foreign share of volume, and price change over 5/20/60-day windows, plus the current net-buy/net-sell streak |
| `screen_stocks` | `rank_by` (gainers/losers/value/volume/foreign buy/sell), `sector`, `min_value`, `min_price`, `max_price`, `limit` | Screen all ~950 stocks on the latest trading day, with listing board and special-notation flags per stock |
| `get_announcements` | `code`, `keyword` (optional, e.g. `dividen`), `days` (default 30), `limit` | Official IDX disclosures (keterbukaan informasi), newest first, with PDF attachment links — where dividend schedules, RUPS calls, and corporate actions are announced |
| `get_dividend_history` | `code`, `years` (1–5, default 3) | Multi-year timeline of dividend-related disclosures, newest first, with PDF links (amounts live in the PDFs; `get_dividends` has the latest structured declaration) |
| `get_issued_history` | `code` | Corporate actions that changed the issued share count (stock splits, rights issues, partial delistings), oldest first — for spotting dilution/float changes |
| `get_index_constituents` | `index` (e.g. `LQ45`, `IDX30`, `IDX80`, `KOMPAS100`) | Current index members from the latest official evaluation announcement: ticker, free-float ratio, index shares, capped weight, and new/kept/adjusted tag |
| `compare_stocks` | `codes` (2–5 tickers), `year`, `period` | Side-by-side valuation against the same filing period: price, market cap, PER, PBV, ROE, annualized EPS, BVPS, dividend yield (per-ticker errors degrade to error rows) |

Concept coverage is validated across sectors (bank, infrastructure, general conglomerate). Non-
financial issuers report `SalesAndRevenue`; banks report `InterestIncome`. Earnings-per-share is a
per-share ratio whose scale is inconsistent across issuers — treat it as advisory. See
[`docs/spike-findings.md`](docs/spike-findings.md) for the full data-source analysis.

## Requirements

- Go **1.25+** (only to build; the result is a standalone binary)

## Build

```sh
go build -o idx-mcp ./cmd/idx-mcp
```

Or install into your `GOBIN`:

```sh
go install github.com/lovanto/idx-mcp/cmd/idx-mcp@latest
```

## Configure your MCP client

`idx-mcp` speaks MCP over stdio. Point your client at the built binary.

### Claude Desktop

Edit `claude_desktop_config.json` (macOS:
`~/Library/Application Support/Claude/claude_desktop_config.json`,
Windows: `%APPDATA%\Claude\claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "idx": {
      "command": "/absolute/path/to/idx-mcp",
      "env": {
        "IDX_CACHE_PATH": "/absolute/path/to/idx-cache.db"
      }
    }
  }
}
```

Restart Claude Desktop; the three tools appear under the 🔌 menu.

### Claude Code

```sh
claude mcp add idx -- /absolute/path/to/idx-mcp
```

## Configuration

All configuration is via environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `IDX_CACHE_PATH` | `idx-cache.db` (cwd) | SQLite cache file. Use an absolute path when run from an MCP client. |
| `IDX_MIN_INTERVAL` | `15` | Minimum seconds between requests to idx.co.id. Keep it polite. |

Caching TTLs are tuned per data class: trading data ~6h, company profile ~30d, and financial
reports forever (immutable once published). Repeat questions are served from cache with no network
call.

## How it works

```
cmd/idx-mcp         stdio MCP server, registers the three tools
internal/idx        typed client — trading / profile / financial
internal/fetcher    Cloudflare-aware HTTP (Firefox TLS profile) + rate limit + backoff
internal/cache      SQLite TTL key/value cache (pure-Go modernc.org/sqlite)
internal/xbrl       streaming XBRL parser for IDX financial filings
```

idx.co.id sits behind Cloudflare. A plain `net/http` client (and even a Chrome TLS fingerprint) gets
a 403 challenge; a **Firefox** TLS profile via
[`bogdanfinn/tls-client`](https://github.com/bogdanfinn/tls-client) passes reliably without any
JavaScript challenge solver. The fetcher treats any HTML/non-JSON response as a WAF block and never
caches it.

## Development

```sh
go test ./...     # unit tests (synthetic fixtures, no network)
go vet ./...
```

The `cmd/spike-*` commands are the Phase 1 throwaway spikes kept for reference; the reusable code
lives under `internal/`.

## License

MIT — see [LICENSE](LICENSE). Note the data-usage caveat above: the MIT license covers this code,
not the IDX data it retrieves.
