package bot

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

const (
	LibraryMovieEdit   = "LIBRARY_MOVIE_EDIT"
	LibraryMovieGoBack = "LIBRARY_MOVIE_GOBACK"
	//LibraryFilteredGoBack        = "LIBRARY_FILTERED_GOBACK" already defined in librarymenu.go
	LibraryMovieMonitor          = "LIBRARY_MOVIE_MONITOR"
	LibraryMovieUnmonitor        = "LIBRARY_MOVIE_UNMONITOR"
	LibraryMovieSearch           = "LIBRARY_MOVIE_SEARCH"
	LibraryMovieMonitorSearchNow = "LIBRARY_MOVIE_MONITOR_SEARCHNOW"
	LibraryFilteredActive        = "LIBRARYFILTERED"
	//LibraryMenuActive            = "LIBRARYMENU" already defined in librarymenu.go
)

const (
	MonitorIcon   = "\u2705" // Green checkmark
	UnmonitorIcon = "\u274C" // Red X
)

func (b *Bot) libraryFiltered(update tgbotapi.Update) bool {
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
	case LibraryMovieGoBack:
		command.movie = nil
		b.setActiveCommand(userID, LibraryFilteredActive)
		b.setLibraryState(command.chatID, command)
		return b.showLibraryMenuFiltered(update, command)
	case LibraryFilteredGoBack:
		command.filter = ""
		b.setActiveCommand(userID, LibraryMenuActive)
		b.setLibraryState(command.chatID, command)
		return b.showLibraryMenu(update, command)
	case LibraryMovieMonitor:
		return b.handleLibraryMovieMonitor(update, command)
	case LibraryMovieUnmonitor:
		return b.handleLibraryMovieUnMonitor(update, command)
	case LibraryMovieSearch:
		return b.handleLibraryMovieSearch(update, command)
	case LibraryMovieMonitorSearchNow:
		return b.handleLibraryMovieMonitorSearchNow(update, command)
	default:
		return b.showLibraryMovieDetail(update, command)
	}
}

func (b *Bot) showLibraryMovieDetail(update tgbotapi.Update, command *userLibrary) bool {
	var movie *radarr.Movie
	if command.movie == nil {
		movie = command.libraryFiltered[update.CallbackQuery.Data]
		command.movie = movie

	} else {
		movie = command.movie
	}

	var monitorIcon string
	if movie.Monitored {
		monitorIcon = MonitorIcon
	} else {
		monitorIcon = UnmonitorIcon
	}

	var lastSearchString string
	if command.lastSearch.IsZero() {
		lastSearchString = "" // Set empty string if the time is the zero value
	} else {
		lastSearchString = command.lastSearch.Format("02 Jan 06 - 15:04") // Convert non-zero time to string
	}

	var tagLabels []string
	for _, tagID := range movie.Tags {
		tag := findTagByID(command.allTags, tagID)
		tagLabels = append(tagLabels, tag.Label)
	}
	tagsString := strings.Join(tagLabels, ", ")

	// Create a message with movie details
	messageText := fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n\n", utils.Escape(movie.Title), movie.ImdbID, movie.Year)
	messageText += fmt.Sprintf("Monitored: %s\n", monitorIcon)
	messageText += fmt.Sprintf("Last Manual Search: %s\n", utils.Escape(lastSearchString))
	messageText += fmt.Sprintf("Status: %s\n", utils.Escape(movie.Status))
	messageText += fmt.Sprintf("Language: %s\n", utils.Escape(movie.OriginalLanguage.Name))
	messageText += fmt.Sprintf("Size: %d GB\n", movie.SizeOnDisk/(1024*1024*1024))
	messageText += fmt.Sprintf("Tags: %s\n", utils.Escape(tagsString))
	messageText += fmt.Sprintf("Quality Profile: %s\n", utils.Escape(findQualityProfileByID(command.qualityProfiles, movie.QualityProfileID).Name))

	var keyboard tgbotapi.InlineKeyboardMarkup
	if !movie.Monitored {
		keyboard = b.createKeyboard(
			[]string{LibraryMovieMonitor, LibraryMovieMonitorSearchNow, LibraryMovieEdit, LibraryMovieGoBack},
			[]string{"Monitor Movie", "Monitor Movie & Search Now", "Edit Movie", "Go back - Show Movies"},
		)
	} else {
		keyboard = b.createKeyboard(
			[]string{LibraryMovieUnmonitor, LibraryMovieSearch, LibraryMovieEdit, LibraryMovieGoBack},
			[]string{"Unmonitor Movie", "Search Movie", "Edit Movie", "Go back - Show Movies"},
		)
	}

	// Send the message containing movie details along with the keyboard
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		messageText,
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = true
	b.setLibraryState(command.chatID, command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) handleLibraryMovieMonitor(update tgbotapi.Update, command *userLibrary) bool {
	bulkEdit := radarr.BulkEdit{
		MovieIDs:  []int64{command.movie.ID},
		Monitored: starr.True(),
	}
	_, err := b.RadarrServer.EditMovies(&bulkEdit)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		b.sendMessage(msg)
		return false
	}
	command.movie.Monitored = true
	b.setLibraryState(command.chatID, command)
	return b.showLibraryMovieDetail(update, command)
}

