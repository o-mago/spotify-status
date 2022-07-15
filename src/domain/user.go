package domain

import "time"

type User struct {
	ID                  string
	SlackUserID         string
	SlackAccessToken    string
	SpotifyAccessToken  string
	SpotifyRefreshToken string
	SpotifyExpiry       time.Time
	SpotifyTokenType    string
}
