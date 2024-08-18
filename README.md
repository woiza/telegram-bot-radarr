# Go-Powered Telegram Bot for Radarr Movie Management
This Telegram bot is specifically designed for movie management through Radarr, a movie collection manager. It enables users to execute a range of commands for searching, adding, editing, deleting, and organizing movies within their Radarr library. Developed in Go, the bot operates with minimal resource consumption, utilizing less than 10 MB of RAM. It maintains a stateless operation and does not persist data to disk, except for error logs. The Docker image size is efficiently kept under 10 MB (compressed), supporting multiple CPU architectures including `arm32v7`, `arm64v8`, and `x86_64`/`amd64`.

This bot is built using [golift/starr](https://github.com/golift/starr/) and [go-telegram-bot-api/telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api/) without any additional dependencies.

## Features and Commands

<img src="screenshots/menu.png?raw=true" alt="q1" title="menu" width="300" />

### Start Bot
<img src="screenshots/start.png?raw=true" alt="q1" title="start" width="300" />

### Search and Add Movies
``/q [movie]`` or just type the movie's title: Search for a movie.\
Once a movie is found, the bot offers options to add the movie to your Radarr library along with various monitoring settings. If you have only one root folder and one quality profile, the bot will automatically select the first option for you. However, if multiple choices exist, you will be prompted to select a root folder and a quality profile. If you have tags defined in Radarr, you can select them as well.

<img src="screenshots/add_links.png?raw=true" alt="q1" title="add movie" width="300" />
<img src="screenshots/add_inline.png?raw=true" alt="q2" title="add movie" width="300" />
<img src="screenshots/add_confirmation.png?raw=true" alt="q3" title="add movie" width="300" />
<img src="screenshots/add_monsea.png?raw=true" alt="q4" title="add movie" width="300" />

### Movie Management
``/library [movie]`` or ``/l [movie]``: Manage movies in your library. Allows editing a movie's quality profile (if more than one is configured in Radarr) and tags. Furthermore, you can monitor/unmonitor a movie, search for it, and delete it. Movie/title is optional. If omitted, a filter menu is shown.

<img src="screenshots/library.png?raw=true" alt="q1" title="library" width="300" />
<img src="screenshots/library_movie.png?raw=true" alt="q1" title="library movie" width="300" />


### Movie Deletion
``/delete [movie]`` or ``/d [movie]``: Initiate the process of deleting movies from your Radarr library. Be cautious as this action deletes associated files. Movie/title is optional. If omitted, all movies are shown as inline keyboards and multiple movies can be selected.

<img src="screenshots/delete_confirmation.png?raw=true" alt="q1" title="delete" width="300" />

### Cancel or Abort Commands
``/clear`` or ``/cancel`` or ``/stop``: 
This command clears all previously issued commands and resets the bot's state. It can be issued at any time.

### Library Management
- ``/up`` or ``/upcoming``: List upcoming movies in the next 30 days
- ``/rss``: Initiate an RSS sync
- ``/searchmonitored``: Search all monitored movies
- ``/updateall``: Update metadata and rescan files/folders for all movies


### System Information
- ``/free`` or ``/diskspace``: Display free space of disks connected to your Radarr server
- ``/system`` : Display your Radarr configuration
- ``/id`` or ``/getid``: Show your Telegram user ID


## Installation and Configuration
You can either build the bot yourself using the provided source code or utilize the Docker image hosted on GitHub Container Registry and Docker Hub:
- GitHub [ghcr.io/woiza/telegram-bot-radarr](https://github.com/woiza/telegram-bot-radarr/pkgs/container/telegram-bot-radarr)
- Docker Hub [woiza/telegram-bot-radarr](https://hub.docker.com/repository/docker/woiza/telegram-bot-radarr/)

The bot requires configuration through seven mandatory environment variables. For specific details, please refer to the Docker Compose example provided below. Before running this bot, ensure you have obtained a Telegram bot token and your Radarr API key. Additionally, determine who should have access to this bot (Telegram user ID). Several users are supported by providing a list of Telegram user IDs. You can find detailed instructions on obtaining these credentials in the official documentation:
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
            - RBOT_BOT_ALLOWED_USERIDS=123,987,-567 # Telegram user ID(s), Group IDs are negative
            - RBOT_BOT_MAX_ITEMS=10 # pagination
            - RBOT_BOT_IGNORE_TAGS=false # true/false; true = bot will not ask for tags (useful with auto-tagging)
            - RBOT_RADARR_PROTOCOL=http # http or https
            - RBOT_RADARR_PORT=7878
            - RBOT_RADARR_HOSTNAME=192.168.2.2 # IP or hostname
            - RBOT_RADARR_BASE_URL=/radarr # optional, e.g. /radarr, depending on radarr configuration
            - RBOT_RADARR_API_KEY=1010d7...
```
### Commands for Botfather's /setcommands

```
q - searches a movie 
library - lists all movies - WARNING: can be large
delete - deletes a movie - WARNING: can be large
clear - deletes all previously sent commands
free - lists the free space of your disks
up - lists upcoming movies in the next 30 days
rss - performs a RSS sync
searchmonitored - searches all monitored movies
updateall - updates metadata and rescan files/folders
system - shows your Radarr configuration
id - shows your Telegram user ID
```

## Contributing
Feel free to contribute to this Telegram bot by submitting pull requests, reporting issues, or suggesting enhancements. Your contributions are welcome!


## Beer
If you appreciate what we do, consider treating us to a refreshing beverage.

<a href="https://paypal.me/telegramarrbots?country.x=EUR" target="_blank">
  <img src="pp.png?raw=true" alt="q1" title="donate" width="200">
</a>


## License
This Telegram bot is licensed under the [MIT License](https://opensource.org/license/mit/).
