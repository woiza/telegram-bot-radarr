package bot

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

const (
	LibraryMovieEditToggleMonitor        = "LIBRARY_MOVIE_EDIT_TOGGLE_MONITOR"
	LibraryMovieEditToggleQualityProfile = "LIBRARY_MOVIE_EDIT_TOGGLE_QUALITY_PROFILE"
	LibraryMovieEditSubmitChanges        = "LIBRARY_MOVIE_EDIT_SUBMIT_CHANGES"
	LibraryMovieEditGoBack               = "LIBRARY_MOVIE_EDIT_GOBACK"
	LibraryMovieEditCancel               = "LIBRARY_MOVIE_EDIT_CANCEL"
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
	case LibraryMovieEditSubmitChanges:
		return b.handleLibraryMovieEditSubmitChanges(update, command)
	case LibraryMovieEditGoBack:
		b.setActiveCommand(userID, LibraryFilteredActive)
		b.setLibraryState(command.chatID, command)
		return b.showLibraryMovieDetail(update, command)
	case LibraryMovieEditCancel:
		b.clearState(update)
		b.sendMessageWithEdit(command, CommandsCleared)
		return false
	default:
		// Check if it starts with "TAG_"
		if strings.HasPrefix(update.CallbackQuery.Data, "TAG_") {
			return b.handleLibraryMovieEditSelectTag(update, command)
		}
		return b.showLibraryMovieEdit(update, command)
	}
}

func (b *Bot) showLibraryMovieEdit(update tgbotapi.Update, command *userLibrary) bool {
	movie := command.movie

	var monitorIcon string
	if command.selectedMonitoring {
		monitorIcon = MonitorIcon
	} else {
		monitorIcon = UnmonitorIcon
	}

	qualityProfile := getQualityProfileByID(command.qualityProfiles, command.selectedQualityProfile).Name

	messageText := fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n\n", utils.Escape(movie.Title), movie.ImdbID, movie.Year)

	var keyboard tgbotapi.InlineKeyboardMarkup

	keyboard = b.createKeyboard(
		[]string{"Monitored: " + monitorIcon, qualityProfile},
		[]string{LibraryMovieEditToggleMonitor, LibraryMovieEditToggleQualityProfile},
	)

	var tagsKeyboard [][]tgbotapi.InlineKeyboardButton
	for _, tag := range command.allTags {
		// Check if the tag is selected
		isSelected := isSelectedTag(command.selectedTags, tag.ID)

		var buttonText string
		if isSelected {
			buttonText = tag.Label + " \u2705"
		} else {
			buttonText = tag.Label
		}

		row := []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(buttonText, "TAG_"+strconv.Itoa(int(tag.ID))),
		}
		tagsKeyboard = append(tagsKeyboard, row)
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, tagsKeyboard...)

	var keyboardSubmitCancelGoBack tgbotapi.InlineKeyboardMarkup
	keyboardSubmitCancelGoBack = b.createKeyboard(
		[]string{"Submit - Confirm Changes", "Go back - Show Movie Details", "Cancel - clear command"},
		[]string{LibraryMovieEditSubmitChanges, LibraryMovieEditGoBack, LibraryMovieEditCancel},
	)

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, keyboardSubmitCancelGoBack.InlineKeyboard...)

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
	command.selectedMonitoring = !command.selectedMonitoring
	b.setLibraryState(command.chatID, command)
	return b.showLibraryMovieEdit(update, command)
}

func (b *Bot) handleLibraryMovieEditToggleQualityProfile(update tgbotapi.Update, command *userLibrary) bool {
	currentProfileIndex := getQualityProfileIndexByID(command.qualityProfiles, command.selectedQualityProfile)
	nextProfileIndex := (currentProfileIndex + 1) % len(command.qualityProfiles)
	command.selectedQualityProfile = command.qualityProfiles[nextProfileIndex].ID
	b.setLibraryState(command.chatID, command)
	return b.showLibraryMovieEdit(update, command)
}

func (b *Bot) handleLibraryMovieEditSelectTag(update tgbotapi.Update, command *userLibrary) bool {
	tagIDStr := strings.TrimPrefix(update.CallbackQuery.Data, "TAG_")
	// Parse the tag ID
	tagID, err := strconv.Atoi(tagIDStr)
	if err != nil {
		fmt.Printf("Cannot convert tag string to int: %v", err)
		return false
	}

	// Check if the tag is already selected
	if isSelectedTag(command.selectedTags, tagID) {
		// If selected, remove the tag from selectedTags (deselect)
		command.selectedTags = removeTag(command.selectedTags, tagID)
	} else {
		// If not selected, add the tag to selectedTags (select)
		tag := &starr.Tag{ID: tagID} // Create a new starr.Tag with the ID
		command.selectedTags = append(command.selectedTags, tag.ID)
	}

	b.setLibraryState(command.chatID, command)
	return b.showLibraryMovieEdit(update, command)
}

func (b *Bot) handleLibraryMovieEditSubmitChanges(update tgbotapi.Update, command *userLibrary) bool {
	bulkEdit := radarr.BulkEdit{
		MovieIDs:         []int64{command.movie.ID},
		Monitored:        &command.selectedMonitoring,
		QualityProfileID: &command.selectedQualityProfile,
		Tags:             command.selectedTags,
		ApplyTags:        starr.TagsReplace.Ptr(),
	}
	_, err := b.RadarrServer.EditMovies(&bulkEdit)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		b.sendMessage(msg)
		return false
	}
	text := fmt.Sprintf("Movie '%v' updated\n", command.movie.Title)
	b.clearState(update)
	b.sendMessageWithEdit(command, text)
	return false
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

func isSelectedTag(selectedTags []int, tagID int) bool {
	for _, selectedTag := range selectedTags {
		if selectedTag == tagID {
			return true
		}
	}
	return false
}

func removeTag(tags []int, tagID int) []int {
	var updatedTags []int
	for _, tag := range tags {
		if tag != tagID {
			updatedTags = append(updatedTags, tag)
		}
	}
	return updatedTags
}
