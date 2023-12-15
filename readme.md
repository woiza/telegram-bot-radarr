```
docker buildx build --push --platform linux/amd64,linux/arm64,linux/arm/v7 --tag woiza/telegram-bot-radarr:0.0.2 .
```

```
services:
    telegram-bot-radarr:
        image: woiza/telegram-bot-radarr:0.0.2
        mem_limit: 256M
        container_name: telegram-bot-radarr
#        depends_on:
#            - radarr
        restart: always
        environment:
            - RBOT_TELEGRAM_BOT_TOKEN=1460...:AAHlBW_mabVg...
            - RBOT_BOT_ALLOWED_USERIDS=1367,23112321,2331212 # Telegram user ID(s)
            - RBOT_BOT_MAX_ITEMS=50 # 50 lines per message
            - RBOT_RADARR_PROTOCOL=http
            - RBOT_RADARR_PORT=7878
            - RBOT_RADARR_HOSTNAME=192.168.1.2
            - RBOT_RADARR_API_KEY=20208d6f4d6...
```

```
q - searches a movie 
clear - deletes all previously sent commands
free - lists the free space of your disks
delete - delete a movie - WARNING: can be large
rss - perform a RSS sync
wanted - searches all monitored movies
upcoming - lists upcoming movies in the next 30 days
dl - lists downloaded movies - WARNING: can be large
library - lists all movies - WARNING: can be large
updateall - update metadata and rescan files and folders
id - shows your Telegram user ID
```