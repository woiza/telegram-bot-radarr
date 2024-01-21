package bot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

const (
	AddMovieYes    = "ADDMOVIE_YES"
	AddMovieGoBack = "ADDMOVIE_GOBACK"
	AddMovieCancel = "ADDMOVIE_CANCEL"
)

func (b *Bot) processAddCommand(update tgbotapi.Update, userID int64, r *radarr.Radarr) {
	msg := tgbotapi.NewMessage(userID, "Handling add movie command... please wait")
	message, _ := b.sendMessage(msg)
	command := userAddMovie{}
	command.chatID = message.Chat.ID
	command.messageID = message.MessageID

	criteria := update.Message.CommandArguments()
	if len(criteria) < 1 {
		b.sendMessageWithEdit(&command, "Please provide a search criteria /q [query]")
		return
	}
	searchResults, err := r.Lookup(criteria)
	if err != nil {
		msg := tgbotapi.NewMessage(userID, err.Error())
		b.sendMessage(msg)
		return
	}

	if len(searchResults) == 0 {
		b.sendMessageWithEdit(&command, "No movies found matching your search criteria")
		return
	}
	if len(searchResults) > 25 {
		b.sendMessageWithEdit(&command, "Result size too large, please narrow down your search criteria")
		return
	}

	command.searchResults = make(map[string]*radarr.Movie, len(searchResults))
	for _, movie := range searchResults {
		tmdbID := strconv.Itoa(int(movie.TmdbID))
		command.searchResults[tmdbID] = movie
	}

	b.setAddMovieState(command.chatID, &command)
	b.setActiveCommand(command.chatID, AddMovieCommand)
	b.showAddMovieSearchResults(update, command)
}

