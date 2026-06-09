package config

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

// Config holds application configuration from flags and environment.
type Config struct {
	Site     string
	Query    string
	MaxPages int
	Delay    time.Duration
	Output   string
	Cookies  string
	ProxyURL string
	Verbose  bool
}

// ScraperConfig is the subset passed to the MercadoLibre scraper.
type ScraperConfig struct {
	Site      string
	Query     string
	Brand     string
	Model     string
	MaxPages  int
	Delay     time.Duration
	Parallel  int
	UserAgent string
	Cookies   string
	ProxyURL  string
	Verbose   bool
}

// Scraper returns scraper settings derived from this config.
func (c Config) Scraper() ScraperConfig {
	return ScraperConfig{
		Site:     strings.ToUpper(c.Site),
		Query:    c.Query,
		MaxPages: c.MaxPages,
		Delay:    c.Delay,
		Cookies:  c.Cookies,
		ProxyURL: c.ProxyURL,
		Verbose:  c.Verbose,
	}
}

// Load parses flags and environment into Config.
func Load() (Config, error) {
	LoadEnv()

	cfg := Config{}
	flag.StringVar(&cfg.Site, "site", "MLA", "MercadoLibre site ID (MLA, MLB, MLM, MLC, MCO, MLU)")
	flag.StringVar(&cfg.Query, "query", "", "Search query (e.g. toyota corolla 2020)")
	flag.IntVar(&cfg.MaxPages, "pages", 1, "Number of result pages to scrape")
	flag.DurationVar(&cfg.Delay, "delay", 2*time.Second, "Delay between requests")
	flag.StringVar(&cfg.Output, "output", "output/listings.json", "Output file path (.json or .csv)")
	flag.StringVar(&cfg.Cookies, "cookies", GetEnv("ML_COOKIES", ""), "Path to browser-exported cookies file")
	flag.StringVar(&cfg.ProxyURL, "proxy", "", "HTTP proxy URL (e.g. http://user:pass@host:port)")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable debug logging")
	flag.Parse()

	if strings.TrimSpace(cfg.Cookies) == "" {
		return Config{}, fmt.Errorf(`browser cookies are required.

Add to .env:
  ML_COOKIES=cookies.json

Or pass -cookies cookies.json

Export cookies from mercadolibre.com.ar while logged in (browser extension).`)
	}

	return cfg, nil
}
