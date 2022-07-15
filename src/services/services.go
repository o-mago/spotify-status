package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/o-mago/spotify-status/src/app_error"
	"github.com/o-mago/spotify-status/src/domain"
	"github.com/o-mago/spotify-status/src/repositories"
	"github.com/slack-go/slack"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

type services struct {
	repositories         repositories.Repositories
	spotifyAuthenticator spotify.Authenticator
}

type Services interface {
	AddUser(ctx context.Context, slackUserID, slackAccessToken string, spotifyToken *oauth2.Token) error
	ChangeUserStatus(ctx context.Context) error
}

func NewServices(repositories repositories.Repositories, spotifyAuthenticator spotify.Authenticator) Services {
	return services{
		repositories,
		spotifyAuthenticator,
	}
}

func (s services) AddUser(ctx context.Context, slackUserID, slackAccessToken string, spotifyToken *oauth2.Token) error {
	_, err := s.repositories.GetUserBySlackUserID(ctx, slackUserID)
	if err != nil {
		if err == app_error.UserNotFound {
			user := domain.User{
				ID:                  uuid.New().String(),
				SlackUserID:         slackUserID,
				SlackAccessToken:    slackAccessToken,
				SpotifyAccessToken:  spotifyToken.AccessToken,
				SpotifyRefreshToken: spotifyToken.RefreshToken,
				SpotifyExpiry:       spotifyToken.Expiry,
				SpotifyTokenType:    spotifyToken.TokenType,
			}
			_, err := s.repositories.CreateUser(ctx, user)
			return err
		}
		return err
	}
	return app_error.UserAlreadyExists
}

func (s services) ChangeUserStatus(ctx context.Context) error {
	users, err := s.repositories.SearchUsers(ctx)
	if err != nil {
		return err
	}

	for _, user := range users {
		go func(user domain.User) {
			slackApi := slack.New(user.SlackAccessToken)

			spotifyApi := s.spotifyAuthenticator.NewClient(new(oauth2.Token))

			player, err := spotifyApi.PlayerCurrentlyPlaying()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				return
			}

			if player == nil || player.Item == nil {
				return
			}

			profile, err := slackApi.GetUserProfile(&slack.GetUserProfileParameters{UserID: user.SlackUserID})
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				return
			}

			canChangeStatus := profile.StatusEmoji == ":spotify:" || profile.StatusEmoji == ""
			if !canChangeStatus {
				return
			}

			if player.Playing && canChangeStatus {
				songName := player.Item.Name
				slackStatus := songName + " - " + player.Item.Artists[0].Name
				if len(slackStatus) > 100 {
					extraChars := len(slackStatus) - 100 + 3
					songName = player.Item.Name[:len(player.Item.Name)-extraChars]
					slackStatus = songName + "... - " + player.Item.Artists[0].Name
				}
				err = slackApi.SetUserCustomStatusWithUser(user.SlackUserID, slackStatus, ":spotify:", 0)

				if err != nil {
					fmt.Printf("Error: %s\n", err)
					return
				}
			} else if profile.StatusEmoji == ":spotify:" {
				err = slackApi.SetUserCustomStatusWithUser(user.SlackUserID, "", "", 0)

				if err != nil {
					fmt.Printf("Error: %s\n", err)
					return
				}
			}
		}(user)
	}

	return nil
}