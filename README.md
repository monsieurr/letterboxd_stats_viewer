## Stack
- Go
- SQLite
- HTML / CSS 
- Javascript

## Fonctionnement de base
Récupération des informations via les fichiers .csv extraits manuellement depuis le profil Letterboxd + croisement avec informations disponibles sur TMDB.
Le serveur web (Go) permet de servir les informations qui sont stockées dans une base de données (SQLite).

## Améliorations à venir
- modification possible du thème (toggle à minima) -> mais ce serait cool de pouvoir choisir les 3, 4 couleurs de thèmes et garder ça quelque part en cache ou sauvegarde locale? => OK
- ajouter une petite flèche au niveau du tableau pour la partie filtre et effet au survol d'une colonne du tableau (au niveau du header) => OK


## Améliorations possibles
- récupération des données des films sur une DB type TMBD avec toutes les informations et croiser avec ce qu'on a pour avoir des stats plus précises (exemple : Combien de films
"d'action vus"? quel est le genre de film le plus vu? Par année? Durée moyenne d'un film?) => En cours
- ajout d'une carte avec les pays pour voir les pays d'où viennent les différents films vus => En cours


## Faire tourner l'application
1. Télécharger l'export sur Letterboxd
2. Mettre le dossier exporté dans la racine du projet
3. Renommer le dossier exporté "stats"
4. Récupérer sa clé API sur TMDB
5. Mettre la clé API dans un fichier .env à l'intérieur du dossier tmdb 
5. Faire tourner tmdb_call.go -> récupération des données via l'API TMDB (TMDB_API_KEY=votreclefAPI)
6. Faire tourner main.go (go run main.go) -> nécessite d'installer GO

Lors du premier run, la base de donnée va être créée.


## Gestion de la BDD
```
rm movies.db
go run main.go
```
