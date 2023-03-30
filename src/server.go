package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/o-mago/spotify-status/src/handlers"
	"github.com/o-mago/spotify-status/src/repositories"
	"github.com/o-mago/spotify-status/src/repositories/db_entities"
	"github.com/o-mago/spotify-status/src/services"
	"github.com/robfig/cron/v3"
	"github.com/zmb3/spotify"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	// Get environment variables
	newRelicAppName := os.Getenv("SPOTIFY_SLACK_APP_NEW_RELIC_APP_NAME")
	newRelicLicense := os.Getenv("SPOTIFY_SLACK_APP_NEW_RELIC_LICENSE")
	databaseURL := os.Getenv("SPOTIFY_SLACK_APP_DATABASE_URL")
	slackAuthURL := os.Getenv("SPOTIFY_SLACK_APP_SLACK_AUTH_URL")
	spotifyRedirectURL := os.Getenv("SPOTIFY_SLACK_APP_SPOTIFY_REDIRECT_URL")
	slackClientID := os.Getenv("SPOTIFY_SLACK_APP_SLACK_CLIENT_ID")
	slackClientSecret := os.Getenv("SPOTIFY_SLACK_APP_SLACK_CLIENT_SECRET")
	spotifyClientID := os.Getenv("SPOTIFY_SLACK_APP_SPOTIFY_CLIENT_ID")
	spotifyClientSecret := os.Getenv("SPOTIFY_SLACK_APP_SPOTIFY_CLIENT_SECRET")
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

	// Creating Spotify Authenticator
	spotifyAuthenticator := spotify.NewAuthenticator(spotifyRedirectURL, spotify.ScopeUserReadCurrentlyPlaying)
	spotifyAuthenticator.SetAuthInfo(spotifyClientID, spotifyClientSecret)

	// Creating app layers (repositories, services, handlers)
	repositories := repositories.NewRepository(db)
	services := services.NewServices(repositories, spotifyAuthenticator)
	handlers := handlers.NewHandlers(services, spotifyAuthenticator, stateGenerator(), slackClientID, slackClientSecret, slackAuthURL)

	// Setup cronjob for updating status
	c := cron.New(cron.WithSeconds())
	c.AddFunc("@every 10s", func() { services.ChangeUserStatus(context.Background()) })
	c.Start()

	// Add handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", handlers.SpotifyCallbackHandler)
	mux.HandleFunc("/slackAuth", handlers.SlackCallbackHandler)
	mux.HandleFunc(newrelic.WrapHandleFunc(newRelicApp, "/users", handlers.HealthHandler))
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)

	srv := &http.Server{
		Addr:         ":" + port,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Println(err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	srv.Shutdown(ctx)

	fmt.Println("shutting down")
	os.Exit(0)
}

func stateGenerator() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
