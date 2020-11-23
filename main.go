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
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/robfig/cron/v3"
	"github.com/slack-go/slack"
	"github.com/zmb3/spotify"
)

var (
	redirectURI = os.Getenv("SPOTIFY_REDIRECT_URL")
	auth        = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadCurrentlyPlaying)
	chSpotify   = make(chan *spotify.Client)
	state       = tokenGenerator()
	users       = make(map[string]APIs)
	port        = os.Getenv("PORT")
)

type Cookie struct {
	Name       string
	Value      string
	Path       string
	Domain     string
	Expires    time.Time
	RawExpires string

	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'
	// MaxAge>0 means Max-Age attribute present and given in seconds
	MaxAge   int
	Secure   bool
	HttpOnly bool
	Raw      string
	Unparsed []string // Raw text of unparsed attribute-value pairs
}

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

type APIs struct {
	spotify *spotify.Client
	slack   *slack.Client
	clear   bool
}

func main() {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(os.Getenv("NEW_RELIC_APP_NAME")),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE")),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	spotifyUrl := &URLString{url: auth.AuthURL(state)}
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/slackAuth", spotifyUrl.slackAdd)
	http.HandleFunc(newrelic.WrapHandleFunc(app, "/users", usersHandler))
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	go http.ListenAndServe(":"+port, nil)

	c := cron.New(cron.WithSeconds())
	c.AddFunc("@every 10s", func() { changeStatus() })
	c.Start()

	// client := <-chSpotify

	// api := slack.New(os.Getenv("SLACK_TOKEN"))

	// c := cron.New(cron.WithSeconds())
	// c.AddFunc("@every 10s", func() { changeStatus(client, api) })
	// c.Start()

	select {}
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("New Relic ok")
	fmt.Fprintf(w, "OK")
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

	fmt.Fprintf(w, "Login Completed!")

	cookieUser, _ := r.Cookie("user")
	cookieSlack, _ := r.Cookie("slack")

	if _, ok := users[cookieUser.Value]; !ok {
		api := slack.New(cookieSlack.Value)
		client := auth.NewClient(tok)
		userInfo := APIs{
			slack:   api,
			spotify: &client,
			clear:   false,
		}
		users[cookieUser.Value] = userInfo
	}
	// chSpotify <- &client
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

	expiration := time.Now().Add(1 * time.Hour)
	cookieUser := http.Cookie{Name: "user", Value: parsedResp.AuthedUser.Id, Expires: expiration}
	cookieSlack := http.Cookie{Name: "slack", Value: parsedResp.AuthedUser.AccessToken, Expires: expiration}
	http.SetCookie(w, &cookieUser)
	http.SetCookie(w, &cookieSlack)

	http.Redirect(w, r, spotifyUrl.url, http.StatusSeeOther)
}

func changeStatus() {
	fmt.Println(len(users))
	for user, userInfo := range users {
		go func(user string, apis APIs) {
			player, err := apis.spotify.PlayerCurrentlyPlaying()
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(user, player.Item.Name+" - "+player.Item.Artists[0].Name)

			if player.Playing {
				err = apis.slack.SetUserCustomStatusWithUser(user, player.Item.Name+" - "+player.Item.Artists[0].Name, ":spotify:", 0)

				if err != nil {
					fmt.Printf("Error: %s\n", err)
					return
				}
				apis.clear = true
			} else if apis.clear {
				err = apis.slack.SetUserCustomStatusWithUser(user, "", "", 0)

				if err != nil {
					fmt.Printf("Error: %s\n", err)
					return
				}
				apis.clear = false
			}
		}(user, userInfo)
	}

}

func tokenGenerator() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
