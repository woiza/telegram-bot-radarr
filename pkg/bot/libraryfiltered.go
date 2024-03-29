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
	LibraryMovieDelete    = "LIBRARY_MOVIE_DELETE"
	LibraryMovieDeleteYes = "LIBRARY_MOVIE_DELETE_YES"
	LibraryMovieDeleteNo  = "LIBRARY_MOVIE_DELETE_NO"
	LibraryMovieEdit      = "LIBRARY_MOVIE_EDIT"
	LibraryMovieGoBack    = "LIBRARY_MOVIE_GOBACK"
	//LibraryFilteredGoBack        = "LIBRARY_FILTERED_GOBACK" already defined in librarymenu.go
	LibraryMovieMonitor          = "LIBRARY_MOVIE_MONITOR"
	LibraryMovieUnmonitor        = "LIBRARY_MOVIE_UNMONITOR"
	LibraryMovieSearch           = "LIBRARY_MOVIE_SEARCH"
	LibraryMovieMonitorSearchNow = "LIBRARY_MOVIE_MONITOR_SEARCHNOW"
	LibraryFilteredActive        = "LIBRARYFILTERED"
	//LibraryMenuActive            = "LIBRARYMENU" already defined in librarymenu.go
	LibraryFirstPage    = "LIBRARY_FIRST_PAGE"
	LibraryPreviousPage = "LIBRARY_PREV_PAGE"
	LibraryNextPage     = "LIBRARY_NEXT_PAGE"
	LibraryLastPage     = "LIBRARY_LAST_PAGE"
)

const (
	MonitorIcon   = "\u2705" // Green checkmark
	UnmonitorIcon = "\u274C" // Red X
)

func (b *Bot) libraryFiltered(update tgbotapi.Update) bool {
	chatID, err := b.getChatID(update)
	if err != nil {
		fmt.Printf("Cannot manage library: %v", err)
		return false
	}

	command, exists := b.getLibraryState(chatID)
	if !exists {
		return false
	}
	switch update.CallbackQuery.Data {
	// ignore click on page number
	case "current_page":
		return false
	case LibraryFirstPage:
		command.page = 0
		return b.showLibraryMenuFiltered(command)
	case LibraryPreviousPage:
		if command.page > 0 {
			command.page--
		}
		return b.showLibraryMenuFiltered(command)
	case LibraryNextPage:
		command.page++
		return b.showLibraryMenuFiltered(command)
	case LibraryLastPage:
		totalPages := (len(command.libraryFiltered) + b.Config.MaxItems - 1) / b.Config.MaxItems
		command.page = totalPages - 1
		return b.showLibraryMenuFiltered(command)
	case LibraryMovieGoBack:
		command.movie = nil
		b.setActiveCommand(chatID, LibraryFilteredActive)
		b.setLibraryState(command.chatID, command)
		return b.showLibraryMenuFiltered(command)
	case LibraryFilteredGoBack:
		command.filter = ""
		b.setActiveCommand(chatID, LibraryMenuActive)
		b.setLibraryState(command.chatID, command)
		return b.showLibraryMenu(command)
	case LibraryMovieMonitor:
		return b.handleLibraryMovieMonitor(update, command)
	case LibraryMovieUnmonitor:
		return b.handleLibraryMovieUnMonitor(update, command)
	case LibraryMovieSearch:
		return b.handleLibraryMovieSearch(update, command)
	case LibraryMovieDelete:
		return b.handleLibraryMovieDelete(command)
	case LibraryMovieDeleteYes:
		return b.handleLibraryMovieDeleteYes(update, command)
	case LibraryMovieDeleteNo:
		return b.showLibraryMovieDetail(update, command)
	case LibraryMovieEdit:
		return b.handleLibraryMovieEdit(command)
	case LibraryMovieMonitorSearchNow:
		return b.handleLibraryMovieMonitorSearchNow(update, command)
	default:
		return b.showLibraryMovieDetail(update, command)
	}
}

