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
	command.filtered = nil
	command.chatID = message.Chat.ID
	command.messageID = message.MessageID

	b.setLibraryState(userID, &command)
	b.showLibraryMenu(update, &command)
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
	case command.filtered == nil:
		return b.processLibrary(update, command)
	case update.CallbackQuery.Data == "LIBRARY_MENU":
		return b.processLibrary(update, command)
	case command.movie == nil:
		return b.processLibraryMovieSelection(update, command)
	case !command.confirmation:
		return b.processLibraryMovieConfig(update, command)
	// case !command.confirmation:
	// 	return b.processConfirmationForDelete(update, command)
	default:
		return true
	}
}

func (b *Bot) showLibraryMenu(update tgbotapi.Update, command *userLibrary) bool {
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
			tgbotapi.NewInlineKeyboardButtonData("Movies on Disk", "LIBRARY_ONDISK"),
			tgbotapi.NewInlineKeyboardButtonData("All Movies", "LIBRARY_ALL"),
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
	case "LIBRARY_ONDISK":
		for _, movie := range command.library {
			if movie.SizeOnDisk > 0 {
				filtered = append(filtered, movie)
			}
		}
		responseText = "Movies on disk:"
	case "LIBRARY_ALL":
		for _, movie := range command.library {
			filtered = append(filtered, movie)
		}
		responseText = "All Movies:"
	case "LIBRARY_CANCEL":
		b.clearState(update)
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"All commands have been cleared",
		)
		b.sendMessage(editMsg)
		return false
	case "LIBRARY_MENU":
		command.filtered = nil
		command.movie = nil
		b.setLibraryState(command.chatID, command)
		b.showLibraryMenu(update, command)
		return false
	default:
		command.filtered = nil
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
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("Go back - Show library menu", "LIBRARY_MENU"))
	inlineKeyboard = append(inlineKeyboard, row)
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		responseText,
		tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	)
	command.filtered = make(map[string]*radarr.Movie, len(filtered))
	for _, movie := range filtered {
		tmdbID := strconv.Itoa(int(movie.TmdbID))
		command.filtered[tmdbID] = movie
	}
	b.setLibraryState(command.chatID, command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) processLibraryMovieSelection(update tgbotapi.Update, command *userLibrary) bool {
	movie := command.library[update.CallbackQuery.Data]
	command.movie = movie

	var monitorIcon string
	if movie.Monitored {
		monitorIcon = "\u2705" // Green checkmark
	} else {
		monitorIcon = "\u274C" // Red X
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
	messageText += fmt.Sprintf("Status: %s\n", utils.Escape(movie.Status))
	messageText += fmt.Sprintf("Language: %s\n", utils.Escape(movie.OriginalLanguage.Name))
	messageText += fmt.Sprintf("Size: %d GB\n", movie.SizeOnDisk/(1024*1024*1024))
	messageText += fmt.Sprintf("Tags: %s\n", utils.Escape(tagsString))
	messageText += fmt.Sprintf("Quality Profile: %s\n", utils.Escape(findQualityProfileByID(command.qualityProfiles, movie.QualityProfileID).Name))

	// //Create buttons for movie tags
	// var tagButtons [][]tgbotapi.InlineKeyboardButton
	// for _, tag := range movie.Tags {
	// 	var buttonText string
	// 	if tag == 1 { // Replace "DesiredTag" with the actual desired tag condition
	// 		buttonText = "\u2705" // Green checkmark
	// 	} else {
	// 		buttonText = "\u274C" // Red X
	// 	}
	// 	button := tgbotapi.NewInlineKeyboardButtonData(buttonText, "TAG_"+strconv.Itoa(int(tag)))
	// 	row := []tgbotapi.InlineKeyboardButton{button}
	// 	tagButtons = append(tagButtons, row)
	// }

	// // Append "Done" button at the end
	// doneButton := tgbotapi.NewInlineKeyboardButtonData("Done", "DONE")
	// row := []tgbotapi.InlineKeyboardButton{doneButton}
	// tagButtons = append(tagButtons, row)

	// //Create inline keyboard markup
	// inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(tagButtons...)

	var keyboard tgbotapi.InlineKeyboardMarkup
	if !movie.Monitored {
		buttons := make([][]tgbotapi.InlineKeyboardButton, 5)
		buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Monitor Movie", "LIBRARY_MOVIE_MONITOR"))
		buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Monitor Movie & Search Now", "LIBRARY_MOVIE_MONITOR_SEARCHNOW"))
		buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Edit Movie", "LIBRARY_MOVIE_EDIT"))
		buttons[3] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Go back - Show Movies", "LIBRARY_MOVIE_GOBACK"))
		buttons[4] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "LIBRARY_MOVIE_CANCEL"))
		keyboard = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	} else {
		buttons := make([][]tgbotapi.InlineKeyboardButton, 5)
		buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Unmonitor Movie", "LIBRARY_MOVIE_UNMONITOR"))
		buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Search Movie", "LIBRARY_MOVIE_SEARCH"))
		buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Edit Movie", "LIBRARY_MOVIE_EDIT"))
		buttons[3] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Go back - Show Movies", "LIBRARY_MOVIE_GOBACK"))
		buttons[4] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "LIBRARY_MOVIE_CANCEL"))
		keyboard = tgbotapi.NewInlineKeyboardMarkup(buttons...)
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

func (b *Bot) processLibraryMovieConfig(update tgbotapi.Update, command *userLibrary) bool {
	switch update.CallbackQuery.Data {
	// case "LIBRARY_MOVIE_MONITOR":
	// 	return b.handleLibraryMovieMonitor(update, command)
	// case "LIBRARY_MOVIE_MONITOR_SEARCHNOW":
	// 	return b.handleLibraryMovieMonitorAndSearchNow(update, command)
	// case "LIBRARY_MOVIE_EDIT":
	// 	return b.handleLibraryMovieEdit(update, command)
	case "LIBRARY_MOVIE_GOBACK":
		//update.CallbackQuery.Data = "LIBRARY_MENU"
		return b.handleLibraryMovieGoBack(update, command)
	case "LIBRARY_MOVIE_CANCEL":
		b.clearState(update)
		return false
	default:
		command.confirmation = false
		command.movie = nil
		b.setLibraryState(command.chatID, command)
		return false
	}
}

// func (b *Bot) handleLibraryGoBack(update tgbotapi.Update, command *userLibrary) bool {
// 	command.confirmation = false
// 	b.setLibraryState(command.chatID, command)
// 	return false
// }

func findQualityProfileByID(qualityProfiles []*radarr.QualityProfile, qualityProfileID int64) *radarr.QualityProfile {
	for _, profile := range qualityProfiles {
		if profile.ID == qualityProfileID {
			return profile
		}
	}
	return nil
}

func (b *Bot) handleLibraryMovieGoBack(update tgbotapi.Update, command *userLibrary) bool {
	filtered := command.filtered
	command.confirmation = false
	command.movie = nil
	movies := make([]*radarr.Movie, 0, len(filtered))
	for _, value := range filtered {
		movies = append(movies, value)
	}

	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})

	inlineKeyboard := b.getMoviesAsInlineKeyboard(movies)
	var row []tgbotapi.InlineKeyboardButton
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("Go back - Show library menu", "LIBRARY_MENU"))
	inlineKeyboard = append(inlineKeyboard, row)
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		"Filtered Movies:",
		tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	)
	b.setLibraryState(command.chatID, command)
	b.sendMessage(editMsg)
	return false
}
