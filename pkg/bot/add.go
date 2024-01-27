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
	AddMovieYes      = "ADDMOVIE_YES"
	AddMovieGoBack   = "ADDMOVIE_GOBACK"
	AddMovieCancel   = "ADDMOVIE_CANCEL"
	AddMovieTagsDone = "ADDMOVIE_TAGS_DONE"
	AddMovieMonSea   = "ADDMOVIE_MONSEA"
	AddMovieMon      = "ADDMOVIE_MON"
	AddMovieUnMon    = "ADDMOVIE_UNMON"
	AddMovieColSea   = "ADDMOVIE_COLSEA"
	AddMovieColMon   = "ADDMOVIE_COLMON"
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
		b.handleAddMovieYes(update, *command)
	case AddMovieGoBack:
		b.setAddMovieState(command.chatID, command)
		b.showAddMovieSearchResults(update, *command)
	case AddMovieCancel:
		b.clearState(update)
		b.sendMessageWithEdit(command, CommandsCleared)
		return false
	case AddMovieTagsDone:
		b.showAddMovieAddOptions(update, *command)
	case AddMovieMonSea:
		b.handleAddMovieMonSea(update, *command)
	case AddMovieMon:
		b.handleAddMovieMon(update, *command)
	case AddMovieUnMon:
		b.handleAddMovieUnMon(update, *command)
	case AddMovieColSea:
		b.handleAddMovieColSea(update, *command)
	case AddMovieColMon:
		b.handleAddMovieColMon(update, *command)
	default:
		// Check if it starts with "PROFILE_"
		if strings.HasPrefix(update.CallbackQuery.Data, "PROFILE_") {
			return b.handleAddMovieProfile(update, *command)
		}
		// Check if it starts with "PROFILE_"
		if strings.HasPrefix(update.CallbackQuery.Data, "ROOTFOLDER_") {
			return b.handleAddMovieRootFolder(update, *command)
		}
		// Check if it starts with "TAG_"
		if strings.HasPrefix(update.CallbackQuery.Data, "TAG_") {
			return b.handleAddMovieEditSelectTag(update, command)
		}
		// Check if it starts with "TMDBID_"
		if strings.HasPrefix(update.CallbackQuery.Data, "TMDBID_") {
			return b.addMovieDetails(update, *command)
		}
		return b.showAddMovieSearchResults(update, *command)
	}
	return false
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
	keyboardCancel := b.createKeyboard(
		[]string{"Cancel - clear command"},
		[]string{AddMovieCancel},
	)
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, keyboardCancel.InlineKeyboard...)

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

func (b *Bot) handleAddMovieYes(update tgbotapi.Update, command userAddMovie) bool {
	//movie already in library...
	if command.movie.ID != 0 {
		b.sendMessageWithEdit(&command, "Movie already in library\nAll commands have been cleared")
		return false
	}

	profiles, err := b.RadarrServer.GetQualityProfiles()
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}
	if len(profiles) == 0 {
		b.sendMessageWithEdit(&command, "No quality profile(s) found on your radarr server.\nAll commands have been cleared.")
		b.clearState(update)
	}
	if len(profiles) == 1 {
		command.profileID = profiles[0].ID
	}
	command.allProfiles = profiles

	rootFolders, err := b.RadarrServer.GetRootFolders()
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}
	if len(rootFolders) == 1 {
		command.rootFolder = rootFolders[0].Path
	}
	if len(rootFolders) == 0 {
		b.sendMessageWithEdit(&command, "No root folder(s) found on your radarr server.\nAll commands have been cleared.")
		b.clearState(update)
	}
	command.allRootFolders = rootFolders

	tags, err := b.RadarrServer.GetTags()
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}
	command.allTags = tags

	b.setAddMovieState(command.chatID, &command)
	return b.showAddMovieProfiles(update, command)
}

