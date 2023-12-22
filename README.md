# Go-Powered Telegram Bot for Radarr Movie Management
This Telegram bot is specifically designed for movie management through Radarr, a movie collection manager. It enables users to execute a range of commands for searching, adding, deleting, and organizing movies within their Radarr library. Developed in Go, the bot operates with minimal resource consumption, utilizing less than 10 MB of RAM. It maintains a stateless operation and does not persist data to disk, except for error logs. The Docker image size is efficiently kept under 10 MB (compressed), supporting multiple CPU architectures including `arm32v7`, `arm64v8`, and `x86_64`/`amd64`.

This bot draws inspiration from [itsmegb/telegram-radarr-bot](https://github.com/itsmegb/telegram-radarr-bot/) and is built using [golift/starr](https://github.com/golift/starr/) and [go-telegram-bot-api/telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api/) without any additional dependencies.

## Features and Commands

![menu](screenshots/menu.jpg?raw=true "menu")

### Start Bot
![start](screenshots/start.png?raw=true "search movie")

### Search and Add Movies
``/q [movie]``: Search for a movie.\
Once a movie is found, the bot offers options to add the movie to your Radarr library along with various monitoring settings. If you have only one root folder and one quality profile, the bot will automatically select the first option for you. However, if multiple choices exist, you will be prompted to select a root folder and a quality profile.

![q1](screenshots/q1.png?raw=true "search movie")

![q2](screenshots/q2.png?raw=true "search movie")

![q3](screenshots/q3.png?raw=true "search movie")
`
`
### Cancel or Abort Commands
``/clear`` or ``/cancel`` or ``/stop``: 
This command clears all previously issued commands and resets the bot's state. It can be issued at any time.

### Movie Deletion
``/delete`` or ``/remove``: Initiate the process of deleting a movie from your Radarr library. Be cautious as this action deletes associated files.

![delete](screenshots/delete.png?raw=true "search movie")

### Movie Management
- ``/rss``: Initiate an RSS sync
- ``/wanted``: Search for all monitored movies
- ``/upcoming``: List upcoming movies in the next 30 days
- ``/dl`` or ``/downloaded``: List downloaded movies
- ``/library`` or ``/movies``: List all movies in the Radarr library
- ``/updateall``: Update metadata and rescan files/folders for all movies

![management](screenshots/management.png?raw=true "movie management")

### System Information
- ``/free`` or ``/diskspace``: Display free space of disks connected to your Radarr server.
- ``/id`` or ``/getid``: Show your Telegram user ID.


## Installation and Configuration
You can either build the bot yourself using the provided source code or utilize the Docker image hosted on GitHub Container Registry and Docker Hub:
- GitHub [ghcr.io/woiza/telegram-bot-radarr](https://github.com/woiza/telegram-bot-radarr/pkgs/container/telegram-bot-radarr)
- Docker Hub [woiza/telegram-bot-radarr](https://hub.docker.com/repository/docker/woiza/telegram-bot-radarr/)

The bot requires configuration through seven mandatory environment variables. For specific details, please refer to the Docker Compose example provided below. However, before running this bot, ensure you have obtained a Telegram bot token and your Radarr API key. Additionally, determine who should have access to this bot (Telegram user ID). You can find detailed instructions on obtaining these credentials in the official documentation:
- [Telegram Bot Token](https://core.telegram.org/bots/tutorial/)
- [Radarr API Key](https://wiki.servarr.com/en/radarr/settings#security/)



### Build Docker Image
```
docker buildx build --push --platform linux/amd64,linux/arm64,linux/arm/v7 --tag <repo>/<image>:<tag> .
```


### Docker Compose Example
```
services:
    telegram-bot-radarr:
        image: woiza/telegram-bot-radarr
        mem_limit: 128M
        container_name: telegram-bot-radarr
#        depends_on:
#            - radarr
        restart: always
        environment:
            - RBOT_TELEGRAM_BOT_TOKEN=1460...:AAHlBW_mabVg...
            - RBOT_BOT_ALLOWED_USERIDS=12345,98765,45678 # Telegram user ID(s)
            - RBOT_BOT_MAX_ITEMS=50 # 50 lines per message
            - RBOT_RADARR_PROTOCOL=http # http or https
            - RBOT_RADARR_PORT=7878
            - RBOT_RADARR_HOSTNAME=192.168.2.2 # IP or hostname
            - RBOT_RADARR_API_KEY=1010d7...
```
### Commands for Botfather's /setcommands

```
q - searches a movie 
clear - deletes all previously sent commands
free - lists the free space of your disks
delete - deletes a movie - WARNING: can be large
rss - performs a RSS sync
wanted - searches all monitored movies
upcoming - lists upcoming movies in the next 30 days
dl - lists downloaded movies - WARNING: can be large
library - lists all movies - WARNING: can be large
updateall - updates metadata and rescan files and folders
id - shows your Telegram user ID
```

## Contributing
Feel free to contribute to this Telegram bot by submitting pull requests, reporting issues, or suggesting enhancements. Your contributions are welcome!

## License
This Telegram bot is licensed under the [MIT License](https://opensource.org/license/mit/).