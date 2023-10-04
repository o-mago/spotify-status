# Spotify Status for Slack
This slack app allows you to share your musical taste with your coworkers inside Slack, by allowing the app to connect to your spotify account

## Built and running with:
- Go
- Docker
- Fly.io
- New Relic
- Slack API
- Spotify API

## Go external libs
- [google/uuid](https://github.com/google/uuid)
- [new-relic/go-agent](https://github.com/newrelic/go-agent)
- [robfig/cron/v3](https://github.com/robfig/cron)
- [slack-go](https://github.com/slack-go/slack)
- [zmb3/spotify](https://github.com/zmb3/spotify)
- [gorm](https://gorm.io/index.html)

## Application layers

There are 3 main layers:
- handlers: only call exported services.
- services: call unexported services and repositories
- repositories: communicate with database (and eventually other repos)

<p align="center">
  <img src="https://github.com/o-mago/spotify-status/assets/23153316/adac1b5c-e5d3-4fff-a40f-8a4b30d78047" alt="Application layers"/>
</p>



## Folders structure

📦src<br>
 ┣ 📂app_error<br>
 ┣ 📂crypto<br>
 ┣ 📂domain<br>
 ┣ 📂handlers<br>
 ┣ 📂repositories<br>
 ┃ ┣ 📂db_entities<br>
 ┣ 📂services<br>
 ┣ 📂static<br>
 ┃ ┣ 📂completed<br>
 ┃ ┣ 📂home<br>
 ┗ 📜server.go<br>
 
`app_error`: custom application errors

`domain`: entities from application business rules

`handlers`: api handlers

`repositories`: database related, including queries

`db_entities`: database entities, a mirror from the schema

`services`: where all the logic is applied to make the magic happen

`static`: UI files

`completed`: UI for the completed page (after the user accepted everything)

`home`: UI for the homepage

## Medium (outdated)
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
