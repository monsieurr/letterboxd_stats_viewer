package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

const (
	baseURL           = "https://api.themoviedb.org/3/"
	apiKeyEnv         = "TMDB_API_KEY"
	outputFile        = "output.json"
	rateLimitPerSec   = 20 // Conservative rate limit (well below the 50/sec limit)
	maxConcurrentReqs = 5  // Maximum concurrent requests
)

var (
	apiKey      string
	httpClient  = &http.Client{Timeout: 10 * time.Second}
	rateLimiter = time.NewTicker(time.Second / time.Duration(rateLimitPerSec))
)

type Movie struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	ReleaseDate   string  `json:"release_date"`
	PosterPath    string  `json:"poster_path"`
	Popularity    float64 `json:"popularity"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Adult         bool    `json:"adult"`
	GenreIDs      []int   `json:"genre_ids"`
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ProductionCountry struct {
	Iso3166_1 string `json:"iso_3166_1"`
	Name      string `json:"name"`
}

type MovieDetails struct {
	ID                  int                 `json:"id"`
	Title               string              `json:"title"`
	OriginalTitle       string              `json:"original_title"`
	Overview            string              `json:"overview"`
	ReleaseDate         string              `json:"release_date"`
	PosterPath          string              `json:"poster_path"`
	Popularity          float64             `json:"popularity"`
	VoteAverage         float64             `json:"vote_average"`
	VoteCount           int                 `json:"vote_count"`
	Adult               bool                `json:"adult"`
	Genres              []Genre             `json:"genres"`
	OriginalLanguage    string              `json:"original_language"`
	ProductionCountries []ProductionCountry `json:"production_countries"`
	Runtime             int                 `json:"runtime"`
	Tagline             string              `json:"tagline,omitempty"`
	Status              string              `json:"status,omitempty"`
	Source              string              `json:"source,omitempty"` // 'watched' or 'watchlist'
	Year                int                 `json:"year,omitempty"`
	LetterboxdURI       string              `json:"letterboxd_uri,omitempty"`
}

type MovieSearchResponse struct {
	Page         int     `json:"page"`
	Results      []Movie `json:"results"`
	TotalPages   int     `json:"total_pages"`
	TotalResults int     `json:"total_results"`
}

type ErrorResponse struct {
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
	Success       *bool  `json:"success,omitempty"`
}

type MovieEntry struct {
	Date          string
	Name          string
	Year          int
	LetterboxdURI string
	Source        string // 'watched' or 'watchlist'
}

func init() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("⚠️ Warning: No .env file found")
	}

	apiKey = os.Getenv(apiKeyEnv)
	if apiKey == "" {
		fmt.Printf("=============================================================\n")
		fmt.Printf("❌ ERROR: API Key not found. Set %s environment variable\n", apiKeyEnv)
		fmt.Printf("=============================================================\n")
		os.Exit(1)
	}
}

func makeTmdbRequest(endpoint string, queryParams map[string]string, target interface{}) error {
	// Wait for rate limiter
	<-rateLimiter.C

	base, _ := url.Parse(baseURL)
	endpointURL, _ := base.Parse(endpoint)

	params := url.Values{}
	params.Add("api_key", apiKey)
	for key, value := range queryParams {
		params.Add(key, value)
	}
	endpointURL.RawQuery = params.Encode()

	req, _ := http.NewRequest("GET", endpointURL.String(), nil)
	req.Header.Add("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		// Handle rate limiting - wait and retry
		retryAfter := 1 // Default to 1 second if header not present
		if s := resp.Header.Get("Retry-After"); s != "" {
			if i, err := strconv.Atoi(s); err == nil {
				retryAfter = i
			}
		}
		fmt.Printf("Rate limit exceeded. Waiting %d seconds before retry...\n", retryAfter)
		time.Sleep(time.Duration(retryAfter) * time.Second)
		return makeTmdbRequest(endpoint, queryParams, target) // Retry the request
	}

	if resp.StatusCode != http.StatusOK {
		var apiError ErrorResponse
		json.NewDecoder(resp.Body).Decode(&apiError)
		if apiError.StatusMessage != "" {
			return fmt.Errorf("API error: %s (Code: %d)", apiError.StatusMessage, apiError.StatusCode)
		}
		return fmt.Errorf("unexpected status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func readCSVFile(filePath string, source string) ([]MovieEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	_, err = reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header from %s: %w", filePath, err)
	}

	var entries []MovieEntry
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV record: %w", err)
		}

		if len(record) < 4 {
			continue // Skip incomplete records
		}

		year, _ := strconv.Atoi(record[2])
		entry := MovieEntry{
			Date:          record[0],
			Name:          record[1],
			Year:          year,
			LetterboxdURI: record[3],
			Source:        source,
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func loadExistingMovies() (map[string]MovieDetails, error) {
	existingMovies := make(map[string]MovieDetails)

	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		// File doesn't exist, return empty map
		return existingMovies, nil
	}

	file, err := os.Open(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open existing output file: %w", err)
	}
	defer file.Close()

	var movies []MovieDetails
	if err := json.NewDecoder(file).Decode(&movies); err != nil {
		// If the file is empty or invalid JSON, return empty map
		if err == io.EOF {
			return existingMovies, nil
		}
		return nil, fmt.Errorf("failed to parse existing JSON: %w", err)
	}

	// Create a map for quick lookup
	for _, movie := range movies {
		key := fmt.Sprintf("%s_%d", movie.Title, movie.Year)
		existingMovies[key] = movie
	}

	return existingMovies, nil
}

func searchMovie(title string, year int) (int, error) {
	searchParams := map[string]string{
		"query":         title,
		"language":      "en-US",
		"include_adult": "false",
		"year":          strconv.Itoa(year),
	}

	var searchResults MovieSearchResponse
	if err := makeTmdbRequest("search/movie", searchParams, &searchResults); err != nil {
		return 0, fmt.Errorf("search failed: %w", err)
	}

	if len(searchResults.Results) == 0 {
		// Try again without year constraint
		delete(searchParams, "year")
		if err := makeTmdbRequest("search/movie", searchParams, &searchResults); err != nil {
			return 0, fmt.Errorf("secondary search failed: %w", err)
		}

		if len(searchResults.Results) == 0 {
			return 0, fmt.Errorf("no results found for %s (%d)", title, year)
		}
	}

	// Select best match (first result should be most relevant)
	return searchResults.Results[0].ID, nil
}

func getMovieDetails(id int, entry MovieEntry) (MovieDetails, error) {
	var details MovieDetails
	endpoint := fmt.Sprintf("movie/%d", id)
	params := map[string]string{
		"language": "en-US",
	}

	if err := makeTmdbRequest(endpoint, params, &details); err != nil {
		return details, fmt.Errorf("failed to fetch details: %w", err)
	}

	// Add letterboxd metadata
	details.Source = entry.Source
	details.Year = entry.Year
	details.LetterboxdURI = entry.LetterboxdURI

	return details, nil
}

func main() {
	if apiKey == "" {
		log.Fatal("API key not set")
	}

	// Load existing movies to avoid duplicate fetches
	existingMovies, err := loadExistingMovies()
	if err != nil {
		log.Fatalf("Error loading existing movies: %v", err)
	}
	log.Printf("Found %d existing movies in output.json", len(existingMovies))

	// Read watched and watchlist CSVs
	watched, err := readCSVFile("../stats/watched.csv", "watched")
	if err != nil {
		log.Fatalf("Error reading watched.csv: %v", err)
	}
	log.Printf("Read %d entries from watched.csv", len(watched))

	watchlist, err := readCSVFile("../stats/watchlist.csv", "watchlist")
	if err != nil {
		log.Fatalf("Error reading watchlist.csv: %v", err)
	}
	log.Printf("Read %d entries from watchlist.csv", len(watchlist))

	// Combine both lists
	allMovies := append(watched, watchlist...)
	log.Printf("Processing %d total movies", len(allMovies))

	// Process movies with rate limiting and concurrency control
	var (
		newMovies       []MovieDetails
		existingCount   int
		errorCount      int
		processingMutex sync.Mutex
		wg              sync.WaitGroup
		semaphore       = make(chan struct{}, maxConcurrentReqs)
	)

	for _, entry := range allMovies {
		// Check if already exists
		key := fmt.Sprintf("%s_%d", entry.Name, entry.Year)
		if movie, exists := existingMovies[key]; exists {
			log.Printf("Movie already in database: %s (%d)", entry.Name, entry.Year)
			existingCount++
			newMovies = append(newMovies, movie)
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(entry MovieEntry) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			log.Printf("Searching for: %s (%d)", entry.Name, entry.Year)

			movieID, err := searchMovie(entry.Name, entry.Year)
			if err != nil {
				log.Printf("Error searching for %s (%d): %v", entry.Name, entry.Year, err)
				processingMutex.Lock()
				errorCount++
				processingMutex.Unlock()
				return
			}

			details, err := getMovieDetails(movieID, entry)
			if err != nil {
				log.Printf("Error getting details for %s (ID: %d): %v", entry.Name, movieID, err)
				processingMutex.Lock()
				errorCount++
				processingMutex.Unlock()
				return
			}

			processingMutex.Lock()
			newMovies = append(newMovies, details)
			processingMutex.Unlock()

			log.Printf("Successfully processed: %s (%d)", entry.Name, entry.Year)
		}(entry)
	}

	wg.Wait()
	rateLimiter.Stop()

	log.Printf("Processing complete: %d existing, %d new, %d errors",
		existingCount, len(newMovies)-existingCount, errorCount)

	// Ensure output directory exists
	outputDir := filepath.Dir(outputFile)
	if outputDir != "." && outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
	}

	// Write to output.json
	file, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(newMovies); err != nil {
		log.Fatalf("Failed to write JSON: %v", err)
	}

	fmt.Printf("Successfully saved data for %d movies to %s\n", len(newMovies), outputFile)
}
