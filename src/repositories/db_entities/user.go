package db_entities

import (
	"time"

	"github.com/o-mago/spotify-status/src/domain"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	ID                  string    `gorm:"column:id;primaryKey"`
	SlackUserID         string    `gorm:"column:slack_user_id"`
	SlackAccessToken    string    `gorm:"column:slack_access_token"`
	SpotifyAccessToken  string    `gorm:"column:spotify_access_token"`
	SpotifyRefreshToken string    `gorm:"column:spotify_refresh_token"`
	SpotifyExpiry       time.Time `gorm:"column:slack_expiry"`
	SpotifyTokenType    string    `gorm:"column:spotify_token_type"`
}

func (user User) ToDomain() domain.User {
	return domain.User{
		ID:                  user.ID,
		SlackUserID:         user.SlackUserID,
		SlackAccessToken:    user.SlackAccessToken,
		SpotifyAccessToken:  user.SpotifyAccessToken,
		SpotifyRefreshToken: user.SpotifyRefreshToken,
		SpotifyExpiry:       user.SpotifyExpiry,
		SpotifyTokenType:    user.SpotifyTokenType,
	}
}

func NewUserFromDomain(user domain.User) User {
	return User{
		ID:                  user.ID,
		SlackUserID:         user.SlackUserID,
		SlackAccessToken:    user.SlackAccessToken,
		SpotifyAccessToken:  user.SpotifyAccessToken,
		SpotifyRefreshToken: user.SpotifyRefreshToken,
		SpotifyExpiry:       user.SpotifyExpiry,
		SpotifyTokenType:    user.SpotifyTokenType,
	}
}

type Users []User

func (u Users) ToDomain() []domain.User {
	a := make([]domain.User, len(u))
	for i := range u {
		a[i] = u[i].ToDomain()
	}
	return a
}
