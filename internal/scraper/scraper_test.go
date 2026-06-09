package scraper

import (
	"testing"

	"github.com/marceloterrone/car-scrapper/internal/config"
)

func TestBuildSearchURL_brandModel(t *testing.T) {
	s := New(config.ScraperConfig{
		Site:  "MLA",
		Brand: "Toyota",
		Model: "Corolla",
	})

	got := s.buildSearchURL(1)
	want := "https://autos.mercadolibre.com.ar/toyota/corolla"
	if got != want {
		t.Fatalf("page 1: got %q want %q", got, want)
	}

	got = s.buildSearchURL(2)
	want = "https://autos.mercadolibre.com.ar/toyota/corolla/_Desde_49_NoIndex_True"
	if got != want {
		t.Fatalf("page 2: got %q want %q", got, want)
	}
}

func TestBuildSearchURL_fromQuery(t *testing.T) {
	s := New(config.ScraperConfig{
		Site:  "MLA",
		Query: "ford ranger",
	})

	got := s.buildSearchURL(1)
	want := "https://autos.mercadolibre.com.ar/ford/ranger"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSplitQuery(t *testing.T) {
	brand, model := splitQuery("chevrolet onix")
	if brand != "chevrolet" || model != "onix" {
		t.Fatalf("got %q / %q", brand, model)
	}
}
