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

const (
	LibraryFilteredGoBack = "LIBRARY_FILTERED_GOBACK"
	LibraryMenu           = "LIBRARY_MENU"
	LibraryCancel         = "LIBRARY_CANCEL"
	LibraryMenuActive     = "LIBRARYMENU"
	LibraryFiltered       = "LIBRARYFILTERED"
	CommandsCleared       = "All commands have been cleared"
)

const (
	FilterMonitored     = "FILTER_MONITORED"
	FilterUnmonitored   = "FILTER_UNMONITORED"
	FilterMissing       = "FILTER_MISSING"
	FilterWanted        = "FILTER_WANTED"
	FilterOnDisk        = "FILTER_ONDISK"
	FilterShowAll       = "FILTER_SHOWALL"
	FilterSearchResults = "FILTER_SEARCHRESULTS"
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

	b.handleSearchResults(update, searchResults, &command)

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
	case LibraryFilteredGoBack:
		command.filter = ""
		b.setActiveCommand(userID, LibraryMenuActive)
		b.setLibraryState(command.chatID, command)
		return b.showLibraryMenu(update, command)
	case LibraryMenu:
		command.filter = ""
		b.setLibraryState(command.chatID, command)
		b.showLibraryMenu(update, command)
		return false
	case LibraryCancel:
		b.clearState(update)
		b.sendMessageWithEdit(command, CommandsCleared)
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
			tgbotapi.NewInlineKeyboardButtonData("Missing Movies", FilterMissing),
			tgbotapi.NewInlineKeyboardButtonData("Wanted Movies", FilterWanted),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Monitored Movies", FilterMonitored),
			tgbotapi.NewInlineKeyboardButtonData("Unmonitored Movies", FilterUnmonitored),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Movies on Disk", FilterOnDisk),
			tgbotapi.NewInlineKeyboardButtonData("All Movies", FilterShowAll),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("Cancel - clear command", LibraryCancel),
		},
	}

	b.sendMessageWithEditAndKeyboard(command, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: keyboard}, "Select an option:")
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
	case FilterMonitored:
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return movie.Monitored
		})
		command.filter = FilterMonitored
		responseText = "Monitored movies:"
	case FilterUnmonitored:
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return !movie.Monitored
		})
		command.filter = FilterUnmonitored
		responseText = "Unmonitored movies:"
	case FilterMissing:
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return movie.SizeOnDisk == 0 && movie.Monitored
		})
		command.filter = FilterMissing
		responseText = "Missing movies:"
	case FilterWanted:
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return movie.SizeOnDisk == 0 && movie.Monitored && movie.IsAvailable
		})
		command.filter = FilterWanted
		responseText = "Wanted movies:"
	case FilterOnDisk:
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return movie.SizeOnDisk > 0
		})
		command.filter = FilterOnDisk
		responseText = "Movies on disk:"
	case FilterShowAll:
		filteredMovies = filterMovies(movies, func(movie *radarr.Movie) bool {
			return true // All movies included
		})
		command.filter = FilterShowAll
		responseText = "All Movies:"
	case FilterSearchResults:
		for _, movie := range command.searchResultsInLibrary {
			filteredMovies = append(filteredMovies, movie)
		}
		command.filter = FilterSearchResults
		responseText = "Search Results:"
	default:
		command.filter = ""
		b.setLibraryState(command.chatID, command)
		return false
	}

	if len(filteredMovies) == 0 {
		b.clearState(update)
		b.sendMessageWithEdit(command, "No movies in your library. All commands have been cleared")
		return false
	}

	sort.SliceStable(filteredMovies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(filteredMovies[i].Title)) < utils.IgnoreArticles(strings.ToLower(filteredMovies[j].Title))
	})

	inlineKeyboard := b.getMoviesAsInlineKeyboard(filteredMovies)
	var row []tgbotapi.InlineKeyboardButton
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("Go back - Show library menu", LibraryFilteredGoBack))
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
	b.setActiveCommand(command.chatID, LibraryFiltered)
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

func (b *Bot) handleSearchResults(update tgbotapi.Update, searchResults []*radarr.Movie, command *userLibrary) {
	if len(searchResults) == 0 {
		b.sendMessageWithEdit(command, "No movies found matching your search criteria")
		return
	}
	if len(searchResults) > 25 {
		b.sendMessageWithEdit(command, "Result size too large, please narrow down your search criteria")
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
		b.sendMessageWithEdit(command, "No movies found in your library")
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
		command.filter = FilterSearchResults
		b.setLibraryState(command.chatID, command)
		b.setActiveCommand(command.chatID, LibraryFilteredCommand)
		b.showLibraryMovieDetail(update, command)
	} else {
		command.filter = FilterSearchResults
		b.setLibraryState(command.chatID, command)
		b.setActiveCommand(command.chatID, LibraryFilteredCommand)
		b.showLibraryMenuFiltered(update, command)
	}
}
