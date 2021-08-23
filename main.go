package main

import (
	"context"
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

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/robfig/cron/v3"
	"github.com/slack-go/slack"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

var (
	redirectURI = os.Getenv("SPOTIFY_REDIRECT_URL")
	auth        = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadCurrentlyPlaying)
	state       = tokenGenerator()
	port        = os.Getenv("PORT")
	conn        *pgxpool.Pool
)

type Cookie struct {
	Name       string
	Value      string
	Path       string
	Domain     string
	Expires    time.Time
	RawExpires string
	MaxAge     int
	Secure     bool
	HttpOnly   bool
	Raw        string
	Unparsed   []string
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
	slack               string
	clear               bool
	spotifyAccessToken  string
	spotifyRefreshToken string
	spotifyExpiry       time.Time
	spotifyTokenType    string
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

	conn, err = pgxpool.Connect(context.Background(), os.Getenv("DATABASE_SPOTIFY_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
	}
	defer conn.Close()

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

	var id string
	err = conn.QueryRow(context.Background(), "select id from users where id=$1", cookieUser.Value).Scan(&id)
	userInfo := APIs{
		slack:               cookieSlack.Value,
		spotifyAccessToken:  tok.AccessToken,
		spotifyRefreshToken: tok.RefreshToken,
		spotifyExpiry:       tok.Expiry,
		spotifyTokenType:    tok.TokenType,
		clear:               false,
	}
	if err != nil {
		err = addUser(conn, cookieUser.Value, userInfo)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}
	}
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
		fmt.Printf("Error: %s\n", err)
		return
	}

	expiration := time.Now().Add(1 * time.Hour)
	cookieUser := http.Cookie{Name: "user", Value: parsedResp.AuthedUser.Id, Expires: expiration}
	cookieSlack := http.Cookie{Name: "slack", Value: parsedResp.AuthedUser.AccessToken, Expires: expiration}
	http.SetCookie(w, &cookieUser)
	http.SetCookie(w, &cookieSlack)

	http.Redirect(w, r, spotifyUrl.url, http.StatusSeeOther)
}

func changeStatus() {
	rows, err := conn.Query(context.Background(), "select * from users")

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to query database: %v\n", err)
	}

	for rows.Next() {
		var id string
		var spotifyAccessToken string
		var spotifyRefreshToken string
		var spotifyExpiry time.Time
		var spotifyTokenType string
		var slackToken string
		var clear bool
		err := rows.Scan(&id, &slackToken, &clear, &spotifyAccessToken, &spotifyRefreshToken, &spotifyExpiry, &spotifyTokenType)
		if err != nil {
			fmt.Printf("Error: %s", err)
		}
		spotifyToken := new(oauth2.Token)
		spotifyToken.AccessToken = spotifyAccessToken
		spotifyToken.RefreshToken = spotifyRefreshToken
		spotifyToken.Expiry = spotifyExpiry
		spotifyToken.TokenType = slackToken
		go func(user string, slackToken string, spotifyToken *oauth2.Token, clear bool, conn *pgxpool.Pool) {

			slackApi := slack.New(slackToken)

			spotifyApi := auth.NewClient(spotifyToken)

			player, err := spotifyApi.PlayerCurrentlyPlaying()
			if err != nil {
				fmt.Printf("Error: %s\n", err)
				return
			}

			if player.Playing {
				songName := player.Item.Name
				slackStatus := songName + " - " + player.Item.Artists[0].Name
				if len(slackStatus) > 100 {
					extraChars := len(slackStatus) - 100 + 3
					songName = player.Item.Name[:len(player.Item.Name)-extraChars]
					slackStatus = songName + "... - " + player.Item.Artists[0].Name
				}
				err = slackApi.SetUserCustomStatusWithUser(user, slackStatus, ":spotify:", 0)

				if err != nil {
					fmt.Printf("Error: %s\n", err)
					return
				}
				err = updateUserClear(conn, true, user)
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					return
				}
			} else if clear {
				err = slackApi.SetUserCustomStatusWithUser(user, "", "", 0)

				if err != nil {
					fmt.Printf("Error: %s\n", err)
					return
				}
				err = updateUserClear(conn, false, user)
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					return
				}
			}
		}(id, slackToken, spotifyToken, clear, conn)
	}
}

func tokenGenerator() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func addUser(conn *pgxpool.Pool, user string, userInfo APIs) error {
	_, err := conn.Exec(context.Background(), `insert into 
	users(id, spotify_access_token, spotify_refresh_token, spotify_expiry, spotify_token_type, slack, clear) 
	values($1, $2, $3, $4, $5, $6, $7)`,
		user,
		userInfo.spotifyAccessToken,
		userInfo.spotifyRefreshToken,
		userInfo.spotifyExpiry,
		userInfo.spotifyTokenType,
		userInfo.slack,
		userInfo.clear)
	return err
}

func updateUserClear(conn *pgxpool.Pool, clear bool, id string) error {
	_, err := conn.Exec(context.Background(), "update users set clear=$1 where id=$2", clear, id)
	return err
}
