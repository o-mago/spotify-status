package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/robfig/cron/v3"
	"github.com/slack-go/slack"
	"github.com/zmb3/spotify"
)

var redirectURI = os.Getenv("SPOTIFY_REDIRECT_URL")

var (
	auth      = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadCurrentlyPlaying)
	chSpotify = make(chan *spotify.Client)
	state     = tokenGenerator()
	user      = ""
)

type URLString struct {
	url string
}

type AuthedUserStruct struct {
	Id          string `json:"id"`
	Scope       string `json:"scope"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type TeamStruct struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type SlackResp struct {
	Ok         bool             `json:"ok"`
	AppId      string           `json:"app_id"`
	AuthedUser AuthedUserStruct `json:"authed_user"`
	Team       TeamStruct       `json:"team"`
	Enterprise string           `json:"enterprise"`
}

func main() {
	spotifyUrl := &URLString{url: auth.AuthURL(state)}
	// http.Server.Addr
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/slackAuth", spotifyUrl.slackAdd)
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	go http.ListenAndServe(":8080", nil)

	client := <-chSpotify

	api := slack.New(os.Getenv("SLACK_TOKEN"))

	c := cron.New(cron.WithSeconds())
	c.AddFunc("@every 10s", func() { changeStatus(client, api) })
	c.Start()

	select {}
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	client := auth.NewClient(tok)
	fmt.Fprintf(w, "Login Completed!")
	chSpotify <- &client
}

func (spotifyUrl *URLString) slackAdd(w http.ResponseWriter, r *http.Request) {
	slackCode := r.URL.Query().Get("code")

	requestBody := url.Values{}
	requestBody.Set("code", slackCode)
	requestBody.Set("client_id", os.Getenv("SLACK_CLIENT_ID"))
	requestBody.Set("client_secret", os.Getenv("SLACK_CLIENT_SECRET"))

	resp, err := http.Post("https://slack.com/api/oauth.v2.access", "application/x-www-form-urlencoded", strings.NewReader(requestBody.Encode()))

	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	var parsedResp SlackResp
	err = json.Unmarshal(body, &parsedResp)
	if err != nil {
		panic(err)
	}

	user = parsedResp.AuthedUser.Id

	http.Redirect(w, r, spotifyUrl.url, http.StatusSeeOther)
}

func changeStatus(client *spotify.Client, api *slack.Client) {
	player, err := client.PlayerCurrentlyPlaying()
	if err != nil {
		log.Fatal(err)
	}

	errors := api.SetUserCustomStatusWithUser(user, player.Item.Name+" - "+player.Item.Artists[0].Name, ":musical_note:", 0)

	if errors != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
}

func tokenGenerator() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