func (b *Bot) showLibraryMovieDetail(update tgbotapi.Update, command *userLibrary) bool {
	var movie *radarr.Movie
	if command.movie == nil {
		movieIDStr := strings.TrimPrefix(update.CallbackQuery.Data, "TMDBID_")
		movie = command.libraryFiltered[movieIDStr]
		command.movie = movie

	} else {
		movie = command.movie
	}

	command.selectedMonitoring = movie.Monitored
	command.selectedTags = movie.Tags
	command.selectedQualityProfile = movie.QualityProfileID

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
		command.selectedTags = append(command.selectedTags, tag.ID)
	}
	tagsString := strings.Join(tagLabels, ", ")

	movieFiles, err := b.RadarrServer.GetMovieFile(movie.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		b.sendMessage(msg)
		return false
	}

	size := int64(0)
	quality := ""
	videoCodec := ""
	videoDynamicRange := ""
	audioInfo := ""
	customFormatScore := ""
	languages := ""
	formats := ""
	if len(movieFiles) == 1 {
		size = movieFiles[0].Size
		quality = movieFiles[0].Quality.Quality.Name
		videoCodec = movieFiles[0].MediaInfo.VideoCodec
		videoDynamicRange = movieFiles[0].MediaInfo.VideoDynamicRangeType
		audioInfo = movieFiles[0].MediaInfo.AudioCodec
		customFormatScore = fmt.Sprintf("%d", movieFiles[0].CustomFormatScore)
		languages = movieFiles[0].MediaInfo.AudioLanguages
		formats = movieFiles[0].MediaInfo.VideoDynamicRangeType
	}

	// Create a message with movie details
	var message strings.Builder
	fmt.Fprintf(&message, "[%v](https://www.imdb.com/title/%v) \\- _%v_\n\n", utils.Escape(movie.Title), movie.ImdbID, movie.Year)
	fmt.Fprintf(&message, "Monitored: %s\n", monitorIcon)
	fmt.Fprintf(&message, "Status: %s\n", utils.Escape(movie.Status))
	fmt.Fprintf(&message, "Last Manual Search: %s\n", utils.Escape(lastSearchString))
	fmt.Fprintf(&message, "Size: %d GB\n", size/(1024*1024*1024))
	fmt.Fprintf(&message, "Quality: %s\n", utils.Escape(quality))
	fmt.Fprintf(&message, "Video Codec: %s\n", utils.Escape(videoCodec))
	fmt.Fprintf(&message, "Video Dynamic Range: %s\n", utils.Escape(videoDynamicRange))
	fmt.Fprintf(&message, "Audio Info: %s\n", utils.Escape(audioInfo))
	fmt.Fprintf(&message, "Formats: %s\n", utils.Escape(formats))
	fmt.Fprintf(&message, "Languages: %s\n", utils.Escape(languages))
	fmt.Fprintf(&message, "Tags: %s\n", utils.Escape(tagsString))
	fmt.Fprintf(&message, "Quality Profile: %s\n", utils.Escape(findQualityProfileByID(command.qualityProfiles, movie.QualityProfileID).Name))
	fmt.Fprintf(&message, "Custom Format Score: %s\n", utils.Escape(customFormatScore))

	messageText := message.String()

	var keyboard tgbotapi.InlineKeyboardMarkup
	if !movie.Monitored {
		keyboard = b.createKeyboard(
			[]string{"Monitor Movie", "Monitor Movie & Search Now", "Delete Movie", "Edit Movie", "\U0001F519"},
			[]string{LibraryMovieMonitor, LibraryMovieMonitorSearchNow, LibraryMovieDelete, LibraryMovieEdit, LibraryMovieGoBack},
		)
	} else {
		keyboard = b.createKeyboard(
			[]string{"Unmonitor Movie", "Search Movie", "Delete Movie", "Edit Movie", "\U0001F519"},
			[]string{LibraryMovieUnmonitor, LibraryMovieSearch, LibraryMovieDelete, LibraryMovieEdit, LibraryMovieGoBack},
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

func (b *Bot) handleLibraryMovieDelete(command *userLibrary) bool {
	messageText := fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n\n", utils.Escape(command.movie.Title), command.movie.ImdbID, command.movie.Year)
	keyboard := b.createKeyboard(
		[]string{"Yes, delete this movie", "\U0001F519"},
		[]string{LibraryMovieDeleteYes, LibraryMovieDeleteNo},
	)
	// Send the message containing movie details along with the keyboard
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		messageText,
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = false
	b.setLibraryState(command.chatID, command)
	b.sendMessage(editMsg)
	return false

}

func (b *Bot) handleLibraryMovieDeleteYes(update tgbotapi.Update, command *userLibrary) bool {
	err := b.RadarrServer.DeleteMovie(command.movie.ID, *starr.True(), *starr.False())
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		b.sendMessage(msg)
		return false
	}
	text := fmt.Sprintf("Movie '%v' deleted\n", command.movie.Title)
	b.clearState(update)
	b.sendMessageWithEdit(command, text)
	return true
}

func (b *Bot) handleLibraryMovieEdit(command *userLibrary) bool {
	b.setLibraryState(command.chatID, command)
	b.setActiveCommand(command.chatID, LibraryMovieEditCommand)
	return b.showLibraryMovieEdit(command)
}

func findQualityProfileByID(qualityProfiles []*radarr.QualityProfile, qualityProfileID int64) *radarr.QualityProfile {
	for _, profile := range qualityProfiles {
		if profile.ID == qualityProfileID {
			return profile
		}
	}
	return nil
}
