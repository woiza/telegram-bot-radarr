package main

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golift.io/starr"
	"golift.io/starr/radarr"

	"github.com/woiza/telegram-bot-radarr/pkg/bot"
	"github.com/woiza/telegram-bot-radarr/pkg/config"
)

func main() {
	fmt.Println("Starting bot...")

	// get config from environment variables
	config, err := config.LoadConfig()
	if err != nil {
		// Handle error: configuration is incomplete or invalid
		log.Fatal(err)
	}
	b, err := tgbotapi.NewBotAPI(config.TelegramBotToken)
	if err != nil {
		log.Fatal("Error while starting bot: ", err)
	}

	fmt.Printf("Authorized on account %v\n", b.Self.UserName)

	// Get a starr.Config that can plug into any Starr app.
	// starr.New(apiKey, appURL string, timeout time.Duration)
	radarrConfig := starr.New(config.RadarrAPIKey, fmt.Sprintf("%v://%v:%v", config.RadarrProtocol, config.RadarrHostname, config.RadarrPort), 0)
	// Lets make a radarr server with the default starr Config.
	radarrServer := radarr.New(radarrConfig)

	botConfig := bot.Bot{
		Bot:          b,
		RadarrServer: radarrServer,
		Config:       &config,
	}
	botConfig.StartBot()
}
