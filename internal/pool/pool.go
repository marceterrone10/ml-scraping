package pool

import (
	"log"
	"sync"
	"time"

	"github.com/marceloterrone/car-scrapper/internal/config"
	"github.com/marceloterrone/car-scrapper/internal/models"
	"github.com/marceloterrone/car-scrapper/internal/scraper"
)

// Job is one brand/model search to scrape.
type Job struct {
	Brand string `json:"brand,omitempty"`
	Model string `json:"model,omitempty"`
	Query string `json:"query"`
}

// Result holds the outcome of a single job.
type Result struct {
	Job    Job
	Result *models.SearchResult
	Err    error
}

// Pool runs scrape jobs concurrently with a fixed number of workers.
type Pool struct {
	workers int
	baseCfg config.ScraperConfig
	verbose bool
}

// New creates a worker pool. workers defaults to 3 when <= 0.
func New(workers int, baseCfg config.ScraperConfig, verbose bool) *Pool {
	if workers <= 0 {
		workers = 3
	}
	return &Pool{
		workers: workers,
		baseCfg: baseCfg,
		verbose: verbose,
	}
}

// Run executes all jobs and returns one result per job (order not guaranteed).
func (p *Pool) Run(jobs []Job) []Result {
	jobsQuantity := len(jobs)
	jobsCh := make(chan Job)
	resultsCh := make(chan Result, jobsQuantity)

	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.worker(workerID, jobsCh, resultsCh) // consume jobs from jobsCh and send results to resultsCh
		}(i + 1)
	}

	go func() {
		for _, job := range jobs { // send jobs to jobsCh
			jobsCh <- job
		}
		close(jobsCh)
	}()

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	results := make([]Result, 0, len(jobs))
	for r := range resultsCh { // receive results from resultsCh
		results = append(results, r) // append results to results
	}
	return results
}

func (p *Pool) worker(id int, jobs <-chan Job, results chan<- Result) {
	for job := range jobs {
		if p.verbose {
			log.Printf("worker %d: starting job %q", id, job.Query)
		}

		cfg := p.baseCfg
		cfg.Query = job.Query
		cfg.Brand = job.Brand
		cfg.Model = job.Model
		cfg.Parallel = 1

		result, err := scraper.New(cfg).Run()
		if p.verbose {
			if err != nil {
				log.Printf("worker %d: job %q failed: %v", id, job.Query, err)
			} else {
				log.Printf("worker %d: job %q done (%d listings)", id, job.Query, result.TotalFound)
			}
		}

		results <- Result{Job: job, Result: result, Err: err}
	}
}

// MergeResults aggregates job results into a single BatchResult.
func MergeResults(site string, results []Result) *models.BatchResult {
	batch := &models.BatchResult{
		Site:      site,
		TotalJobs: len(results),
		Results:   make([]models.SearchResult, 0, len(results)),
	}

	var all []models.Listing
	for _, r := range results {
		if r.Err != nil {
			batch.Errors = append(batch.Errors, models.JobError{
				Query: r.Job.Query,
				Brand: r.Job.Brand,
				Model: r.Job.Model,
				Error: r.Err.Error(),
			})
			continue
		}
		if r.Result == nil {
			continue
		}
		batch.Results = append(batch.Results, *r.Result)
		all = append(all, r.Result.Listings...)
	}

	batch.Listings = scraper.DedupeListings(all)
	batch.TotalFound = len(batch.Listings)
	batch.ScrapedAt = time.Now().UTC()
	return batch
}
