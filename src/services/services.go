package services

import (
	"context"

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
	RemoveUserBySlackID(ctx context.Context, slackID string) error
	UpdateUserEnabledBySlackID(ctx context.Context, user domain.User) error
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
		return err
	}

	encSpotifyRefreshToken, err := s.crypto.Encrypt(user.SpotifyRefreshToken)
	if err != nil {
		return err
	}

	encSlackAccessToken, err := s.crypto.Encrypt(user.SlackAccessToken)
	if err != nil {
		return err
	}

	user.SlackAccessToken = encSlackAccessToken
	user.SpotifyAccessToken = encSpotifyAccessToken
	user.SpotifyRefreshToken = encSpotifyRefreshToken

	return s.repositories.CreateUser(ctx, user)
}

func (s services) RemoveUserBySlackID(ctx context.Context, id string) error {
	return s.repositories.RemoveUserBySlackID(ctx, id)
}

func (s services) UpdateUserEnabledBySlackID(ctx context.Context, user domain.User) error {
	return s.repositories.UpdateUserEnabledBySlackID(ctx, user)
}

func (s services) ChangeUserStatus(ctx context.Context) error {
	users, err := s.repositories.SearchUsers(ctx)
	if err != nil {
		return err
	}

	for _, user := range users {
		go func(user domain.User) {
			decSpotifyAccessToken, err := s.crypto.Decrypt(user.SpotifyAccessToken)
			if err != nil {
				return
			}

			decSpotifyRefreshToken, err := s.crypto.Decrypt(user.SpotifyRefreshToken)
			if err != nil {
				return
			}

			decSlackAccessToken, err := s.crypto.Decrypt(user.SlackAccessToken)
			if err != nil {
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
				return
			}

			if player == nil || player.Item == nil {
				return
			}

			profile, err := slackApi.GetUserProfile(&slack.GetUserProfileParameters{UserID: user.SlackUserID})
			if err != nil {
				return
			}

			canUpdateStatus := player.Playing && (profile.StatusEmoji == ":spotify:" || profile.StatusEmoji == "")
			canClearStatus := !player.Playing && profile.StatusEmoji == ":spotify:"
			if !canUpdateStatus && !canClearStatus {
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

				slackApi.SetUserCustomStatusWithUser(user.SlackUserID, slackStatus, ":spotify:", 0)

				return
			}

			if canClearStatus {
				slackApi.SetUserCustomStatusWithUser(user.SlackUserID, "", "", 0)

				return
			}
		}(user)
	}

	return nil
}
