package main

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Servir les fichiers statiques
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// Gérer les requêtes de données
	http.HandleFunc("/api/data", dataHandler)

	log.Println("Serveur démarré sur http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	// Gestion CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	// Validation du paramètre
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

	// Lire le fichier CSV
	path := filepath.Join("stats", fileType+".csv")
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, `{"error": "Fichier non trouvé"}`, http.StatusNotFound)
		return
	}
	defer file.Close()

	// Parser le CSV
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		http.Error(w, `{"error": "Erreur de lecture CSV"}`, http.StatusInternalServerError)
		return
	}

	// Convertir en JSON
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
