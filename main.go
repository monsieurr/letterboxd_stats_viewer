package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type Statistics struct {
	AverageRuntime         float64       `json:"average_runtime"`
	TopProductionCountries []CountryStat `json:"top_production_countries"`
}

type CountryStat struct {
	Country string `json:"country"`
	Count   int    `json:"count"`
}

// ProductionCountry représente un pays de production issu du JSON.
type ProductionCountry struct {
	ISO3166_1 string `json:"iso_3166_1"`
	Name      string `json:"name"`
}

// Movie représente la structure d'un film.
type Movie struct {
	LetterboxdURI            string  `json:"letterboxd_uri" db:"letterboxd_uri"` // Utilisé comme identifiant global
	Title                    string  `json:"title" db:"title"`
	OriginalTitle            string  `json:"original_title" db:"original_title"`
	Overview                 string  `json:"overview" db:"overview"`
	ReleaseDate              string  `json:"release_date" db:"release_date"`
	PosterPath               string  `json:"poster_path" db:"poster_path"`
	Popularity               float64 `json:"popularity" db:"popularity"`
	VoteAverage              float64 `json:"vote_average" db:"vote_average"`
	VoteCount                int     `json:"vote_count" db:"vote_count"`
	Adult                    bool    `json:"adult" db:"adult"`
	OriginalLanguage         string  `json:"original_language" db:"original_language"`
	Runtime                  int     `json:"runtime" db:"runtime"`
	Tagline                  string  `json:"tagline" db:"tagline"`
	Status                   string  `json:"status" db:"status"`
	Source                   string  `json:"source" db:"source"`
	Year                     int     `json:"year" db:"year"`
	MainProductionCountry    string  `json:"main_production_country" db:"main_production_country"`
	OtherProductionCountries string  `json:"other_production_countries" db:"other_production_countries"`
	// Champ temporaire pour l'import JSON
	ProductionCountries []ProductionCountry `json:"production_countries" db:"-"`
}

// Watched représente un film visionné
type Watched struct {
	ID            int    `db:"id"`
	LetterboxdURI string `db:"letterboxd_uri"`
	WatchedDate   string `db:"watched_date"`
}

// Watchlist représente un film dans la liste de films à voir
type Watchlist struct {
	ID            int    `db:"id"`
	LetterboxdURI string `db:"letterboxd_uri"`
	AddedDate     string `db:"added_date"`
}

// Review représente une critique de film
type Review struct {
	ID            int     `db:"id"`
	LetterboxdURI string  `db:"letterboxd_uri"`
	ReviewDate    string  `db:"review_date"`
	Rating        float64 `db:"rating"`
	Rewatch       bool    `db:"rewatch"`
	ReviewText    string  `db:"review"`
	Tags          string  `db:"tags"`
	WatchedDate   string  `db:"watched_date"`
}

// Rating représente une notation de film
type Rating struct {
	ID            int     `db:"id"`
	LetterboxdURI string  `db:"letterboxd_uri"`
	RatingDate    string  `db:"rating_date"`
	Rating        float64 `db:"rating"`
}

// Comment représente un commentaire sur un film
type Comment struct {
	ID            int    `db:"id"`
	LetterboxdURI string `db:"letterboxd_uri"`
	CommentDate   string `db:"comment_date"`
	CommentText   string `db:"comment"`
}

// db est la variable globale pour la base SQLite.
var db *sqlx.DB

