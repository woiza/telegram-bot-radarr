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
	qualityProfiles, err := r.GetQualityProfiles()
	if err != nil {
		msg := tgbotapi.NewMessage(userID, err.Error())
		b.sendMessage(msg)
		return
	}
	tags, err := r.GetTags()
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

	command.qualityProfiles = qualityProfiles
	command.allTags = tags
	command.filter = ""
	command.chatID = message.Chat.ID
	command.messageID = message.MessageID

	b.setLibraryState(userID, &command)
	b.showLibraryMenu(update, &command)
	return
}

func (b *Bot) libraryMenu(update tgbotapi.Update) bool {
	userID, err := b.getUserID(update)
	if err != nil {
		fmt.Printf("Cannot manage library: %v", err)
		return false
	}

	command, exists := b.getLibraryState(userID)
	if !exists {
		return false
	}
	switch update.CallbackQuery.Data {
	case "LIBRARY_FILTERED_GOBACK":
		command.filter = ""
		b.setActiveCommand(userID, "LIBRARYMENU")
		b.setLibraryState(command.chatID, command)
		return b.showLibraryMenu(update, command)
	default:
		return b.showLibraryMenuFiltered(update, command)
	}
}

func (b *Bot) showLibraryMenu(update tgbotapi.Update, command *userLibrary) bool {
	keyboard := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("Missing Movies", "FILTER_MISSING"),
			tgbotapi.NewInlineKeyboardButtonData("Wanted Movies", "FILTER_WANTED"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Monitored Movies", "FILTER_MONITORED"),
			tgbotapi.NewInlineKeyboardButtonData("Unmonitored Movies", "FILTER_UNMONITORED"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Movies on Disk", "FILTER_ONDISK"),
			tgbotapi.NewInlineKeyboardButtonData("All Movies", "FILTER_ALL"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Cancel - clear command", "FILTER_CANCEL"),
		},
	}

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		"Select an option:",
		tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard},
	)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) showLibraryMenuFiltered(update tgbotapi.Update, command *userLibrary) bool {
	var filtered []*radarr.Movie
	var responseText string
	if command.filter != "" {
		update.CallbackQuery.Data = command.filter
	}

	switch update.CallbackQuery.Data {
	case "FILTER_MONITORED":
		filtered = filterMovies(command.library, func(movie *radarr.Movie) bool {
			return movie.Monitored
		})
		command.filter = "FILTER_MONITORED"
		responseText = "Monitored movies:"
	case "FILTER_UNMONITORED":
		filtered = filterMovies(command.library, func(movie *radarr.Movie) bool {
			return !movie.Monitored
		})
		command.filter = "FILTER_UNMONITORED"
		responseText = "Unmonitored movies:"
	case "FILTER_MISSING":
		filtered = filterMovies(command.library, func(movie *radarr.Movie) bool {
			return movie.SizeOnDisk == 0 && movie.Monitored
		})
		command.filter = "FILTER_MISSING"
		responseText = "Missing movies:"
	case "FILTER_WANTED":
		filtered = filterMovies(command.library, func(movie *radarr.Movie) bool {
			return movie.SizeOnDisk == 0 && movie.Monitored && movie.IsAvailable
		})
		command.filter = "FILTER_WANTED"
		responseText = "Wanted movies:"
	case "FILTER_ONDISK":
		filtered = filterMovies(command.library, func(movie *radarr.Movie) bool {
			return movie.SizeOnDisk > 0
		})
		command.filter = "FILTER_ONDISK"
		responseText = "Movies on disk:"
	case "FILTER_ALL":
		filtered = filterMovies(command.library, func(movie *radarr.Movie) bool {
			return true // All movies included
		})
		command.filter = "FILTER_ALL"
		responseText = "All Movies:"
	case "FILTER_CANCEL":
		b.clearState(update)
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"All commands have been cleared",
		)
		b.sendMessage(editMsg)
		return false
	case "LIBRARY_MENU":
		command.filter = ""
		b.setLibraryState(command.chatID, command)
		b.showLibraryMenu(update, command)
		return false
	default:
		command.filter = ""
		b.setLibraryState(command.chatID, command)
		return false
	}

	if len(filtered) == 0 {
		b.clearState(update)
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"No (filtered) movies in library. All commands have been cleared",
		)
		b.sendMessage(editMsg)
		return false
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(filtered[i].Title)) < utils.IgnoreArticles(strings.ToLower(filtered[j].Title))
	})

	inlineKeyboard := b.getMoviesAsInlineKeyboard(filtered)
	var row []tgbotapi.InlineKeyboardButton
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("Go back - Show library menu", "LIBRARY_FILTERED_GOBACK"))
	inlineKeyboard = append(inlineKeyboard, row)
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		responseText,
		tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	)

	b.setLibraryState(command.chatID, command)
	b.setActiveCommand(command.chatID, "LIBRARYFILTERED")
	b.sendMessage(editMsg)
	return false
}

func filterMovies(library map[string]*radarr.Movie, filterCondition func(movie *radarr.Movie) bool) []*radarr.Movie {
	var filtered []*radarr.Movie
	for _, movie := range library {
		if filterCondition(movie) {
			filtered = append(filtered, movie)
		}
	}
	return filtered
}
