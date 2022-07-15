package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/o-mago/spotify-status/src/app"
	"github.com/o-mago/spotify-status/src/repositories/db_entities"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type App struct {
	Db *gorm.DB
}

func main() {
	// Get environment variables
	newRelicAppName := os.Getenv("SPOTIFY_SLACK_APP_NEW_RELIC_APP_NAME")
	newRelicLicense := os.Getenv("SPOTIFY_SLACK_APP_NEW_RELIC_LICENSE")
	databaseURL := os.Getenv("SPOTIFY_SLACK_APP_DATABASE_URL")
	slackAuthURL := os.Getenv("SPOTIFY_SLACK_APP_SLACK_AUTH_URL")
	spotifyRedirectURL := os.Getenv("SPOTIFY_SLACK_APP_SPOTIFY_REDIRECT_URL")
	slackClientID := os.Getenv("SPOTIFY_SLACK_APP_SLACK_CLIENT_ID")
	slackClientSecret := os.Getenv("SPOTIFY_SLACK_APP_SLACK_CLIENT_SECRET")
	port := os.Getenv("PORT")

	// Setup New Relic
	newRelicApp, err := newrelic.NewApplication(
		newrelic.ConfigAppName(newRelicAppName),
		newrelic.ConfigLicense(newRelicLicense),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	// Setup connection to the database
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&db_entities.User{})

	// Setup app
	app := app.NewApp(db, slackAuthURL, spotifyRedirectURL, slackClientID, slackClientSecret)

	// Add handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", app.Handlers.CompleteAuthHandler)
	mux.HandleFunc("/slackAuth", app.Handlers.SlackAddHandler)
	mux.HandleFunc(newrelic.WrapHandleFunc(newRelicApp, "/users", app.Handlers.HealthHandler))
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)

	http.ListenAndServe(":"+port, mux)
}
