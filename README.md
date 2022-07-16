# Spotify Status for Slack
This slack app allows you to share your musical taste with your coworkers inside Slack, by allowing the app to connect to your spotify account

Built with:
- Go
- Docker
- Heroku
- New Relic
- Slack API
- Spotify API

### Run locally
`docker compose up`

### Deploying
First, setup your heroku account, database and new relic, then:
```
heroku login -i

heroku container:login

./deployHeroku.sh
```

### Running app
[https://spotify-status-slack.herokuapp.com/](https://spotify-status-slack.herokuapp.com/)
