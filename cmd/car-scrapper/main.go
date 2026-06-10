package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marceloterrone/car-scrapper/internal/config"
	"github.com/marceloterrone/car-scrapper/internal/pool"
	"github.com/marceloterrone/car-scrapper/internal/scraper"

	"github.com/go-co-op/gocron/v2"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	loc, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		log.Fatal(err)
	}

	//Scheduler
	s, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		log.Fatal(err)
	}

	_, err = s.NewJob( // newjob accepts definition and task.
		// definition
		gocron.DailyJob(
			1, // one time each day
			gocron.NewAtTimes(gocron.NewAtTime(12, 0, 0)), // @ 12:00:00
		),
		gocron.NewTask(func() { // execution
			if cfg.JobsFile != "" {
				runBatch(cfg)
			} else {
				runSingle(cfg)
			}
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	s.Start()
}

func runSingle(cfg config.Config) {
	log.Printf("scraping %s cars (query=%q, pages=%d)", cfg.Scraper().Site, cfg.Query, cfg.MaxPages)

	s := scraper.New(cfg.Scraper())
	result, err := s.Run()
	if err != nil {
		log.Fatalf("scrape failed: %v", err)
	}

	if err := writeOutput(cfg.Output, result); err != nil {
		log.Fatalf("write output: %v", err)
	}

	log.Printf("done: %d listings saved to %s", result.TotalFound, cfg.Output)
}

func runBatch(cfg config.Config) {
	jobs, err := pool.LoadJobs(cfg.JobsFile)
	if err != nil {
		log.Fatalf("load jobs: %v", err)
	}

	log.Printf("scraping %s cars (%d jobs, %d workers, pages=%d)",
		cfg.Scraper().Site, len(jobs), cfg.Workers, cfg.MaxPages)

	p := pool.New(cfg.Workers, cfg.Scraper(), cfg.Verbose)
	results := p.Run(jobs)

	batch := pool.MergeResults(cfg.Scraper().Site, results)
	for _, e := range batch.Errors {
		log.Printf("job %q failed: %s", e.Query, e.Error)
	}

	if err := writeOutput(cfg.Output, batch); err != nil {
		log.Fatalf("write output: %v", err)
	}

	log.Printf("done: %d listings from %d jobs saved to %s",
		batch.TotalFound, batch.TotalJobs, cfg.Output)
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
