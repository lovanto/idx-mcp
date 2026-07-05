// Command idx-mcp is an MCP server exposing official Bursa Efek Indonesia
// (idx.co.id) market data as tools over stdio. Unlike yfinance-based
// alternatives, data comes from IDX's own endpoints, giving reliable
// fundamentals and official foreign-flow figures.
//
// Tools:
//   - get_trading_info:    daily OHLCV + foreign buy/sell for an emiten
//   - get_company_profile: listing/sector metadata + recent dividends
//   - get_financial_report: key accounts parsed from official XBRL filings
//
// Config via environment:
//   - IDX_CACHE_PATH:     SQLite cache file (default: idx-cache.db)
//   - IDX_MIN_INTERVAL:   min seconds between IDX requests (default: 15)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lovanto/idx-mcp/internal/cache"
	"github.com/lovanto/idx-mcp/internal/fetcher"
	"github.com/lovanto/idx-mcp/internal/idx"
)

// version is stamped at release time via -ldflags "-X main.version=...".
// For `go install`, resolveVersion falls back to the module version.
var version = "dev"

// resolveVersion returns the ldflags-injected version, or the module version
// recorded in the build info (as with `go install ...@v0.1.0`), or "dev".
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("idx-mcp: %v", err)
	}
}

func run() error {
	// Logs must go to stderr; stdout is the MCP JSON-RPC channel.
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	cachePath := envOr("IDX_CACHE_PATH", "idx-cache.db")
	c, err := cache.Open(cachePath)
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}
	defer c.Close()

	f, err := fetcher.New(fetcher.Config{MinInterval: minInterval()})
	if err != nil {
		return fmt.Errorf("build fetcher: %w", err)
	}

	ver := resolveVersion()
	client := idx.New(f, c)
	server := mcp.NewServer(&mcp.Implementation{Name: "idx-mcp", Version: ver}, nil)
	registerTools(server, client)

	log.Printf("idx-mcp %s ready (cache=%s, min-interval=%s)", ver, cachePath, minInterval())
	return server.Run(context.Background(), &mcp.StdioTransport{})
}

// ---- Tool: get_trading_info ----

type tradingInput struct {
	Code   string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
	Length int    `json:"length,omitempty" jsonschema:"number of recent trading days to return (1-365, default 30)"`
}

type tradingOutput struct {
	Code string           `json:"code"`
	Days []idx.TradingDay `json:"days"`
}

// ---- Tool: get_company_profile ----

type profileInput struct {
	Code string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
}

// ---- Tool: get_dividends ----

type dividendInput struct {
	Code string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
}

type dividendOutput struct {
	Code      string         `json:"code"`
	Dividends []idx.Dividend `json:"dividends"`
	Note      string         `json:"note"`
}

// ---- Tool: get_shareholders ----

type shareholderInput struct {
	Code string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
}

type shareholderOutput struct {
	Code         string            `json:"code"`
	Shareholders []idx.Shareholder `json:"shareholders"`
}

// ---- Tool: get_subsidiaries ----

type subsidiaryInput struct {
	Code string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
}

type subsidiaryOutput struct {
	Code         string           `json:"code"`
	Subsidiaries []idx.Subsidiary `json:"subsidiaries"`
}

// ---- Tool: get_management ----

type managementInput struct {
	Code string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
}

// ---- Tool: get_market_index ----

type marketIndexInput struct {
	Code string `json:"code,omitempty" jsonschema:"index code to filter by, e.g. COMPOSITE (IHSG), LQ45, IDX30; omit for all indices"`
}

type marketIndexOutput struct {
	Indices []idx.MarketIndex `json:"indices"`
}

// ---- Tool: get_market_summary ----

type marketSummaryInput struct{}

// ---- Tool: get_sector_summary ----

type sectorSummaryInput struct{}

// ---- Tool: get_broker_summary ----

type brokerSummaryInput struct{}

type brokerSummaryOutput struct {
	Brokers []idx.BrokerActivity `json:"brokers"`
}

// ---- Tool: list_companies ----

type listCompaniesInput struct {
	Query  string `json:"query,omitempty" jsonschema:"case-insensitive substring to match against ticker code or company name, e.g. bank or BBC; omit to list all"`
	Sector string `json:"sector,omitempty" jsonschema:"optional sector/sub-sector substring filter, e.g. Energi or Keuangan"`
}

// ---- Tool: get_financial_report ----

type financialInput struct {
	Code   string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
	Year   string `json:"year" jsonschema:"report year, e.g. 2026"`
	Period string `json:"period,omitempty" jsonschema:"reporting period: tw1, tw2, tw3, or audit (default tw1)"`
}

// ---- Tool: get_announcements ----

