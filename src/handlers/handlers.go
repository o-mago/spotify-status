package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/o-mago/spotify-status/src/app_error"
	"github.com/o-mago/spotify-status/src/domain"
	"github.com/o-mago/spotify-status/src/services"
	"github.com/zmb3/spotify"
)

type handlers struct {
	services             services.Services
	spotifyAuthenticator spotify.Authenticator
	spotifyState         string
	slackClientID        string
	slackClientSecret    string
	slackAuthURL         string
	slackSigningSecret   string
}

type Handlers interface {
	HealthHandler(w http.ResponseWriter, r *http.Request)
	SpotifyCallbackHandler(w http.ResponseWriter, r *http.Request)
	SlackCallbackHandler(w http.ResponseWriter, r *http.Request)
	OptInHandler(w http.ResponseWriter, r *http.Request)
	OptOutHandler(w http.ResponseWriter, r *http.Request)
	EnableHandler(w http.ResponseWriter, r *http.Request)
	DisableHandler(w http.ResponseWriter, r *http.Request)

	writeResponse(w http.ResponseWriter, resp interface{}, status int)
}

func NewHandlers(services services.Services, spotifyAuthenticator spotify.Authenticator,
	spotifyState, slackClientID, slackClientSecret, slackAuthURL, slackSigningSecret string) Handlers {
	return handlers{
		services,
		spotifyAuthenticator,
		spotifyState,
		slackClientID,
		slackClientSecret,
		slackAuthURL,
		slackSigningSecret,
	}
}

func (h handlers) SpotifyCallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := r.Cookie("user_id")
	if err != nil {
		appError := app_error.InvalidCookie
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}
	slackAccessToken, err := r.Cookie("slack_access_token")
	if err != nil {
		appError := app_error.InvalidCookie
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}

	spotifyToken, err := h.spotifyAuthenticator.Token(h.spotifyState, r)
	if err != nil {
		appError := app_error.InvalidSpotifyAuthCode
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}

	user := domain.User{
		SlackUserID:         userID.Value,
		SlackAccessToken:    slackAccessToken.Value,
		SpotifyAccessToken:  spotifyToken.AccessToken,
		SpotifyRefreshToken: spotifyToken.RefreshToken,
		SpotifyExpiry:       spotifyToken.Expiry,
		SpotifyTokenType:    spotifyToken.TokenType,
	}

	err = h.services.AddUser(ctx, user)
	if err != nil {
		appError := app_error.AddUserError
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}

	http.ServeFile(w, r, "./static/completed/index.html")
}

