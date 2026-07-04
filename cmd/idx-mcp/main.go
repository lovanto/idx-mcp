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
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lovanto/idx-mcp/internal/cache"
	"github.com/lovanto/idx-mcp/internal/fetcher"
	"github.com/lovanto/idx-mcp/internal/idx"
)

const version = "0.1.0"

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

	client := idx.New(f, c)
	server := mcp.NewServer(&mcp.Implementation{Name: "idx-mcp", Version: version}, nil)
	registerTools(server, client)

	log.Printf("idx-mcp %s ready (cache=%s, min-interval=%s)", version, cachePath, minInterval())
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

// ---- Tool: get_financial_report ----

type financialInput struct {
	Code   string `json:"code" jsonschema:"emiten ticker, e.g. BBCA"`
	Year   string `json:"year" jsonschema:"report year, e.g. 2026"`
	Period string `json:"period,omitempty" jsonschema:"reporting period: tw1, tw2, tw3, or audit (default tw1)"`
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
		Name:        "get_financial_report",
		Description: "Key financial-statement accounts (assets, liabilities, equity, profit, etc.) parsed from an IDX-listed company's official XBRL filing.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in financialInput) (*mcp.CallToolResult, idx.FinancialReport, error) {
		rep, err := client.FinancialReport(ctx, in.Code, in.Year, in.Period)
		if err != nil {
			return toolError(err), idx.FinancialReport{}, nil
		}
		return nil, *rep, nil
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
