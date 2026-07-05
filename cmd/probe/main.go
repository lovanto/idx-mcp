// Command probe is a throwaway endpoint prober for Phase 12 discovery.
// Usage: go run ./cmd/probe <outdir> <url>...
// It fetches each URL through the Cloudflare-aware fetcher (15s apart) and
// writes the raw body to <outdir>/<n>.out. Not part of the server; delete
// or ignore for commits.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lovanto/idx-mcp/internal/fetcher"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: probe <outdir> <url>...")
		os.Exit(2)
	}
	outdir := os.Args[1]
	f, err := fetcher.New(fetcher.Config{MinInterval: 15 * time.Second})
	if err != nil {
		panic(err)
	}
	for i, u := range os.Args[2:] {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		body, err := f.Get(ctx, u)
		cancel()
		name := filepath.Join(outdir, fmt.Sprintf("%d.out", i))
		if err != nil {
			fmt.Printf("[%d] %s\n    ERROR: %v\n", i, u, err)
			continue
		}
		if werr := os.WriteFile(name, body, 0o644); werr != nil {
			panic(werr)
		}
		peek := body
		if len(peek) > 200 {
			peek = peek[:200]
		}
		fmt.Printf("[%d] %s\n    %d bytes -> %s\n    peek: %s\n", i, u, len(body), name, string(peek))
	}
}
