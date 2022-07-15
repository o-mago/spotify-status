package app

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/o-mago/spotify-status/src/handlers"
	"github.com/o-mago/spotify-status/src/repositories"
	"github.com/o-mago/spotify-status/src/services"
	"github.com/robfig/cron/v3"
	"github.com/zmb3/spotify"
	"gorm.io/gorm"
)

type App struct {
	DB       *gorm.DB
	Handlers handlers.Handlers
}

func NewApp(db *gorm.DB, slackAuthURL, spotifyRedirectURL, slackClientID, slackClientSecret string) App {
	spotifyAuthenticator := spotify.NewAuthenticator(spotifyRedirectURL, spotify.ScopeUserReadCurrentlyPlaying)

	repositories := repositories.NewRepository(db)
	services := services.NewServices(repositories, spotifyAuthenticator)
	handlers := handlers.NewHandlers(services, spotifyAuthenticator, stateGenerator(), slackClientID, slackClientSecret, slackAuthURL)

	// Setup cronjob for updating status
	c := cron.New(cron.WithSeconds())
	c.AddFunc("@every 10s", func() { services.ChangeUserStatus(context.Background()) })
	c.Start()

	return App{
		DB:       db,
		Handlers: handlers,
	}
}

func stateGenerator() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
