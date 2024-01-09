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

	command := userLibrary{}
	command.qualityProfiles = qualityProfiles
	command.allTags = tags
	command.filter = ""
	command.chatID = message.Chat.ID
	command.messageID = message.MessageID

	criteria := update.Message.CommandArguments()
	// no search criteria --> show menu and return
	if len(criteria) < 1 {
		b.setLibraryState(userID, &command)
		b.showLibraryMenu(update, &command)
		return
	}

	searchResults, err := r.Lookup(criteria)
	if err != nil {
		msg := tgbotapi.NewMessage(userID, err.Error())
		b.sendMessage(msg)
		return
	}
	if len(searchResults) == 0 {
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"No movies found matching your search criteria",
		)
		b.sendMessage(editMsg)
		return
	}
	if len(searchResults) > 25 {
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"Result size too large, please narrow down your search criteria",
		)
		b.sendMessage(editMsg)
		return
	}

	// if movie has a radarr ID, it's in the library
	var moviesInLibrary []*radarr.Movie
	for _, movie := range searchResults {
		if movie.ID != 0 {
			moviesInLibrary = append(moviesInLibrary, movie)
		}
	}
	if len(moviesInLibrary) == 0 {
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"No movies found in your library",
		)
		b.sendMessage(editMsg)
		return
	}

	command.searchResultsInLibrary = make(map[string]*radarr.Movie, len(moviesInLibrary))
	for _, movie := range moviesInLibrary {
		tmdbID := strconv.Itoa(int(movie.TmdbID))
		command.searchResultsInLibrary[tmdbID] = movie
	}

	// go to movie details
	if len(moviesInLibrary) == 1 {
		command.movie = moviesInLibrary[0]
		command.filter = "FILTER_SEARCHRESULTS"
		b.setLibraryState(command.chatID, &command)
		b.setActiveCommand(command.chatID, "LIBRARYFILTERED")
		b.showLibraryMovieDetail(update, &command)
		return
	} else {
		command.filter = "FILTER_SEARCHRESULTS"
		b.setLibraryState(command.chatID, &command)
		b.setActiveCommand(command.chatID, "LIBRARYFILTERED")
		b.showLibraryMenuFiltered(update, &command)
	}

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
	case "LIBRARY_MENU":
		command.filter = ""
		b.setLibraryState(command.chatID, command)
		b.showLibraryMenu(update, command)
		return false
	case "LIBRARY_CANCEL":
		b.clearState(update)
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"All commands have been cleared",
		)
		b.sendMessage(editMsg)
		return false
	default:
		command.filter = update.CallbackQuery.Data
		b.setLibraryState(command.chatID, command)
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
			tgbotapi.NewInlineKeyboardButtonData("All Movies", "FILTER_SHOWALL"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Cancel - clear command", "LIBRARY_CANCEL"),
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
	movies, err := b.RadarrServer.GetMovie(0)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		b.sendMessage(msg)
		return false
	}
	var filteredMovies []*radarr.Movie
	var responseText string

	switch command.filter {
	case "FILTER_MONITORED":
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return movie.Monitored
		})
		command.filter = "FILTER_MONITORED"
		responseText = "Monitored movies:"
	case "FILTER_UNMONITORED":
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return !movie.Monitored
		})
		command.filter = "FILTER_UNMONITORED"
		responseText = "Unmonitored movies:"
	case "FILTER_MISSING":
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return movie.SizeOnDisk == 0 && movie.Monitored
		})
		command.filter = "FILTER_MISSING"
		responseText = "Missing movies:"
	case "FILTER_WANTED":
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return movie.SizeOnDisk == 0 && movie.Monitored && movie.IsAvailable
		})
		command.filter = "FILTER_WANTED"
		responseText = "Wanted movies:"
	case "FILTER_ONDISK":
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return movie.SizeOnDisk > 0
		})
		command.filter = "FILTER_ONDISK"
		responseText = "Movies on disk:"
	case "FILTER_SHOWALL":
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return true // All movies included
		})
		command.filter = "FILTER_SHOWALL"
		responseText = "All Movies:"
	case "FILTER_SEARCHRESULTS":
		for _, movie := range command.searchResultsInLibrary {
			filteredMovies = append(filteredMovies, movie)
		}
		command.filter = "FILTER_SEARCHRESULTS"
		responseText = "Search Results:"
	default:
		command.filter = ""
		b.setLibraryState(command.chatID, command)
		return false
	}

	if len(filteredMovies) == 0 {
		b.clearState(update)
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"No (filteredMovies) movies in library. All commands have been cleared",
		)
		b.sendMessage(editMsg)
		return false
	}

	sort.SliceStable(filteredMovies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(filteredMovies[i].Title)) < utils.IgnoreArticles(strings.ToLower(filteredMovies[j].Title))
	})

	inlineKeyboard := b.getMoviesAsInlineKeyboard(filteredMovies)
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

	command.libraryFiltered = make(map[string]*radarr.Movie, len(filteredMovies))
	for _, movie := range filteredMovies {
		tmdbID := strconv.Itoa(int(movie.TmdbID))
		command.libraryFiltered[tmdbID] = movie
	}

	b.setLibraryState(command.chatID, command)
	b.setActiveCommand(command.chatID, "LIBRARYFILTERED")
	b.sendMessage(editMsg)
	return false
}

func filterMovies(movies []*radarr.Movie, filterCondition func(movie *radarr.Movie) bool) []*radarr.Movie {
	var filtered []*radarr.Movie
	for _, movie := range movies {
		if filterCondition(movie) {
			filtered = append(filtered, movie)
		}
	}
	return filtered
}
