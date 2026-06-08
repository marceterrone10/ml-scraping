package scraper

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
	"github.com/marceloterrone/car-scrapper/internal/config"
	"github.com/marceloterrone/car-scrapper/internal/models"
	"github.com/marceloterrone/car-scrapper/internal/parser"
)

const itemsPerPage = 48

// Scraper crawls MercadoLibre car listings.
type Scraper struct {
	cfg      config.ScraperConfig
	listings []models.Listing
	mu       sync.Mutex
	pages    int
	err      error
}

// New creates a configured scraper.
func New(cfg config.ScraperConfig) *Scraper {
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
	c, err := s.newCollector()
	if err != nil {
		return nil, err
	}

	c.OnRequest(func(r *colly.Request) {
		s.setBrowserHeaders(r)
	})

	c.OnResponse(func(r *colly.Response) {
		if isBlocked(r) {
			s.setError(blockedError())
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

	_ = s.warmup(c)

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

func (s *Scraper) newCollector() (*colly.Collector, error) {
	c := colly.NewCollector(
		colly.UserAgent(s.cfg.UserAgent),
		colly.Async(true),
	)

	if s.cfg.Verbose {
		c.SetDebugger(&debug.LogDebugger{})
	}

	if s.cfg.ProxyURL != "" {
		if err := c.SetProxy(s.cfg.ProxyURL); err != nil {
			return nil, fmt.Errorf("set proxy: %w", err)
		}
	}

	if s.cfg.Cookies != "" {
		cookies, err := LoadCookies(s.cfg.Cookies)
		if err != nil {
			return nil, err
		}
		c.OnRequest(func(r *colly.Request) {
			for _, cookie := range cookies {
				r.Headers.Set("Cookie", appendCookie(r.Headers.Get("Cookie"), cookie))
			}
		})
	}

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: s.cfg.Parallel,
		Delay:       s.cfg.Delay,
		RandomDelay: s.cfg.Delay / 2,
	})

	return c, nil
}

func appendCookie(existing string, cookie *http.Cookie) string {
	pair := cookie.Name + "=" + cookie.Value
	if existing == "" {
		return pair
	}
	return existing + "; " + pair
}

func (s *Scraper) warmup(c *colly.Collector) error {
	home := siteHome(s.cfg.Site)
	sync := c.Clone()
	sync.Async = false

	var warmupErr error
	sync.OnRequest(func(r *colly.Request) {
		s.setBrowserHeaders(r)
	})
	sync.OnResponse(func(r *colly.Response) {
		if isBlocked(r) {
			warmupErr = blockedError()
		}
	})
	sync.OnError(func(_ *colly.Response, err error) {
		warmupErr = err
	})

	if err := sync.Visit(home); err != nil {
		return err
	}
	return warmupErr
}

func (s *Scraper) setBrowserHeaders(r *colly.Request) {
	r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	r.Headers.Set("Accept-Language", siteLanguage(s.cfg.Site))
	r.Headers.Set("Cache-Control", "no-cache")
	r.Headers.Set("Sec-Fetch-Dest", "document")
	r.Headers.Set("Sec-Fetch-Mode", "navigate")
	r.Headers.Set("Sec-Fetch-Site", "same-origin")
	r.Headers.Set("Sec-Fetch-User", "?1")
	r.Headers.Set("Upgrade-Insecure-Requests", "1")
	if r.URL.String() != siteHome(s.cfg.Site) {
		r.Headers.Set("Referer", siteHome(s.cfg.Site))
	}
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

func siteHome(site string) string {
	switch site {
	case "MLB":
		return "https://www.mercadolivre.com.br/"
	case "MLM":
		return "https://www.mercadolibre.com.mx/"
	case "MLC":
		return "https://www.mercadolibre.cl/"
	case "MCO":
		return "https://www.mercadolibre.com.co/"
	case "MLU":
		return "https://www.mercadolibre.com.uy/"
	default:
		return "https://www.mercadolibre.com.ar/"
	}
}

func siteLanguage(site string) string {
	switch site {
	case "MLB":
		return "pt-BR,pt;q=0.9"
	default:
		return "es-AR,es;q=0.9,en;q=0.8"
	}
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

func blockedError() error {
	return fmt.Errorf(`blocked by MercadoLibre anti-bot.

MercadoLibre redirects automated requests to a login wall. Try:

  1. Export fresh cookies from mercadolibre.com.ar while logged in
  2. Set ML_COOKIES=cookies.json in .env or pass -cookies cookies.json
  3. Increase -delay (e.g. 4s) or use -proxy with a residential proxy`)
}

func isBlocked(r *colly.Response) bool {
	reqURL := strings.ToLower(r.Request.URL.String())
	if strings.Contains(reqURL, "account-verification") || strings.Contains(reqURL, "suspicious-traffic") {
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
