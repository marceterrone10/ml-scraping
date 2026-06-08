package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marceloterrone/car-scrapper/internal/scraper"
)

func main() {
	site := flag.String("site", "MLA", "MercadoLibre site ID (MLA, MLB, MLM, MLC, MCO, MLU)")
	query := flag.String("query", "", "Search query (e.g. toyota corolla 2020)")
	pages := flag.Int("pages", 1, "Number of result pages to scrape (48 listings per page)")
	delay := flag.Duration("delay", 2*time.Second, "Delay between requests")
	output := flag.String("output", "output/listings.json", "Output file path (.json or .csv)")
	verbose := flag.Bool("verbose", false, "Enable debug logging")
	flag.Parse()

	cfg := scraper.Config{
		Site:     strings.ToUpper(*site),
		Query:    *query,
		MaxPages: *pages,
		Delay:    *delay,
		Verbose:  *verbose,
	}

	log.Printf("scraping %s cars (query=%q, pages=%d)", cfg.Site, cfg.Query, cfg.MaxPages)

	s := scraper.New(cfg)
	result, err := s.Run()
	if err != nil {
		log.Fatalf("scrape failed: %v", err)
	}

	if err := writeOutput(*output, result); err != nil {
		log.Fatalf("write output: %v", err)
	}

	log.Printf("done: %d listings saved to %s", result.TotalFound, *output)
}

func writeOutput(path string, result any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return writeCSV(path, result)
	default:
		return writeJSON(path, result)
	}
}

func writeJSON(path string, result any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func writeCSV(path string, result any) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	var parsed struct {
		Listings []map[string]any `json:"listings"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	headers := []string{
		"id", "title", "brand", "model", "year", "kilometers",
		"price", "currency", "condition", "location", "url", "image_url", "site",
	}
	if err := w.Write(headers); err != nil {
		return err
	}

	for _, item := range parsed.Listings {
		row := make([]string, len(headers))
		for i, h := range headers {
			if v, ok := item[h]; ok && v != nil {
				row[i] = fmt.Sprint(v)
			}
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
