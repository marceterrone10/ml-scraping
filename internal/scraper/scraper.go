package scraper

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
	"github.com/marceloterrone/car-scrapper/internal/models"
	"github.com/marceloterrone/car-scrapper/internal/parser"
)

const itemsPerPage = 48

// Config controls scraper behavior.
type Config struct {
	Site      string
	Query     string
	MaxPages  int
	Delay     time.Duration
	Parallel  int
	UserAgent string
	Verbose   bool
}

// Scraper crawls MercadoLibre car listings.
type Scraper struct {
	cfg      Config
	listings []models.Listing
	mu       sync.Mutex
	pages    int
	err      error
}

// New creates a configured scraper.
func New(cfg Config) *Scraper {
	if cfg.Site == "" {
		cfg.Site = "MLA"
	}
	if cfg.MaxPages <= 0 {
		cfg.MaxPages = 1
	}
	if cfg.Delay <= 0 {
		cfg.Delay = 2 * time.Second
	}
	if cfg.Parallel <= 0 {
		cfg.Parallel = 1
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	}
	return &Scraper{cfg: cfg}
}

// Run executes the scrape and returns aggregated results.
func (s *Scraper) Run() (*models.SearchResult, error) {
	c := colly.NewCollector(
		colly.UserAgent(s.cfg.UserAgent),
		colly.Async(true),
	)

	if s.cfg.Verbose {
		c.SetDebugger(&debug.LogDebugger{})
	}

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: s.cfg.Parallel,
		Delay:       s.cfg.Delay,
		RandomDelay: s.cfg.Delay / 2,
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "es-AR,es;q=0.9,en;q=0.8")
		r.Headers.Set("Accept-Encoding", "gzip, deflate, br")
		r.Headers.Set("Cache-Control", "no-cache")
		r.Headers.Set("Sec-Fetch-Dest", "document")
		r.Headers.Set("Sec-Fetch-Mode", "navigate")
		r.Headers.Set("Sec-Fetch-Site", "none")
		r.Headers.Set("Sec-Fetch-User", "?1")
		r.Headers.Set("Upgrade-Insecure-Requests", "1")
	})

	c.OnResponse(func(r *colly.Response) {
		if isBlocked(r) {
			s.setError(fmt.Errorf("blocked by MercadoLibre anti-bot — run from a residential network and increase -delay"))
			return
		}

		found, err := parser.ParsePage(r.Body, s.cfg.Site)
		if err != nil {
			s.setError(err)
			return
		}

		s.mu.Lock()
		s.listings = append(s.listings, found...)
		s.pages++
		s.mu.Unlock()
	})

	c.OnError(func(r *colly.Response, err error) {
		s.setError(fmt.Errorf("request failed %s: %w", r.Request.URL, err))
	})

	startURL := s.buildSearchURL(1)
	if err := c.Visit(startURL); err != nil {
		return nil, fmt.Errorf("visit start URL: %w", err)
	}

	for page := 2; page <= s.cfg.MaxPages; page++ {
		pageURL := s.buildSearchURL(page)
		if err := c.Visit(pageURL); err != nil {
			return nil, fmt.Errorf("visit page %d: %w", page, err)
		}
	}

	c.Wait()

	if s.err != nil && len(s.listings) == 0 {
		return nil, s.err
	}

	unique := dedupe(s.listings)
	return &models.SearchResult{
		Query:        s.cfg.Query,
		Site:         s.cfg.Site,
		TotalFound:   len(unique),
		PagesScraped: s.pages,
		Listings:     unique,
		ScrapedAt:    time.Now().UTC(),
	}, nil
}

func (s *Scraper) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

func (s *Scraper) buildSearchURL(page int) string {
	base := listadoBase(s.cfg.Site)
	path := "vehiculos/autos-camionetas"

	query := strings.TrimSpace(s.cfg.Query)
	if query != "" {
		path = path + "/" + slugifyQuery(query)
	}

	u, _ := url.Parse(base + "/" + path)
	if page > 1 {
		offset := (page-1)*itemsPerPage + 1
		u.Path = strings.TrimSuffix(u.Path, "/") + fmt.Sprintf("/_Desde_%d_NoIndex_True", offset)
	}
	return u.String()
}

func listadoBase(site string) string {
	switch site {
	case "MLB":
		return "https://lista.mercadolivre.com.br"
	case "MLM":
		return "https://listado.mercadolibre.com.mx"
	case "MLC":
		return "https://listado.mercadolibre.cl"
	case "MCO":
		return "https://listado.mercadolibre.com.co"
	case "MLU":
		return "https://listado.mercadolibre.com.uy"
	default:
		return "https://listado.mercadolibre.com.ar"
	}
}

func slugifyQuery(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	q = strings.ReplaceAll(q, " ", "-")
	return q
}

func isBlocked(r *colly.Response) bool {
	url := strings.ToLower(r.Request.URL.String())
	if strings.Contains(url, "account-verification") || strings.Contains(url, "suspicious-traffic") {
		return true
	}
	body := strings.ToLower(string(r.Body))
	markers := []string{
		"account-verification",
		"suspicious-traffic",
		"para continuar, ingresa",
		"negative_traffic",
	}
	for _, m := range markers {
		if strings.Contains(body, m) {
			return true
		}
	}
	return false
}

func dedupe(listings []models.Listing) []models.Listing {
	seen := make(map[string]struct{}, len(listings))
	out := make([]models.Listing, 0, len(listings))
	for _, l := range listings {
		if l.ID == "" {
			continue
		}
		if _, ok := seen[l.ID]; ok {
			continue
		}
		seen[l.ID] = struct{}{}
		out = append(out, l)
	}
	return out
}
