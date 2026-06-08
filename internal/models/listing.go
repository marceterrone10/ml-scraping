package models

import "time"

// Data de los vehiculos que se scrapean
type Listing struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Brand         string            `json:"brand,omitempty"`
	Model         string            `json:"model,omitempty"`
	Year          int               `json:"year,omitempty"`
	Kilometers    int               `json:"kilometers,omitempty"`
	Price         float64           `json:"price"`
	OriginalPrice float64           `json:"original_price,omitempty"`
	Currency      string            `json:"currency"`
	Condition     string            `json:"condition,omitempty"`
	Location      string            `json:"location,omitempty"`
	SellerType    string            `json:"seller_type,omitempty"`
	URL           string            `json:"url"`
	ImageURL      string            `json:"image_url,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	Site          string            `json:"site"`
	ScrapedAt     time.Time         `json:"scraped_at"`
}

// Metadata
type SearchResult struct {
	Query        string    `json:"query"`
	Site         string    `json:"site"`
	TotalFound   int       `json:"total_found"`
	PagesScraped int       `json:"pages_scraped"`
	Listings     []Listing `json:"listings"`
	ScrapedAt    time.Time `json:"scraped_at"`
}