type announcementInput struct {
	Code    string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
	Keyword string `json:"keyword,omitempty" jsonschema:"optional search keyword, e.g. dividen or RUPS"`
	Days    int    `json:"days,omitempty" jsonschema:"look-back window in days (1-365, default 30)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum announcements to return (default 20, max 50)"`
}

// ---- Tool: screen_stocks ----

type screenerInput struct {
	RankBy   string  `json:"rank_by,omitempty" jsonschema:"ranking: top_gainers, top_losers, most_active_value, most_active_volume, top_foreign_buy, or top_foreign_sell (default most_active_value)"`
	Sector   string  `json:"sector,omitempty" jsonschema:"optional sector substring filter, e.g. Energi or Keuangan"`
	MinValue float64 `json:"min_value,omitempty" jsonschema:"minimum traded value in IDR (filters out illiquid stocks)"`
	MinPrice float64 `json:"min_price,omitempty" jsonschema:"minimum closing price in IDR"`
	MaxPrice float64 `json:"max_price,omitempty" jsonschema:"maximum closing price in IDR"`
	Limit    int     `json:"limit,omitempty" jsonschema:"maximum rows to return (default 20, max 100)"`
}

// ---- Tool: get_financial_growth ----

type growthInput struct {
	Code   string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
	Year   string `json:"year" jsonschema:"report year, e.g. 2026"`
	Period string `json:"period,omitempty" jsonschema:"reporting period: tw1, tw2, tw3, or audit (default tw1)"`
}

// ---- Tool: get_foreign_flow_trend ----

type flowTrendInput struct {
	Code string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
	Days int    `json:"days,omitempty" jsonschema:"number of recent trading days to analyse (1-365, default 60)"`
}

// ---- Tool: get_valuation_ratios ----

type valuationInput struct {
	Code   string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
	Year   string `json:"year" jsonschema:"financial report year to value against, e.g. 2026"`
	Period string `json:"period,omitempty" jsonschema:"reporting period: tw1, tw2, tw3, or audit (default tw1)"`
}

// ---- Tool: get_issued_history ----

type issuedInput struct {
	Code string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
}

// ---- Tool: get_index_constituents ----

type constituentsInput struct {
	Index string `json:"index" jsonschema:"index code, e.g. LQ45, IDX30, IDX80, KOMPAS100, BISNIS-27, MNC36, SMINFRA18, IDXESGL"`
}

// ---- Tool: compare_stocks ----

type compareInput struct {
	Codes  []string `json:"codes" jsonschema:"2-5 emiten tickers to compare, e.g. [\"BBCA\",\"BBRI\",\"BMRI\"]"`
	Year   string   `json:"year" jsonschema:"financial report year to value against, e.g. 2026"`
	Period string   `json:"period,omitempty" jsonschema:"reporting period: tw1, tw2, tw3, or audit (default tw1)"`
}

// ---- Tool: get_dividend_history ----

type divHistoryInput struct {
	Code  string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
	Years int    `json:"years,omitempty" jsonschema:"look-back window in years (1-5, default 3)"`
}