func (b *Bot) showAddMovieSearchResults(update tgbotapi.Update, command userAddMovie) bool {

	// Extract movies from the map
	movies := make([]*radarr.Movie, 0, len(command.searchResults))
	for _, movie := range command.searchResults {
		movies = append(movies, movie)
	}

	// Sort movies by year in ascending order
	sort.SliceStable(movies, func(i, j int) bool {
		return movies[i].Year < movies[j].Year
	})

	var buttonLabels []string
	var buttonData []string
	var text strings.Builder
	var responseText string

	for _, movie := range movies {
		text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n", utils.Escape(movie.Title), movie.TmdbID, movie.Year))
		buttonLabels = append(buttonLabels, fmt.Sprintf("%v - %v", movie.Title, movie.Year))
		buttonData = append(buttonData, "TMDBID_"+strconv.Itoa(int(movie.TmdbID)))
	}

	keyboard := b.createKeyboard(buttonLabels, buttonData)

	switch len(command.searchResults) {
	case 1:
		responseText = "*Movie found*\n\n"
	default:
		responseText = fmt.Sprintf("*Found %d movies*\n\n", len(command.searchResults))
	}
	responseText += text.String()

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		responseText,
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = true
	b.setAddMovieState(command.chatID, &command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) addMovieDetails(update tgbotapi.Update, command userAddMovie) bool {
	movieIDStr := strings.TrimPrefix(update.CallbackQuery.Data, "TMDBID_")
	command.movie = command.searchResults[movieIDStr]

	var text strings.Builder
	text.WriteString("Is this the correct movie?\n\n")

	text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n\n", utils.Escape(command.movie.Title), command.movie.ImdbID, command.movie.Year))
	keyboard := b.createKeyboard(
		[]string{"Yes, add this movie", "Cancel, clear command", "\U0001F519"},
		[]string{AddMovieYes, AddMovieCancel, AddMovieGoBack},
	)

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		text.String(),
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = false
	b.setAddMovieState(command.chatID, &command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) addMovie(update tgbotapi.Update) bool {
	userID, err := b.getUserID(update)
	if err != nil {
		fmt.Printf("Cannot add movie: %v", err)
		return false
	}
	command, exists := b.getAddMovieState(userID)
	if !exists {
		return false
	}
	switch update.CallbackQuery.Data {
	case AddMovieYes:
		b.setActiveCommand(userID, AddMovieCommand)
	case AddMovieGoBack:
		//command.movie = nil
		b.setAddMovieState(command.chatID, command)
		b.showAddMovieSearchResults(update, *command)
	case AddMovieCancel:
		b.clearState(update)
		b.sendMessageWithEdit(command, CommandsCleared)
		return false
	default:
		// Check if it starts with "TAG_"
		if strings.HasPrefix(update.CallbackQuery.Data, "TMDBID_") {
			return b.addMovieDetails(update, *command)
		}
		return b.showAddMovieSearchResults(update, *command)
	}

	//command := b.AddMovieStates[userID]

	// if !command.confirmation {
	// 	switch update.CallbackQuery.Data {
	// 	case "ADDMOVIE_YES":
	// 		command.confirmation = true
	// 		//movie already in library...
	// 		if command.movie.ID != 0 {
	// 			b.clearState(update)
	// 			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Movie already exists in your library.\nAll commands have been cleared.")
	// 			b.sendMessage(msg)
	// 			return false
	// 		} else {
	// 			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	// 			profiles, err := b.RadarrServer.GetQualityProfiles()
	// 			if err != nil {
	// 				msg.Text = err.Error()
	// 				fmt.Println(err)
	// 				b.sendMessage(msg)
	// 			}
	// 			if len(profiles) > 1 {
	// 				buttons := make([][]tgbotapi.InlineKeyboardButton, len(profiles))
	// 				for i, profile := range profiles {
	// 					button := tgbotapi.NewInlineKeyboardButtonData(profile.Name, strconv.Itoa(int(profile.ID)))
	// 					buttons[i] = tgbotapi.NewInlineKeyboardRow(button)
	// 				}
	// 				msg.Text = "Please choose your quality profile"
	// 				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	// 				b.AddMovieStates[userID] = command
	// 				b.sendMessage(msg)
	// 				return false
	// 			} else if len(profiles) == 1 {
	// 				profileID := profiles[0].ID
	// 				update.CallbackQuery.Data = strconv.FormatInt(profileID, 10)
	// 			} else {
	// 				b.AddMovieStates[userID] = command
	// 				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, utils.Escape("No quality profile(s) found on your radarr server.\nAll commands have been cleared."))
	// 				b.clearState(update)
	// 				b.sendMessage(msg)
	// 				return false
	// 			}
	// 		}
	// 	case "ADDMOVIE_CANCEL":
	// 		b.clearState(update)
	// 		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
	// 		b.sendMessage(msg)
	// 		return false
	// 	// ADDMOVIE_NO is the same as the default
	// 	default:
	// 		command.confirmation = false
	// 		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	// 		command.movie = nil
	// 		b.AddMovieStates[userID] = command
	// 		b.sendSearchResults(command.searchResults, &msg)
	// 		return false
	// 	}
	// }
	// if command.profileID == nil {
	// 	profileID, _ := strconv.Atoi(update.CallbackQuery.Data)
	// 	command.movie.QualityProfileID = int64(profileID)
	// 	command.profileID = &command.movie.QualityProfileID

	// 	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	// 	rootFolders, err := b.RadarrServer.GetRootFolders()
	// 	if err != nil {
	// 		msg.Text = err.Error()
	// 		fmt.Println(err)
	// 		b.sendMessage(msg)
	// 		return false
	// 	}

	// 	buttons := make([][]tgbotapi.InlineKeyboardButton, len(rootFolders))
	// 	if len(rootFolders) > 1 {
	// 		for i, folder := range rootFolders {
	// 			path := folder.Path
	// 			button := tgbotapi.NewInlineKeyboardButtonData(path, path)
	// 			buttons[i] = tgbotapi.NewInlineKeyboardRow(button)
	// 		}

	// 		b.AddMovieStates[userID] = command
	// 		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, utils.Escape(fmt.Sprintf("Please choose the root folder for '%v'\n", command.movie.Title)))
	// 		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	// 		b.sendMessage(msg)
	// 		return false
	// 	} else if len(rootFolders) == 1 {
	// 		update.CallbackQuery.Data = rootFolders[0].Path
	// 	} else {
	// 		b.AddMovieStates[userID] = command
	// 		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, utils.Escape("No root folder(s) found on your radarr server.\nAll commands have been cleared."))
	// 		b.clearState(update)
	// 		b.sendMessage(msg)
	// 		return false
	// 	}
	// }

	// if command.allTags == nil {
	// 	command.movie.Path = update.CallbackQuery.Data // there is no rootFolderPath in movie struct --> misuse path
	// 	command.path = &command.movie.Path

	// 	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Select tag(s) you want to add:\n")
	// 	// Fetch tags from Radarr server.
	// 	tags, err := b.RadarrServer.GetTags()
	// 	if err != nil {
	// 		msg.Text = err.Error()
	// 		fmt.Println(err)
	// 		b.sendMessage(msg)
	// 		return false
	// 	}

	// 	if len(tags) == 0 {
	// 		update.CallbackQuery.Data = "DONE_ADDING_TAGS"
	// 		command.tagDone = true
	// 	}

	// 	// Sort tags by label
	// 	sort.Slice(tags, func(i, j int) bool {
	// 		return tags[i].Label < tags[j].Label
	// 	})

	// 	command.allTags = tags

	// 	buttons := make([][]tgbotapi.InlineKeyboardButton, len(tags)+1) // +1 for the "Done" button
	// 	for i, tag := range tags {
	// 		button := tgbotapi.NewInlineKeyboardButtonData(tag.Label, "TAG_"+strconv.Itoa(tag.ID))
	// 		buttons[i] = []tgbotapi.InlineKeyboardButton{button}
	// 	}

	// 	// Add a "Done" button for user confirmation.
	// 	doneButton := tgbotapi.NewInlineKeyboardButtonData("Done - continue", "DONE_ADDING_TAGS")
	// 	buttons[len(tags)] = []tgbotapi.InlineKeyboardButton{doneButton}

	// 	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	// 	b.sendMessage(msg)
	// 	b.AddMovieStates[userID] = command // Update the user's state
	// 	return false
	// }

	// if !command.tagDone {
	// 	switch update.CallbackQuery.Data {
	// 	case "DONE_ADDING_TAGS":
	// 		command.tagDone = true
	// 	default:
	// 		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	// 		tags := command.allTags

	// 		tagIDStr := strings.TrimPrefix(update.CallbackQuery.Data, "TAG_")
	// 		tagID, convErr := strconv.ParseInt(tagIDStr, 10, 64)
	// 		if convErr != nil {
	// 			b.clearState(update)
	// 			b.sendMessage(msg)
	// 			return false
	// 		}

	// 		// Check if the tag is already selected, if so, deselect it, otherwise, select it.
	// 		tagIndex := findTagIndex(command.selectedTags, int(tagID))
	// 		selectedTag := findTagByID(command.allTags, int(tagID))

	// 		if selectedTag != nil {
	// 			if tagIndex > -1 {
	// 				command.selectedTags = removeTagByID(command.selectedTags, tagIndex)
	// 			} else {
	// 				command.selectedTags = append(command.selectedTags, selectedTag)
	// 			}
	// 		}

	// 		// Sort tags by label
	// 		sort.Slice(tags, func(i, j int) bool {
	// 			return tags[i].Label < tags[j].Label
	// 		})

	// 		// Update the keyboard to reflect the changes.
	// 		buttons := make([][]tgbotapi.InlineKeyboardButton, len(tags)+1) // +1 for the "Done" button
	// 		for i, tag := range tags {
	// 			var buttonText string
	// 			if tagIndex := findTagIndex(command.selectedTags, tag.ID); tagIndex > -1 {
	// 				// Add a green check mark to indicate selected tags
	// 				buttonText = tag.Label + " " + "\u2705"
	// 			} else {
	// 				buttonText = tag.Label
	// 			}
	// 			button := tgbotapi.NewInlineKeyboardButtonData(buttonText, "TAG_"+strconv.Itoa(int(tag.ID)))
	// 			buttons[i] = []tgbotapi.InlineKeyboardButton{button}
	// 		}

	// 		// Add a "Done" button for user confirmation.
	// 		doneButton := tgbotapi.NewInlineKeyboardButtonData("Done - continue", "DONE_ADDING_TAGS")
	// 		buttons[len(tags)] = []tgbotapi.InlineKeyboardButton{doneButton}

	// 		// Update the message with the revised keyboard markup.
	// 		msg = tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Select tag(s) you want to add:")
	// 		//msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)

	// 		editedMessage := tgbotapi.NewEditMessageTextAndMarkup(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, msg.Text, tgbotapi.NewInlineKeyboardMarkup(buttons...))

	// 		b.sendMessage(editedMessage)

	// 		b.AddMovieStates[userID] = command
	// 		return false

	// 	}
	// }

	// if !command.movieAdded {
	// 	var addMovieInput radarr.AddMovieInput
	// 	var addOptions radarr.AddMovieOptions
	// 	switch update.CallbackQuery.Data {
	// 	case "MOVIE_MONSEA":
	// 		addMovieInput.Monitored = *starr.True()
	// 		addOptions = radarr.AddMovieOptions{
	// 			SearchForMovie: *starr.True(),
	// 			Monitor:        "movieOnly",
	// 		}
	// 		addMovie(*command, addMovieInput, addOptions, update, b)
	// 	case "MOVIE_MON":
	// 		addMovieInput.Monitored = *starr.True()
	// 		addOptions = radarr.AddMovieOptions{
	// 			SearchForMovie: *starr.False(),
	// 			Monitor:        "movieOnly",
	// 		}
	// 		addMovie(*command, addMovieInput, addOptions, update, b)
	// 	case "MOVIE_UNMON":
	// 		addMovieInput.Monitored = *starr.False()
	// 		addOptions = radarr.AddMovieOptions{
	// 			SearchForMovie: *starr.False(),
	// 			Monitor:        "none",
	// 		}
	// 		addMovie(*command, addMovieInput, addOptions, update, b)
	// 	case "COLLECTION_MONSEA":
	// 		addMovieInput.Monitored = *starr.True()
	// 		addOptions = radarr.AddMovieOptions{
	// 			SearchForMovie: *starr.True(),
	// 			Monitor:        "movieAndCollection",
	// 		}
	// 		addMovie(*command, addMovieInput, addOptions, update, b)
	// 	case "COLLECTION_MON":
	// 		addMovieInput.Monitored = *starr.True()
	// 		addOptions = radarr.AddMovieOptions{
	// 			SearchForMovie: *starr.False(),
	// 			Monitor:        "movieAndCollection",
	// 		}
	// 		addMovie(*command, addMovieInput, addOptions, update, b)
	// 	case "MONITORED_CANCEL":
	// 		b.clearState(update)
	// 		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
	// 		b.sendMessage(msg)
	// 	default:
	// 		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "How would you like to add the movie?\n")
	// 		buttons := make([][]tgbotapi.InlineKeyboardButton, 6)
	// 		buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Add movie monitored + search now", "MOVIE_MONSEA"))
	// 		buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Add movie monitored", "MOVIE_MON"))
	// 		buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Add movie unmonitored", "MOVIE_UNMON"))
	// 		buttons[3] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Add collection monitored + search now", "COLLECTION_MONSEA"))
	// 		buttons[4] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Add collection monitored", "COLLECTION_MON"))
	// 		buttons[5] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "MONITORED_CANCEL"))

	// 		b.AddMovieStates[userID] = command
	// 		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	// 		b.sendMessage(msg)
	// 	}
	// }
	return false
}

// Helper functions to manage tag selections.
func findTagIndex(tags []*starr.Tag, tagID int) int {
	for i, tag := range tags {
		if tag.ID == tagID {
			return i
		}
	}
	return -1
}

func findTagByID(tags []*starr.Tag, tagID int) *starr.Tag {
	for _, tag := range tags {
		if int(tag.ID) == tagID {
			return tag
		}
	}
	return nil
}

func removeTagByID(tags []*starr.Tag, index int) []*starr.Tag {
	copy(tags[index:], tags[index+1:])
	return tags[:len(tags)-1]
}

func addMovie(command userAddMovie, addMovieInput radarr.AddMovieInput, addOptions radarr.AddMovieOptions, update tgbotapi.Update, b *Bot) bool {
	var tagIDs []int
	for _, tag := range command.selectedTags {
		tagIDs = append(tagIDs, tag.ID)
	}

	addMovieInput.TmdbID = command.movie.TmdbID
	addMovieInput.Title = command.movie.Title
	addMovieInput.QualityProfileID = *command.profileID
	addMovieInput.RootFolderPath = *command.path
	addMovieInput.AddOptions = &addOptions
	addMovieInput.Tags = tagIDs

	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
	var _, err = b.RadarrServer.AddMovie(&addMovieInput)
	if err != nil {
		msg.Text = err.Error()
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}
	movies, err := b.RadarrServer.GetMovie((command.movie.TmdbID))
	if err != nil {
		msg.Text = err.Error()
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}

	if addOptions.Monitor == "movieAndCollection" {
		msg.Text = fmt.Sprintf("Collection '%v' added\n", movies[0].Title)
	} else {
		msg.Text = fmt.Sprintf("Movie '%v' added\n", movies[0].Title)
	}
	b.clearState(update)
	b.sendMessage(msg)
	return true
}
