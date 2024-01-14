package bot

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
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
	qualityProfile := findQualityProfileByID(command.qualityProfiles, movie.QualityProfileID).Name

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
			[]string{LibraryMovieMonitor, LibraryMovieMonitorSearchNow, LibraryMovieEdit},
			[]string{"Monitored: " + monitorIcon, qualityProfile, tagsString},
		)
	} else {
		keyboard = b.createKeyboard(
			[]string{LibraryMovieUnmonitor, LibraryMovieSearch, LibraryMovieEdit},
			[]string{"Monitored: " + monitorIcon, qualityProfile, tagsString},
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