func (b *Bot) showAddMovieProfiles(update tgbotapi.Update, command userAddMovie) bool {
	// If there is only one profile, skip this step
	if len(command.allProfiles) == 1 {
		return b.showAddMovieRootFolders(update, command)
	}
	var profileKeyboard [][]tgbotapi.InlineKeyboardButton
	for _, profile := range command.allProfiles {
		row := []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(profile.Name, "PROFILE_"+strconv.Itoa(int(profile.ID))),
		}
		profileKeyboard = append(profileKeyboard, row)
	}

	var messageText strings.Builder
	var keyboard tgbotapi.InlineKeyboardMarkup
	keyboardCancelGoBack := b.createKeyboard(
		[]string{"Cancel - clear command", "\U0001F519"},
		[]string{AddMovieCancel, AddMovieGoBack},
	)
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, profileKeyboard...)
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, keyboardCancelGoBack.InlineKeyboard...)
	messageText.WriteString("Select quality profile:")
	b.sendMessageWithEditAndKeyboard(
		&command,
		keyboard,
		messageText.String(),
	)
	return false
}

func (b *Bot) handleAddMovieProfile(update tgbotapi.Update, command userAddMovie) bool {
	profileIDStr := strings.TrimPrefix(update.CallbackQuery.Data, "PROFILE_")
	// Parse the profile ID
	profileID, err := strconv.Atoi(profileIDStr)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}
	command.profileID = int64(profileID)
	b.setAddMovieState(command.chatID, &command)
	return b.showAddMovieRootFolders(update, command)
}

func (b *Bot) showAddMovieRootFolders(update tgbotapi.Update, command userAddMovie) bool {
	// If there is only one root folder, skip this step
	if len(command.allRootFolders) == 1 {
		return b.showAddMovieTags(update, &command)
	}
	var rootFolderKeyboard [][]tgbotapi.InlineKeyboardButton
	for _, rootFolder := range command.allRootFolders {
		row := []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(rootFolder.Path, "ROOTFOLDER_"+rootFolder.Path),
		}
		rootFolderKeyboard = append(rootFolderKeyboard, row)
	}

	var messageText strings.Builder
	var keyboard tgbotapi.InlineKeyboardMarkup
	keyboardCancelGoBack := b.createKeyboard(
		[]string{"Cancel - clear command", "\U0001F519"},
		[]string{AddMovieCancel, AddMovieGoBack},
	)
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, rootFolderKeyboard...)
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, keyboardCancelGoBack.InlineKeyboard...)
	messageText.WriteString("Select root folder:")
	b.sendMessageWithEditAndKeyboard(
		&command,
		keyboard,
		messageText.String(),
	)
	return false

}

func (b *Bot) handleAddMovieRootFolder(update tgbotapi.Update, command userAddMovie) bool {
	command.rootFolder = strings.TrimPrefix(update.CallbackQuery.Data, "ROOTFOLDER_")
	b.setAddMovieState(command.chatID, &command)
	return b.showAddMovieTags(update, &command)
}

func (b *Bot) showAddMovieTags(update tgbotapi.Update, command *userAddMovie) bool {
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
	var keyboard tgbotapi.InlineKeyboardMarkup
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, tagsKeyboard...)

	var keyboardSubmitCancelGoBack tgbotapi.InlineKeyboardMarkup
	keyboardSubmitCancelGoBack = b.createKeyboard(
		[]string{"Done - Continue", "Cancel - clear command", "\U0001F519"},
		[]string{AddMovieTagsDone, AddMovieCancel, AddMovieGoBack},
	)

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, keyboardSubmitCancelGoBack.InlineKeyboard...)

	// Send the message containing movie details along with the keyboard
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		"Select tags:",
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = true
	b.setAddMovieState(command.chatID, command)
	b.sendMessage(editMsg)
	return false

}

func (b *Bot) handleAddMovieEditSelectTag(update tgbotapi.Update, command *userAddMovie) bool {
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

	b.setAddMovieState(command.chatID, command)
	return b.showAddMovieTags(update, command)
}