func registerTools(s *mcp.Server, client *idx.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_trading_info",
		Description: "Daily OHLCV plus official foreign buy/sell flow for an IDX-listed company, most recent day first.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in tradingInput) (*mcp.CallToolResult, tradingOutput, error) {
		days, err := client.TradingInfo(ctx, in.Code, in.Length)
		if err != nil {
			return toolError(err), tradingOutput{}, nil
		}
		return nil, tradingOutput{Code: in.Code, Days: days}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_company_profile",
		Description: "Listing and sector metadata plus recent dividends for an IDX-listed company.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in profileInput) (*mcp.CallToolResult, idx.CompanyProfile, error) {
		p, err := client.CompanyProfile(ctx, in.Code)
		if err != nil {
			return toolError(err), idx.CompanyProfile{}, nil
		}
		return nil, *p, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_dividends",
		Description: "Most recently declared dividend(s) for an IDX-listed company: cash per share, key dates (cum/ex/record/payment), and bonus-share ratio if any.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in dividendInput) (*mcp.CallToolResult, dividendOutput, error) {
		divs, err := client.Dividends(ctx, in.Code)
		if err != nil {
			return toolError(err), dividendOutput{}, nil
		}
		return nil, dividendOutput{
			Code:      in.Code,
			Dividends: divs,
			Note:      "Reflects the most recently declared dividend(s) from IDX company data, not full historical series.",
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_shareholders",
		Description: "Ownership structure of an IDX-listed company: each holder's name, category, share count, percentage, and whether they are the controlling shareholder, largest first.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in shareholderInput) (*mcp.CallToolResult, shareholderOutput, error) {
		sh, err := client.Shareholders(ctx, in.Code)
		if err != nil {
			return toolError(err), shareholderOutput{}, nil
		}
		return nil, shareholderOutput{Code: in.Code, Shareholders: sh}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_subsidiaries",
		Description: "Consolidated subsidiaries of an IDX-listed company: name, line of business, location, ownership percentage, total assets, and operation status, largest ownership first.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in subsidiaryInput) (*mcp.CallToolResult, subsidiaryOutput, error) {
		subs, err := client.Subsidiaries(ctx, in.Code)
		if err != nil {
			return toolError(err), subsidiaryOutput{}, nil
		}
		return nil, subsidiaryOutput{Code: in.Code, Subsidiaries: subs}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_management",
		Description: "Board of an IDX-listed company: directors (with affiliation flag) and commissioners (with independence flag), plus their positions.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in managementInput) (*mcp.CallToolResult, idx.Management, error) {
		m, err := client.Management(ctx, in.Code)
		if err != nil {
			return toolError(err), idx.Management{}, nil
		}
		return nil, *m, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_market_index",
		Description: "Latest end-of-day summary for IDX market indices (COMPOSITE/IHSG, LQ45, IDX30, and ~45 others): OHLC-style values, change, change %, volume, value, and market cap. Pass a code to filter, or omit for all.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in marketIndexInput) (*mcp.CallToolResult, marketIndexOutput, error) {
		idxs, err := client.MarketIndices(ctx, in.Code)
		if err != nil {
			return toolError(err), marketIndexOutput{}, nil
		}
		return nil, marketIndexOutput{Indices: idxs}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_market_summary",
		Description: "Exchange-wide summary for the latest IDX trading day: market breadth (advancing/declining/unchanged), total volume/value/frequency, official net foreign flow, and the day's top gainers and losers.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ marketSummaryInput) (*mcp.CallToolResult, idx.MarketSummary, error) {
		sum, err := client.MarketSummary(ctx)
		if err != nil {
			return toolError(err), idx.MarketSummary{}, nil
		}
		return nil, *sum, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_sector_summary",
		Description: "Per-sector market roll-up for the latest IDX trading day: for each sector, how many of its stocks traded, market breadth (advancing/declining/unchanged), total traded value, and net foreign flow, sorted by traded value. Built by joining the company directory with the daily stock feed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ sectorSummaryInput) (*mcp.CallToolResult, idx.SectorSummary, error) {
		sum, err := client.SectorSummary(ctx)
		if err != nil {
			return toolError(err), idx.SectorSummary{}, nil
		}
		return nil, *sum, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_broker_summary",
		Description: "Per-broker trading activity for the latest IDX trading day (all ~88 brokers): firm code, name, volume, value, and frequency, sorted by traded value (most active first).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ brokerSummaryInput) (*mcp.CallToolResult, brokerSummaryOutput, error) {
		brokers, err := client.BrokerSummary(ctx)
		if err != nil {
			return toolError(err), brokerSummaryOutput{}, nil
		}
		return nil, brokerSummaryOutput{Brokers: brokers}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_companies",
		Description: "Search or browse the IDX listed-company directory (~957 companies) to discover ticker codes: filter by a case-insensitive query on code/name and/or a sector substring. Returns code, name, listing board/date, sector, and industry; capped at 100 results (truncated flag set when more matched).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listCompaniesInput) (*mcp.CallToolResult, idx.CompanyDirectory, error) {
		dir, err := client.ListCompanies(ctx, in.Query, in.Sector)
		if err != nil {
			return toolError(err), idx.CompanyDirectory{}, nil
		}
		return nil, *dir, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_financial_report",
		Description: "Key financial-statement accounts (assets, liabilities, equity, profit, etc.) parsed from an IDX-listed company's official XBRL filing.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in financialInput) (*mcp.CallToolResult, idx.FinancialReport, error) {
		rep, err := client.FinancialReport(ctx, in.Code, in.Year, in.Period)
		if err != nil {
			return toolError(err), idx.FinancialReport{}, nil
		}
		return nil, *rep, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_announcements",
		Description: "Official IDX disclosures (keterbukaan informasi) for a listed company, newest first: announcement number, date, title, and PDF attachment links. Filter by keyword (e.g. dividen, RUPS) and look-back window. This is where dividend schedules, RUPS calls, and corporate actions are announced.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in announcementInput) (*mcp.CallToolResult, idx.AnnouncementList, error) {
		list, err := client.Announcements(ctx, in.Code, in.Keyword, in.Days, in.Limit)
		if err != nil {
			return toolError(err), idx.AnnouncementList{}, nil
		}
		return nil, *list, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "screen_stocks",
		Description: "Screen the latest IDX trading day across all ~950 stocks: rank by gainers/losers, traded value/volume, or net foreign buy/sell, with optional sector, price-band, and minimum-liquidity filters. Price/flow data only (no fundamental screening).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in screenerInput) (*mcp.CallToolResult, idx.ScreenerResult, error) {
		res, err := client.ScreenStocks(ctx, in.RankBy, in.Sector, in.MinValue, in.MinPrice, in.MaxPrice, in.Limit)
		if err != nil {
			return toolError(err), idx.ScreenerResult{}, nil
		}
		return nil, *res, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_financial_growth",
		Description: "Year-over-year growth analysis from an IDX-listed company's XBRL filing: revenue and profit vs the same period a year earlier, balance sheet vs the prior year-end, growth percentages, and net margin trend. Uses the comparatives embedded in a single filing.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in growthInput) (*mcp.CallToolResult, idx.FinancialGrowth, error) {
		g, err := client.FinancialGrowth(ctx, in.Code, in.Year, in.Period)
		if err != nil {
			return toolError(err), idx.FinancialGrowth{}, nil
		}
		return nil, *g, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_foreign_flow_trend",
		Description: "Foreign accumulation/distribution analysis for an IDX-listed stock: net foreign flow (shares and approximate IDR value), foreign share of volume, and price change over 5/20/60-day windows, plus the current consecutive net-buy/net-sell streak. Built from official IDX foreign flow data.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in flowTrendInput) (*mcp.CallToolResult, idx.ForeignFlowTrend, error) {
		tr, err := client.ForeignFlowTrend(ctx, in.Code, in.Days)
		if err != nil {
			return toolError(err), idx.ForeignFlowTrend{}, nil
		}
		return nil, *tr, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_valuation_ratios",
		Description: "Valuation ratios for an IDX-listed company: market cap, PER, PBV, book value per share, and annualized EPS — computed from the latest official close, listed shares, and the XBRL filing for the given year/period.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in valuationInput) (*mcp.CallToolResult, idx.ValuationRatios, error) {
		v, err := client.ValuationRatios(ctx, in.Code, in.Year, in.Period)
		if err != nil {
			return toolError(err), idx.ValuationRatios{}, nil
		}
		return nil, *v, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_index_constituents",
		Description: "Current members of an IDX stock index (LQ45, IDX30, IDX80, KOMPAS100, …) from the latest official evaluation announcement: ticker, free-float ratio, index shares, capped weight, and whether the member is new/kept/adjusted. Parsed from the announcement's attached workbook.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in constituentsInput) (*mcp.CallToolResult, idx.IndexConstituents, error) {
		ic, err := client.IndexConstituents(ctx, in.Index)
		if err != nil {
			return toolError(err), idx.IndexConstituents{}, nil
		}
		return nil, *ic, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "compare_stocks",
		Description: "Side-by-side valuation of 2-5 IDX-listed companies against the same report year/period: price, market cap, PER, PBV, ROE, annualized EPS, book value per share, and dividend yield. A ticker without data degrades to an error row. Uncached tickers each cost several rate-limited fetches, so a cold comparison can take minutes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in compareInput) (*mcp.CallToolResult, idx.StockComparison, error) {
		cmp, err := client.CompareStocks(ctx, in.Codes, in.Year, in.Period)
		if err != nil {
			return toolError(err), idx.StockComparison{}, nil
		}
		return nil, *cmp, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_dividend_history",
		Description: "Multi-year timeline of dividend-related official disclosures for an IDX-listed company, newest first: schedule announcements (cum/ex/payment dates) with PDF attachment links. Amounts live in the PDFs; use get_dividends for the latest structured declaration.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in divHistoryInput) (*mcp.CallToolResult, idx.DividendHistory, error) {
		h, err := client.DividendHistory(ctx, in.Code, in.Years)
		if err != nil {
			return toolError(err), idx.DividendHistory{}, nil
		}
		return nil, *h, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_issued_history",
		Description: "Corporate actions that changed an IDX-listed company's issued share count (stock splits, rights issues, partial delistings, …), oldest first — useful for spotting dilution or float changes behind a price history.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in issuedInput) (*mcp.CallToolResult, idx.IssuedHistory, error) {
		h, err := client.IssuedHistory(ctx, in.Code)
		if err != nil {
			return toolError(err), idx.IssuedHistory{}, nil
		}
		return nil, *h, nil
	})
}

// toolError renders an error as an MCP tool result with IsError set, so the
// model sees the failure reason instead of the call silently breaking.
func toolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func minInterval() time.Duration {
	if v := os.Getenv("IDX_MIN_INTERVAL"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return 15 * time.Second
}