func (h handlers) SlackCallbackHandler(w http.ResponseWriter, r *http.Request) {
	slackCode := r.URL.Query().Get("code")

	requestBody := url.Values{}
	requestBody.Set("code", slackCode)
	requestBody.Set("client_id", h.slackClientID)
	requestBody.Set("client_secret", h.slackClientSecret)

	resp, err := http.Post(h.slackAuthURL, "application/x-www-form-urlencoded", strings.NewReader(requestBody.Encode()))
	if err != nil {
		appError := app_error.SlackAuthBadRequest
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		appError := app_error.SlackAuthBadRequest
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}

	var slackAuthResponse struct {
		Ok         bool   `json:"ok"`
		AppId      string `json:"app_id"`
		AuthedUser struct {
			Id          string `json:"id"`
			Scope       string `json:"scope"`
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
		} `json:"authed_user"`
		Team struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"team"`
		Enterprise string `json:"enterprise"`
	}
	err = json.Unmarshal(body, &slackAuthResponse)
	if err != nil {
		appError := app_error.SlackAuthBadRequest
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}

	expiration := time.Now().Add(1 * time.Hour)
	cookieUser := http.Cookie{Name: "user_id", Value: slackAuthResponse.AuthedUser.Id, Expires: expiration}
	cookieSlack := http.Cookie{Name: "slack_access_token", Value: slackAuthResponse.AuthedUser.AccessToken, Expires: expiration}
	http.SetCookie(w, &cookieUser)
	http.SetCookie(w, &cookieSlack)

	spotifyAuthURL := h.spotifyAuthenticator.AuthURL(h.spotifyState)

	http.Redirect(w, r, spotifyAuthURL, http.StatusSeeOther)
}

func (h handlers) OptInHandler(w http.ResponseWriter, r *http.Request) {
	err := h.verifySlackSignature(w, r)
	if err != nil {
		fmt.Println(err)
		h.writeResponse(w, "error", http.StatusBadRequest)

		return
	}

	h.writeResponse(w, "Please visit: https://slack.com/oauth/v2/authorize?client_id=1514600029252.1508440748514&scope=commands,chat:write&user_scope=users.profile:read,users.profile:write", http.StatusOK)
}

func (h handlers) OptOutHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := h.verifySlackSignature(w, r)
	if err != nil {
		fmt.Println(err)
		h.writeResponse(w, "error", http.StatusBadRequest)

		return
	}

	err = r.ParseForm()
	if err != nil {
		fmt.Println(err)

		return
	}

	slackUserID := r.PostForm.Get("user_id")

	err = h.services.RemoveUserBySlackID(ctx, slackUserID)
	if err != nil {
		appError := app_error.RemoveUserError
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}

	h.writeResponse(w, "All your data has been removed from Spotify Status", http.StatusOK)
}

func (h handlers) EnableHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := h.verifySlackSignature(w, r)
	if err != nil {
		fmt.Println(err)
		h.writeResponse(w, "error", http.StatusBadRequest)

		return
	}

	err = r.ParseForm()
	if err != nil {
		fmt.Println(err)

		return
	}

	user := domain.User{
		SlackUserID: r.PostForm.Get("user_id"),
		Enabled:     true,
	}

	err = h.services.UpdateUserEnabledBySlackID(ctx, user)
	if err != nil {
		appError := app_error.RemoveUserError
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}

	h.writeResponse(w, "Spotify Status has been enabled", http.StatusOK)
}

func (h handlers) DisableHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := h.verifySlackSignature(w, r)
	if err != nil {
		fmt.Println(err)
		h.writeResponse(w, "error", http.StatusBadRequest)

		return
	}

	err = r.ParseForm()
	if err != nil {
		fmt.Println(err)

		return
	}

	user := domain.User{
		SlackUserID: r.PostForm.Get("user_id"),
		Enabled:     false,
	}

	err = h.services.UpdateUserEnabledBySlackID(ctx, user)
	if err != nil {
		appError := app_error.RemoveUserError
		fmt.Println(err, appError)
		h.writeResponse(w, appError.Error(), appError.Status())

		return
	}

	h.writeResponse(w, "Spotify Status has been disabled", http.StatusOK)
}

func (h handlers) verifySlackSignature(w http.ResponseWriter, r *http.Request) error {
	slackTimestamp := r.Header.Get("X-Slack-Request-Timestamp")

	unixInt, err := strconv.ParseInt(slackTimestamp, 10, 64)
	if err != nil {
		return err
	}

	// The request timestamp is more than five minutes from local time.
	// It could be a replay attack, so let's ignore it.
	slackTime := time.UnixMilli(unixInt)
	if time.Since(slackTime) > 60*5 {
		return err
	}

	var bodyString string
	err = json.NewDecoder(r.Body).Decode(&bodyString)
	if err != nil {
		return err
	}

	signature := "v0:" + slackTimestamp + ":" + bodyString

	hash := hmac.New(sha256.New, []byte(h.slackSigningSecret))

	_, err = hash.Write([]byte(signature))
	if err != nil {
		return err
	}

	hashSignature := "v0=" + hex.EncodeToString(hash.Sum(nil))

	slackSignature := r.Header.Get("X-Slack-Signature")

	if !hmac.Equal([]byte(hashSignature), []byte(slackSignature)) {
		return err
	}

	return nil
}

func (h handlers) writeResponse(w http.ResponseWriter, resp interface{}, status int) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
	}

	w.Write(jsonResp)
}

func (h handlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("New Relic ok")
	fmt.Fprintf(w, "OK")
}