func (b *Bot) showAddMovieAddOptions(update tgbotapi.Update, command userAddMovie) bool {
	keyboard := b.createKeyboard(
		[]string{"Add movie monitored + search now", "Add movie monitored", "Add movie unmonitored", "Add collection monitored + search now", "Add collection monitored", "Cancel, clear command"},
		[]string{AddMovieMonSea, AddMovieMon, AddMovieUnMon, AddMovieColSea, AddMovieColMon, AddMovieCancel},
	)
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		"How would you like to add the movie?\n",
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = true
	b.setAddMovieState(command.chatID, &command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) handleAddMovieMonSea(update tgbotapi.Update, command userAddMovie) bool {
	command.monitored = *starr.True()
	command.addMovieOptions = &radarr.AddMovieOptions{
		SearchForMovie: *starr.True(),
		Monitor:        "movieOnly",
	}
	b.setAddMovieState(command.chatID, &command)
	return b.addMovieToLibrary(update, command)
}

func (b *Bot) handleAddMovieMon(update tgbotapi.Update, command userAddMovie) bool {
	command.monitored = *starr.True()
	command.addMovieOptions = &radarr.AddMovieOptions{
		SearchForMovie: *starr.False(),
		Monitor:        "movieOnly",
	}
	b.setAddMovieState(command.chatID, &command)
	return b.addMovieToLibrary(update, command)
}

func (b *Bot) handleAddMovieUnMon(update tgbotapi.Update, command userAddMovie) bool {
	command.monitored = *starr.False()
	command.addMovieOptions = &radarr.AddMovieOptions{
		SearchForMovie: *starr.False(),
		Monitor:        "none",
	}
	b.setAddMovieState(command.chatID, &command)
	return b.addMovieToLibrary(update, command)
}

func (b *Bot) handleAddMovieColSea(update tgbotapi.Update, command userAddMovie) bool {
	command.monitored = *starr.True()
	command.addMovieOptions = &radarr.AddMovieOptions{
		SearchForMovie: *starr.True(),
		Monitor:        "movieAndCollection",
	}
	b.setAddMovieState(command.chatID, &command)
	return b.addMovieToLibrary(update, command)
}

func (b *Bot) handleAddMovieColMon(update tgbotapi.Update, command userAddMovie) bool {
	command.monitored = *starr.True()
	command.addMovieOptions = &radarr.AddMovieOptions{
		SearchForMovie: *starr.False(),
		Monitor:        "movieAndCollection",
	}
	b.setAddMovieState(command.chatID, &command)
	return b.addMovieToLibrary(update, command)
}

func (b *Bot) addMovieToLibrary(update tgbotapi.Update, command userAddMovie) bool {
	var addMovieInput radarr.AddMovieInput
	var tagIDs []int
	for _, tag := range command.selectedTags {
		tagIDs = append(tagIDs, tag)
	}
	// does anyone ever user anything other than announced?
	addMovieInput.MinimumAvailability = "announced"
	addMovieInput.TmdbID = command.movie.TmdbID
	addMovieInput.Title = command.movie.Title
	addMovieInput.QualityProfileID = command.profileID
	addMovieInput.RootFolderPath = command.rootFolder
	addMovieInput.AddOptions = command.addMovieOptions
	addMovieInput.Tags = tagIDs
	addMovieInput.Monitored = command.monitored

	var messageText string
	var _, err = b.RadarrServer.AddMovie(&addMovieInput)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}
	movies, err := b.RadarrServer.GetMovie((command.movie.TmdbID))
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}

	if command.addMovieOptions.Monitor == "movieAndCollection" {
		messageText = fmt.Sprintf("Collection '%v' added\n", movies[0].Title)
	} else {
		messageText = fmt.Sprintf("Movie '%v' added\n", movies[0].Title)
	}
	b.sendMessageWithEdit(&command, messageText)
	b.clearState(update)
	return true
}