func main() {
	var err error
	// Ouvrir (ou créer) la base SQLite avec SQLx
	db, err = sqlx.Open("sqlite3", "./movies.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Création des tables avec la nouvelle structure
	if err := createTables(db); err != nil {
		log.Fatal(err)
	}

	// Importation des fichiers CSV et JSON depuis le dossier "stats"
	if err := importCSV(db, filepath.Join("stats", "watched.csv"), "watched"); err != nil {
		log.Println("Erreur import CSV watched:", err)
	}
	if err := importCSV(db, filepath.Join("stats", "watchlist.csv"), "watchlist"); err != nil {
		log.Println("Erreur import CSV watchlist:", err)
	}
	if err := importCSV(db, filepath.Join("stats", "reviews.csv"), "reviews"); err != nil {
		log.Println("Erreur import CSV reviews:", err)
	}
	if err := importCSV(db, filepath.Join("stats", "ratings.csv"), "ratings"); err != nil {
		log.Println("Erreur import CSV ratings:", err)
	}
	if err := importCSV(db, filepath.Join("stats", "comments.csv"), "comments"); err != nil {
		log.Println("Erreur import CSV comments:", err)
	}
	if err := importJSON(db, filepath.Join("stats", "output.json")); err != nil {
		log.Println("Erreur import JSON:", err)
	}

	// Servir les fichiers statiques
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// Endpoint existant pour lire les CSV directement
	http.HandleFunc("/api/data", dataHandler)
	// Nouvel endpoint qui renvoie les films depuis la base SQLite
	http.HandleFunc("/api/movies", moviesHandler)

	http.HandleFunc("/api/statistics", statisticsHandler)

	log.Println("Serveur démarré sur http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// createTables crée la table movies avec Letterboxd URI comme clé primaire et les colonnes de production.
func createTables(db *sqlx.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS movies (
			letterboxd_uri TEXT PRIMARY KEY,
			title TEXT,
			original_title TEXT,
			overview TEXT,
			release_date TEXT,
			poster_path TEXT,
			popularity REAL,
			vote_average REAL,
			vote_count INTEGER,
			adult BOOLEAN,
			original_language TEXT,
			runtime INTEGER,
			tagline TEXT,
			status TEXT,
			source TEXT,
			year INTEGER,
			main_production_country TEXT,
			other_production_countries TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS watched (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			letterboxd_uri TEXT,
			watched_date TEXT,
			FOREIGN KEY(letterboxd_uri) REFERENCES movies(letterboxd_uri)
		);`,
		`CREATE TABLE IF NOT EXISTS watchlist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			letterboxd_uri TEXT,
			added_date TEXT,
			FOREIGN KEY(letterboxd_uri) REFERENCES movies(letterboxd_uri)
		);`,
		`CREATE TABLE IF NOT EXISTS reviews (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			letterboxd_uri TEXT,
			review_date TEXT,
			rating REAL,
			rewatch BOOLEAN,
			review TEXT,
			tags TEXT,
			watched_date TEXT,
			FOREIGN KEY(letterboxd_uri) REFERENCES movies(letterboxd_uri)
		);`,
		`CREATE TABLE IF NOT EXISTS ratings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			letterboxd_uri TEXT,
			rating_date TEXT,
			rating REAL,
			FOREIGN KEY(letterboxd_uri) REFERENCES movies(letterboxd_uri)
		);`,
		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			letterboxd_uri TEXT,
			comment_date TEXT,
			comment TEXT,
			FOREIGN KEY(letterboxd_uri) REFERENCES movies(letterboxd_uri)
		);`,
	}
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

// getOrCreateMovie recherche un film par son Letterboxd URI et l'insère s'il n'existe pas.
// In the getOrCreateMovie function
func getOrCreateMovie(db *sqlx.DB, title string, year int, letterboxdURI string) error {
	// Recherche dans la table movies
	var exists bool
	err := db.Get(&exists, "SELECT 1 FROM movies WHERE letterboxd_uri = ? LIMIT 1", letterboxdURI)
	if err != nil {
		// If the film doesn't exist
		if err == sql.ErrNoRows { // Change from sqlx.ErrNoRows to sql.ErrNoRows
			// Insertion d'un film avec les infos minimales
			_, err := db.Exec(`INSERT INTO movies (letterboxd_uri, title, year) VALUES (?, ?, ?)`,
				letterboxdURI, title, year)
			return err
		}
		return err
	}
	return nil
}

// importCSV lit un fichier CSV et insère les données dans la table appropriée.
func importCSV(db *sqlx.DB, filename, source string) error {
	f, err := os.Open(filename)
	if err != nil {
		log.Printf("Erreur lors de l'ouverture du fichier %s: %v", filename, err)
		return err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return err
	}
	if len(records) < 2 {
		return nil // Pas de données
	}

	header := records[0]
	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[col] = i
	}

	for _, row := range records[1:] {
		date := row[colIdx["Date"]]
		name := row[colIdx["Name"]]
		year, _ := strconv.Atoi(row[colIdx["Year"]])
		letterboxdURI := row[colIdx["Letterboxd URI"]]

		if err := getOrCreateMovie(db, name, year, letterboxdURI); err != nil {
			log.Printf("Erreur lors de la récupération/création du film %s: %v", name, err)
			continue
		}

		switch source {
		case "watched":
			watched := Watched{
				LetterboxdURI: letterboxdURI,
				WatchedDate:   date,
			}
			_, err = db.NamedExec("INSERT INTO watched (letterboxd_uri, watched_date) VALUES (:letterboxd_uri, :watched_date)", watched)
		case "watchlist":
			watchlist := Watchlist{
				LetterboxdURI: letterboxdURI,
				AddedDate:     date,
			}
			_, err = db.NamedExec("INSERT INTO watchlist (letterboxd_uri, added_date) VALUES (:letterboxd_uri, :added_date)", watchlist)
		case "reviews":
			rating, _ := strconv.ParseFloat(row[colIdx["Rating"]], 64)
			rewatch := false
			if val := row[colIdx["Rewatch"]]; strings.TrimSpace(val) != "" {
				rewatch = true
			}
			reviewText := row[colIdx["Review"]]
			tags := row[colIdx["Tags"]]
			watchedDate := ""
			if idx, ok := colIdx["Watched Date"]; ok {
				watchedDate = row[idx]
			}

			review := Review{
				LetterboxdURI: letterboxdURI,
				ReviewDate:    date,
				Rating:        rating,
				Rewatch:       rewatch,
				ReviewText:    reviewText,
				Tags:          tags,
				WatchedDate:   watchedDate,
			}
			_, err = db.NamedExec(`INSERT INTO reviews 
				(letterboxd_uri, review_date, rating, rewatch, review, tags, watched_date)
				VALUES (:letterboxd_uri, :review_date, :rating, :rewatch, :review, :tags, :watched_date)`, review)
		case "ratings":
			rating, _ := strconv.ParseFloat(row[colIdx["Rating"]], 64)
			ratingObj := Rating{
				LetterboxdURI: letterboxdURI,
				RatingDate:    date,
				Rating:        rating,
			}
			_, err = db.NamedExec("INSERT INTO ratings (letterboxd_uri, rating_date, rating) VALUES (:letterboxd_uri, :rating_date, :rating)", ratingObj)
		case "comments":
			commentText := row[colIdx["Comment"]]
			comment := Comment{
				LetterboxdURI: letterboxdURI,
				CommentDate:   date,
				CommentText:   commentText,
			}
			_, err = db.NamedExec("INSERT INTO comments (letterboxd_uri, comment_date, comment) VALUES (:letterboxd_uri, :comment_date, :comment)", comment)
		default:
			log.Printf("Source inconnue: %s", source)
		}

		if err != nil {
			log.Printf("Erreur lors de l'insertion dans %s: %v", source, err)
		}
	}
	return nil
}

// importJSON lit le fichier JSON et insère ou met à jour les films dans la base.
func importJSON(db *sqlx.DB, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		log.Printf("Erreur lors de l'ouverture du fichier %s: %v", filename, err)
		return err
	}
	defer f.Close()

	var movies []Movie
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&movies); err != nil {
		return err
	}

	for _, m := range movies {
		// Extraire le pays principal et les autres pays à partir du tableau ProductionCountries
		var mainCountry string
		var otherCountries []string
		for i, pc := range m.ProductionCountries {
			if i == 0 {
				mainCountry = pc.Name
			} else {
				otherCountries = append(otherCountries, pc.Name)
			}
		}
		m.MainProductionCountry = mainCountry
		m.OtherProductionCountries = strings.Join(otherCountries, ", ")

		// Insertion ou mise à jour du film dans la table movies avec NamedExec
		_, err := db.NamedExec(`INSERT OR REPLACE INTO movies 
			(letterboxd_uri, title, original_title, overview, release_date, poster_path, 
			popularity, vote_average, vote_count, adult, original_language, runtime, 
			tagline, status, source, year, main_production_country, other_production_countries)
			VALUES (:letterboxd_uri, :title, :original_title, :overview, :release_date, :poster_path, 
			:popularity, :vote_average, :vote_count, :adult, :original_language, :runtime, 
			:tagline, :status, :source, :year, :main_production_country, :other_production_countries)`, m)
		if err != nil {
			log.Printf("Erreur lors de l'insertion/mise à jour du film %s: %v", m.Title, err)
			continue
		}
	}
	return nil
}