func (b *Bot) handleLibraryMovieUnMonitor(update tgbotapi.Update, command *userLibrary) bool {
	bulkEdit := radarr.BulkEdit{
		MovieIDs:  []int64{command.movie.ID},
		Monitored: starr.False(),
	}
	_, err := b.RadarrServer.EditMovies(&bulkEdit)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		b.sendMessage(msg)
		return false
	}
	command.movie.Monitored = false
	b.setLibraryState(command.chatID, command)
	return b.showLibraryMovieDetail(update, command)
}

func (b *Bot) handleLibraryMovieSearch(update tgbotapi.Update, command *userLibrary) bool {
	cmd := radarr.CommandRequest{
		Name:     "MoviesSearch",
		MovieIDs: []int64{command.movie.ID},
	}
	_, err := b.RadarrServer.SendCommand(&cmd)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		b.sendMessage(msg)
		return false
	}
	command.lastSearch = time.Now()
	b.setLibraryState(command.chatID, command)
	return b.showLibraryMovieDetail(update, command)
}

func (b *Bot) handleLibraryMovieMonitorSearchNow(update tgbotapi.Update, command *userLibrary) bool {
	bulkEdit := radarr.BulkEdit{
		MovieIDs:  []int64{command.movie.ID},
		Monitored: starr.True(),
	}
	_, err := b.RadarrServer.EditMovies(&bulkEdit)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		b.sendMessage(msg)
		return false
	}
	command.movie.Monitored = true
	cmd := radarr.CommandRequest{
		Name:     "MoviesSearch",
		MovieIDs: []int64{command.movie.ID},
	}
	_, err = b.RadarrServer.SendCommand(&cmd)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		b.sendMessage(msg)
		return false
	}
	command.lastSearch = time.Now()
	b.setLibraryState(command.chatID, command)
	return b.showLibraryMovieDetail(update, command)
}

func findQualityProfileByID(qualityProfiles []*radarr.QualityProfile, qualityProfileID int64) *radarr.QualityProfile {
	for _, profile := range qualityProfiles {
		if profile.ID == qualityProfileID {
			return profile
		}
	}
	return nil
}

func (b *Bot) createKeyboard(buttonData, buttonText []string) tgbotapi.InlineKeyboardMarkup {
	buttons := make([][]tgbotapi.InlineKeyboardButton, len(buttonData))
	for i := range buttonData {
		buttons[i] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(buttonText[i], buttonData[i]))
	}
	return tgbotapi.NewInlineKeyboardMarkup(buttons...)
}
