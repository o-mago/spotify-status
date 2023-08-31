package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/o-mago/spotify-status/src/crypto"
	"github.com/o-mago/spotify-status/src/domain"
	"github.com/o-mago/spotify-status/src/repositories"
	"github.com/slack-go/slack"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

type services struct {
	repositories         repositories.Repositories
	spotifyAuthenticator spotify.Authenticator
	crypto               crypto.Crypto
}

type Services interface {
	AddUser(ctx context.Context, user domain.User) error
	ChangeUserStatus(ctx context.Context) error
}

func NewServices(repositories repositories.Repositories, spotifyAuthenticator spotify.Authenticator, crypto crypto.Crypto) Services {
	return services{
		repositories,
		spotifyAuthenticator,
		crypto,
	}
}

func (s services) AddUser(ctx context.Context, user domain.User) error {
	user.ID = uuid.New().String()

	encSpotifyAccessToken, err := s.crypto.Encrypt(user.SpotifyAccessToken)
	if err != nil {
		fmt.Printf("encSpotifyAccessToken: %s\n", err)
		return err
	}

	encSpotifyRefreshToken, err := s.crypto.Encrypt(user.SpotifyRefreshToken)
	if err != nil {
		fmt.Printf("encSpotifyRefreshToken: %s\n", err)
		return err
	}

	encSlackAccessToken, err := s.crypto.Encrypt(user.SlackAccessToken)
	if err != nil {
		fmt.Printf("encSlackAccessToken: %s\n", err)
		return err
	}

	user.SlackAccessToken = encSlackAccessToken
	user.SpotifyAccessToken = encSpotifyAccessToken
	user.SpotifyRefreshToken = encSpotifyRefreshToken

	return s.repositories.CreateUser(ctx, user)
}

func (s services) ChangeUserStatus(ctx context.Context) error {
	users, err := s.repositories.SearchUsers(ctx)
	if err != nil {
		fmt.Printf("SearchUsers: %s\n", err)
		return err
	}

	for _, user := range users {
		go func(user domain.User) {
			decSpotifyAccessToken, err := s.crypto.Decrypt(user.SpotifyAccessToken)
			if err != nil {
				fmt.Printf("decSpotifyAccessToken: %s\n", err)
				return
			}

			decSpotifyRefreshToken, err := s.crypto.Decrypt(user.SpotifyRefreshToken)
			if err != nil {
				fmt.Printf("decSpotifyRefreshToken: %s\n", err)
				return
			}

			decSlackAccessToken, err := s.crypto.Decrypt(user.SlackAccessToken)
			if err != nil {
				fmt.Printf("decSlackAccessToken: %s\n", err)
				return
			}

			slackApi := slack.New(string(decSlackAccessToken))

			spotifyToken := oauth2.Token{
				AccessToken:  string(decSpotifyAccessToken),
				RefreshToken: string(decSpotifyRefreshToken),
				Expiry:       user.SpotifyExpiry,
				TokenType:    user.SpotifyTokenType,
			}
			spotifyApi := s.spotifyAuthenticator.NewClient(&spotifyToken)

			player, err := spotifyApi.PlayerCurrentlyPlaying()
			if err != nil {
				fmt.Printf("Error spotify currently playing: %s\n", err)
				return
			}

			if player == nil || player.Item == nil {
				fmt.Printf("player == nil || player.Item == nil: %s\n", err)
				return
			}

			profile, err := slackApi.GetUserProfile(&slack.GetUserProfileParameters{UserID: user.SlackUserID})
			if err != nil {
				fmt.Printf("Error slack get user profile: %s\n", err)
				return
			}

			canUpdateStatus := player.Playing && (profile.StatusEmoji == ":spotify:" || profile.StatusEmoji == "")
			canClearStatus := !player.Playing && profile.StatusEmoji == ":spotify:"
			if !canUpdateStatus && !canClearStatus {
				fmt.Printf("!canUpdateStatus && !canClearStatus: %s\n", err)
				return
			}

			if canUpdateStatus {
				songName := player.Item.Name
				slackStatus := songName + " - " + player.Item.Artists[0].Name
				if len(slackStatus) > 100 {
					extraChars := len(slackStatus) - 100 + 3
					songName = player.Item.Name[:len(player.Item.Name)-extraChars]
					slackStatus = songName + "... - " + player.Item.Artists[0].Name
				}

				err = slackApi.SetUserCustomStatusWithUser(user.SlackUserID, slackStatus, ":spotify:", 0)
				if err != nil {
					fmt.Printf("Error slack set user custom status: %s\n", err)
				}
				return
			}

			if canClearStatus {
				err = slackApi.SetUserCustomStatusWithUser(user.SlackUserID, "", "", 0)
				if err != nil {
					fmt.Printf("Error clear user status: %s\n", err)
				}
				return
			}
		}(user)
	}

	return nil
}
