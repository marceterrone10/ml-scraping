package pool

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// LoadJobs reads a JSON array of jobs from path.
func LoadJobs(path string) ([]Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read jobs file: %w", err)
	}

	var jobs []Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, fmt.Errorf("parse jobs file: %w", err)
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("jobs file %s is empty", path)
	}

	out := make([]Job, 0, len(jobs))
	for i, job := range jobs {
		job.Query = strings.TrimSpace(job.Query)
		if job.Query == "" {
			brand := strings.TrimSpace(job.Brand)
			model := strings.TrimSpace(job.Model)
			if brand == "" && model == "" {
				return nil, fmt.Errorf("job %d: query or brand/model required", i+1)
			}
			job.Query = strings.TrimSpace(brand + " " + model)
		}
		out = append(out, job)
	}
	return out, nil
}
