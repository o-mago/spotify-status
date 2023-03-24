# Spotify Status for Slack
This slack app allows you to share your musical taste with your coworkers inside Slack, by allowing the app to connect to your spotify account

Built with:
- Go
- Docker
- Fly.io
- New Relic
- Slack API
- Spotify API

## Medium
https://medium.com/@alexandre.cabral/building-a-slack-app-for-spotify-with-go-64ff71959bd1

### Run locally
`docker compose up`

### Deploying
First, setup your fly.io account, database and new relic, then:
```
fly launch

fly auth login

fly secrets set <secret>

fly deploy
```

### Running app
[https://spotify-status-slack.fly.dev](https://spotify-status-slack.fly.dev)
