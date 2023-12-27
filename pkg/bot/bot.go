package bot

import (
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golift.io/starr/radarr"

	"github.com/woiza/telegram-bot-radarr/pkg/config"
)

type userAddMovie struct {
	searchResults map[string]*radarr.Movie
	movie         *radarr.Movie
	confirmation  bool
	profileID     *int64
	path          *string
	addStatus     *string
}

type userDeleteMovie struct {
	library      map[string]*radarr.Movie
	movie        *radarr.Movie
	confirmation bool
}

type Bot struct {
	Config       *config.Config
	Bot          *tgbotapi.BotAPI
	RadarrServer *radarr.Radarr

	UserActiveCommand     map[int64]string
	AddMovieUserStates    map[int64]userAddMovie
	DeleteMovieUserStates map[int64]userDeleteMovie
}

func New(config *config.Config, botAPI *tgbotapi.BotAPI, radarrServer *radarr.Radarr) *Bot {
	return &Bot{
		Config:                config,
		Bot:                   botAPI,
		RadarrServer:          radarrServer,
		UserActiveCommand:     make(map[int64]string),
		AddMovieUserStates:    make(map[int64]userAddMovie),
		DeleteMovieUserStates: make(map[int64]userDeleteMovie),
	}
}

func (b *Bot) StartBot() {

	for userID := range b.Config.AllowedUserIDs {
		b.UserActiveCommand[userID] = ""
		b.AddMovieUserStates[userID] = userAddMovie{}
		b.DeleteMovieUserStates[userID] = userDeleteMovie{}
	}

	lastOffset := 0
	updateConfig := tgbotapi.NewUpdate(lastOffset + 1)
	updateConfig.Timeout = 60

	updatesChannel := b.Bot.GetUpdatesChan(updateConfig)

	time.Sleep(time.Millisecond * 500)
	updatesChannel.Clear()

	for update := range updatesChannel {
		lastOffset = update.UpdateID

		if update.Message != nil {
			if !b.Config.AllowedUserIDs[update.Message.From.ID] {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Access denied. You are not authorized.")
				b.sendMessage(msg)
				continue
			}
		}

		if update.CallbackQuery != nil {
			switch b.UserActiveCommand[update.CallbackQuery.From.ID] {
			case "ADDMOVIE":
				if !b.addMovie(update) {
					continue
				}
			case "DELETEMOVIE":
				if !b.deleteMovie(update) {
					continue
				}
			default:
				// Handle unexpected callback queries
				b.clearState(update)
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "I am not sure what you mean.\nAll commands have been cleared")
				b.sendMessage(msg)
				break
			}
		}

		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		// If no command was passed we will handle a search command.
		if update.Message.Entities == nil {
			update.Message.Text = fmt.Sprintf("/q %s", update.Message.Text)
			update.Message.Entities = []tgbotapi.MessageEntity{{
				Type:   "bot_command",
				Length: 2, // length of the command `/q`
			}}
		}

		if update.Message.IsCommand() {
			b.handleCommand(b.Bot, update, b.RadarrServer)
		}
	}
}

func (b *Bot) clearState(update tgbotapi.Update) {
	userID := update.Message.From.ID
	if update.CallbackQuery != nil {
		userID = update.CallbackQuery.From.ID
	}
	delete(b.UserActiveCommand, userID)
	delete(b.AddMovieUserStates, userID)
	delete(b.DeleteMovieUserStates, userID)
}
