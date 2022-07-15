package app_error

import "net/http"

type appError struct {
	tag    string
	status int
}

func newAppError(tag string, status int) appError {
	return appError{
		tag,
		status,
	}
}

func (e appError) Error() string {
	return e.tag
}

func (e appError) Status() int {
	return e.status
}

var InvalidSpotifyAuthCode = newAppError("INVALID_SPOTIFY_TOKEN", http.StatusForbidden)
var InvalidCookie = newAppError("INVALID_COOKIE", http.StatusForbidden)
var SlackAuthBadRequest = newAppError("SLACK_AUTH_BAD_REQUEST", http.StatusBadRequest)
var UserNotFound = newAppError("USER_NOT_FOUND", http.StatusNotFound)
var UserAlreadyExists = newAppError("USER_ALREADY_EXISTS", http.StatusConflict)
