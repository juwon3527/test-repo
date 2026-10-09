package main

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed web/*
var webFiles embed.FS

type Match struct {
	Home      string `json:"home"`
	Away      string `json:"away"`
	HomeCode  string `json:"homeCode"`
	AwayCode  string `json:"awayCode"`
	HomeScore int    `json:"homeScore"`
	AwayScore int    `json:"awayScore"`
	Status    string `json:"status"`
	Venue     string `json:"venue"`
	Kickoff   string `json:"kickoff"`
}

type Fixture struct {
	Competition string `json:"competition"`
	Opponent    string `json:"opponent"`
	Code        string `json:"code"`
	Date        string `json:"date"`
	Time        string `json:"time"`
	Venue       string `json:"venue"`
}

type Article struct {
	Category string `json:"category"`
	Title    string `json:"title"`
	ReadTime string `json:"readTime"`
	Image    string `json:"image"`
}

type MatchCentre struct {
	Demo     bool      `json:"demo"`
	Match    Match     `json:"match"`
	Fixtures []Fixture `json:"fixtures"`
	News     []Article `json:"news"`
}

func matchCentre() MatchCentre {
	return MatchCentre{
		Demo: true,
		Match: Match{
			Home: "Chelsea", Away: "Arsenal", HomeCode: "CHE", AwayCode: "ARS",
			HomeScore: 2, AwayScore: 1, Status: "FULL TIME", Venue: "Stamford Bridge", Kickoff: "Saturday, 3:00 PM",
		},
		Fixtures: []Fixture{
			{Competition: "Premier League", Opponent: "Newcastle United", Code: "NEW", Date: "Sat, 18 Oct", Time: "17:30", Venue: "Home"},
			{Competition: "UEFA Champions League", Opponent: "Ajax", Code: "AJA", Date: "Wed, 22 Oct", Time: "20:00", Venue: "Away"},
			{Competition: "Premier League", Opponent: "Tottenham Hotspur", Code: "TOT", Date: "Sun, 26 Oct", Time: "16:30", Venue: "Away"},
		},
		News: []Article{
			{Category: "MATCH REPORT", Title: "Blue belief carries Chelsea over the line", ReadTime: "4 MIN READ", Image: "https://images.unsplash.com/photo-1522778119026-d647f0596c20?auto=format&fit=crop&w=1100&q=85"},
			{Category: "THE VIEW", Title: "A night at the Bridge, told from the stands", ReadTime: "6 MIN READ", Image: "https://images.unsplash.com/photo-1517466787929-bc90951d0974?auto=format&fit=crop&w=900&q=85"},
			{Category: "CULTURE", Title: "Why west London always sings its own song", ReadTime: "5 MIN READ", Image: "https://images.unsplash.com/photo-1517466787929-bc90951d0974?auto=format&fit=crop&w=900&q=85"},
		},
	}
}

// App holds the server's dependencies.
type App struct {
	store  Store
	images ImageStore // nil when S3 isn't configured; uploads are then disabled
}

// newAppFromEnv connects to MySQL (MYSQL_DSN) and S3 (S3_BUCKET). Either may
// be left unset for local development.
func newAppFromEnv(ctx context.Context) (*App, error) {
	app := &App{}
	if dsn := os.Getenv("MYSQL_DSN"); dsn != "" {
		store, err := openMySQL(ctx, dsn)
		if err != nil {
			return nil, err
		}
		app.store = store
	} else {
		log.Print("MYSQL_DSN is not set: using an in-memory store, so accounts and photos are lost on restart")
		app.store = newMemoryStore()
	}
	if bucket := os.Getenv("S3_BUCKET"); bucket != "" {
		images, err := newS3Images(ctx, bucket, os.Getenv("S3_ENDPOINT"), os.Getenv("S3_PUBLIC_URL"))
		if err != nil {
			return nil, err
		}
		app.images = images
	} else {
		log.Print("S3_BUCKET is not set: image uploads are disabled")
	}
	return app, nil
}

func newServer(app *App) http.Handler {
	content, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/match-centre", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(matchCentre()); err != nil {
			http.Error(w, "could not encode match centre", http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("POST /api/signup", app.handleSignup)
	mux.HandleFunc("POST /api/login", app.handleLogin)
	mux.HandleFunc("POST /api/logout", app.handleLogout)
	mux.HandleFunc("GET /api/session", app.handleSession)
	mux.HandleFunc("POST /api/account/avatar", app.handleAvatarUpload)
	mux.HandleFunc("GET /api/gallery", app.handleGalleryList)
	mux.HandleFunc("POST /api/gallery", app.handleGalleryUpload)
	mux.HandleFunc("GET /media/{key...}", app.handleMedia)
	mux.Handle("GET /", http.FileServer(http.FS(content)))
	return securityHeaders(sameOrigin(mux))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func main() {
	app, err := newAppFromEnv(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              ":8080",
		Handler:           newServer(app),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("The Bridge is running at http://localhost%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
