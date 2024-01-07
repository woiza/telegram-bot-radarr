package bot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr/radarr"
)

func (b *Bot) processLibraryCommand(update tgbotapi.Update, userID int64, r *radarr.Radarr) {
	msg := tgbotapi.NewMessage(userID, "Handling library command... please wait")
	message, _ := b.sendMessage(msg)

	movies, err := r.GetMovie(0)
	if err != nil {
		msg := tgbotapi.NewMessage(userID, err.Error())
		b.sendMessage(msg)
		return
	}
	command := userLibrary{
		library: make(map[string]*radarr.Movie, len(movies)),
	}
	for _, movie := range movies {
		tmdbID := strconv.Itoa(int(movie.TmdbID))
		command.library[tmdbID] = movie
	}
	command.chatID = message.Chat.ID
	command.messageID = message.MessageID

	keyboard := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("Monitored Movies", "LIBRARY_MONITORED"),
			tgbotapi.NewInlineKeyboardButtonData("Unmonitored Movies", "LIBRARY_UNMONITORED"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Wanted Movies", "LIBRARY_WANTED"),
			tgbotapi.NewInlineKeyboardButtonData("Missing Movies", "LIBRARY_MISSING"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Cut-Off unmet Movies", "LIBRARY_CUTOFFUNMET"),
			tgbotapi.NewInlineKeyboardButtonData("Movies on Disk", "LIBRARY_ONDISK"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("All Movies", "LIBRARY_ALL"),
			tgbotapi.NewInlineKeyboardButtonData("Cancel - clear command", "LIBRARY_CANCEL"),
		},
	}

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		message.Chat.ID,
		message.MessageID,
		"Select an option:",
		tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard},
	)

	b.setLibraryState(userID, &command)
	b.sendMessage(editMsg)
	return

}

func (b *Bot) library(update tgbotapi.Update) bool {
	userID, err := b.getUserID(update)
	if err != nil {
		fmt.Printf("Cannot manage library: %v", err)
		return false
	}

	command, exists := b.getLibraryState(userID)
	if !exists {
		return false
	}

	switch {
	case command.movie == nil:
		return b.processLibrary(update, command)
	// case !command.confirmation:
	// 	return b.processConfirmationForDelete(update, command)
	default:
		return true
	}
}

func (b *Bot) processLibrary(update tgbotapi.Update, command *userLibrary) bool {
	var filtered []*radarr.Movie
	var responseText string

	switch update.CallbackQuery.Data {
	case "LIBRARY_MONITORED":
		for _, movie := range command.library {
			if movie.Monitored {
				filtered = append(filtered, movie)
			}
		}
		responseText = "Monitored movies:"
	case "LIBRARY_UNMONITORED":
		for _, movie := range command.library {
			if !movie.Monitored {
				filtered = append(filtered, movie)
			}
		}
		responseText = "Unmonitored movies:"
	case "LIBRARY_MISSING":
		for _, movie := range command.library {
			if movie.SizeOnDisk == 0 && movie.Monitored {
				filtered = append(filtered, movie)
			}
		}
		responseText = "Missing movies:"
	case "LIBRARY_WANTED":
		for _, movie := range command.library {
			if movie.SizeOnDisk == 0 && movie.Monitored && movie.IsAvailable {
				filtered = append(filtered, movie)
			}
		}
		responseText = "Wanted movies:"
	case "LIBRARY_CUTOFFUNMET":
		// Handle watched movies logic
		// ...
	case "LIBRARY_ONDISK":
		for _, movie := range command.library {
			if movie.SizeOnDisk > 0 {
				filtered = append(filtered, movie)
			}
		}
		responseText = "Movies on disk:"
	case "LIBRARY_ALL":
		// Handle all movies logic
		// ...
	case "LIBRARY_CANCEL":
		// Handle cancel logic
		// ...
	default:
		// Handle invalid command
		responseText = "Invalid command"
	}

	if len(filtered) == 0 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "No filtered movies in library")
		b.clearState(update)
		b.sendMessage(msg)
		return false
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(filtered[i].Title)) < utils.IgnoreArticles(strings.ToLower(filtered[j].Title))
	})

	inlineKeyboard := b.getMoviesAsInlineKeyboard(filtered)
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		responseText,
		tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	)
	b.sendMessage(editMsg)
	return false

}