// dataHandler reste inchangé et lit un fichier CSV dans le dossier "stats".
func dataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	fileType := r.URL.Query().Get("type")
	validTypes := map[string]bool{
		"watched":   true,
		"watchlist": true,
		"reviews":   true,
		"ratings":   true,
		"comments":  true,
	}

	if !validTypes[fileType] {
		http.Error(w, `{"error": "Type de fichier invalide"}`, http.StatusBadRequest)
		return
	}

	path := filepath.Join("stats", fileType+".csv")
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, `{"error": "Fichier non trouvé"}`, http.StatusNotFound)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		http.Error(w, `{"error": "Erreur de lecture CSV"}`, http.StatusInternalServerError)
		return
	}

	headers := records[0]
	var results []map[string]string
	for _, record := range records[1:] {
		item := make(map[string]string)
		for i, value := range record {
			item[headers[i]] = strings.TrimSpace(value)
		}
		results = append(results, item)
	}

	json.NewEncoder(w).Encode(results)
}

// moviesHandler renvoie les films stockés dans la base SQLite au format JSON.
func moviesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	var movies []Movie
	err := db.Select(&movies, `SELECT 
		letterboxd_uri, title, original_title, overview, release_date, poster_path, 
		popularity, vote_average, vote_count, adult, original_language, runtime, 
		tagline, status, source, year, main_production_country, other_production_countries 
		FROM movies`)
	if err != nil {
		http.Error(w, `{"error": "Erreur lors de la récupération des films"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(movies)
}

// Add this function after the moviesHandler function
func statisticsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	var stats Statistics

	// Get average runtime of watched movies
	err := db.Get(&stats.AverageRuntime, `
        SELECT AVG(m.runtime) 
        FROM movies m
        JOIN watched w ON m.letterboxd_uri = w.letterboxd_uri
        WHERE m.runtime > 0
    `)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Erreur lors du calcul du runtime moyen: %s"}`, err), http.StatusInternalServerError)
		return
	}

	// Get top 3 production countries
	var countries []CountryStat
	err = db.Select(&countries, `
        SELECT main_production_country as country, COUNT(*) as count
        FROM movies
        WHERE main_production_country != ""
        GROUP BY main_production_country
        ORDER BY count DESC
        LIMIT 3
    `)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Erreur lors de la récupération des pays: %s"}`, err), http.StatusInternalServerError)
		return
	}

	stats.TopProductionCountries = countries

	json.NewEncoder(w).Encode(stats)
}
