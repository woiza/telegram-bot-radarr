package bot

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr/radarr"
)

const (
	LibraryMovieEditToggleMonitor        = "LIBRARY_MOVIE_EDIT_TOGGLE_MONITOR"
	LibraryMovieEditToggleQualityProfile = "LIBRARY_MOVIE_EDIT_TOGGLE_QUALITY_PROFILE"
	LibraryMovieEditSubmitChanges        = "LIBRARY_MOVIE_EDIT_SUBMIT_CHANGES"
	LibraryMovieEditCancel               = "LIBRARY_MOVIE_EDIT_CANCEL"
	LibraryMovieEditGoBack               = "LIBRARY_MOVIE_EDIT_GOBACK"
)

func (b *Bot) libraryMovieEdit(update tgbotapi.Update) bool {
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
	case LibraryMovieEditToggleMonitor:
		return b.handleLibraryMovieEditToggleMonitor(update, command)
	case LibraryMovieEditToggleQualityProfile:
		return b.handleLibraryMovieEditToggleQualityProfile(update, command)
	case LibraryMovieEditGoBack:
		b.setActiveCommand(userID, LibraryFilteredActive)
		b.setLibraryState(command.chatID, command)
		return b.showLibraryMovieDetail(update, command)
	case LibraryMovieEditCancel:
		b.clearState(update)
		b.sendMessageWithEdit(command, CommandsCleared)
		return false
	default:
		return b.showLibraryMovieEdit(update, command)
	}
}

func (b *Bot) showLibraryMovieEdit(update tgbotapi.Update, command *userLibrary) bool {
	movie := command.movie

	var monitorIcon string
	if movie.Monitored {
		monitorIcon = MonitorIcon
	} else {
		monitorIcon = UnmonitorIcon
	}

	//minimumAvailability := movie.MinimumAvailability
	qualityProfile := getQualityProfileByID(command.qualityProfiles, command.movie.QualityProfileID).Name

	var tagLabels []string
	for _, tagID := range movie.Tags {
		tag := findTagByID(command.allTags, tagID)
		tagLabels = append(tagLabels, tag.Label)
	}
	tagsString := strings.Join(tagLabels, ", ")

	messageText := fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n\n", utils.Escape(movie.Title), movie.ImdbID, movie.Year)

	var keyboard tgbotapi.InlineKeyboardMarkup
	if !movie.Monitored {
		keyboard = b.createKeyboard(
			[]string{"Monitored: " + monitorIcon, qualityProfile, tagsString, "Go back - Show Movie Details", "Cancel - clear command"},
			[]string{LibraryMovieEditToggleMonitor, LibraryMovieEditToggleQualityProfile, LibraryMovieEdit, LibraryMovieEditGoBack, LibraryMovieEditCancel},
		)
	} else {
		keyboard = b.createKeyboard(
			[]string{"Monitored: " + monitorIcon, qualityProfile, tagsString, "Go back - Show Movie Details", "Cancel - clear command"},
			[]string{LibraryMovieEditToggleMonitor, LibraryMovieEditToggleQualityProfile, LibraryMovieEdit, LibraryMovieEditGoBack, LibraryMovieEditCancel},
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

func (b *Bot) handleLibraryMovieEditToggleMonitor(update tgbotapi.Update, command *userLibrary) bool {
	command.movie.Monitored = !command.movie.Monitored
	b.setLibraryState(command.chatID, command)
	return b.showLibraryMovieEdit(update, command)
}

func (b *Bot) handleLibraryMovieEditToggleQualityProfile(update tgbotapi.Update, command *userLibrary) bool {
	currentProfileIndex := getQualityProfileIndexByID(command.qualityProfiles, command.movie.QualityProfileID)
	nextProfileIndex := (currentProfileIndex + 1) % len(command.qualityProfiles)
	command.movie.QualityProfileID = command.qualityProfiles[nextProfileIndex].ID
	b.setLibraryState(command.chatID, command)
	return b.showLibraryMovieEdit(update, command)
}

func getQualityProfileByID(qualityProfiles []*radarr.QualityProfile, id int64) *radarr.QualityProfile {
	for _, profile := range qualityProfiles {
		if profile.ID == id {
			return profile
		}
	}
	return nil // Return an appropriate default or handle the error as needed
}

func getQualityProfileIndexByID(qualityProfiles []*radarr.QualityProfile, id int64) int {
	for i, profile := range qualityProfiles {
		if profile.ID == id {
			return i
		}
	}
	return -1 // Return an appropriate default or handle the error as needed
}
