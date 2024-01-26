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
	DeleteMovieConfirm = "DELETEMOVIE_SUBMIT"
	DeleteMovieCancel  = "DELETEMOVIE_CANCEL"
	DeleteMovieGoBack  = "DELETEMOVIE_GOBACK"
	DeleteMovieYes     = "DELETEMOVIE_YES"
)

func (b *Bot) processDeleteCommand(update tgbotapi.Update, userID int64, r *radarr.Radarr) {
	msg := tgbotapi.NewMessage(userID, "Handling delete command... please wait")
	message, _ := b.sendMessage(msg)

	movies, err := r.GetMovie(0)
	if err != nil {
		msg := tgbotapi.NewMessage(userID, err.Error())
		b.sendMessage(msg)
		return
	}
	command := userDeleteMovie{
		library: make(map[string]*radarr.Movie, len(movies)),
	}
	for _, movie := range movies {
		tmdbID := strconv.Itoa(int(movie.TmdbID))
		command.library[tmdbID] = movie
	}
	command.chatID = message.Chat.ID
	command.messageID = message.MessageID
	b.setDeleteMovieState(userID, &command)

	criteria := update.Message.CommandArguments()
	// no search criteria --> show complete library and return
	if len(criteria) < 1 {
		b.showDeleteMovieSelection(update, &command)
		return
	}

	searchResults, err := r.Lookup(criteria)
	if err != nil {
		msg := tgbotapi.NewMessage(userID, err.Error())
		b.sendMessage(msg)
		return
	}

	b.setDeleteMovieState(userID, &command)
	b.handleDeleteSearchResults(update, searchResults, &command)
	return

}
func (b *Bot) deleteMovie(update tgbotapi.Update) bool {
	userID, err := b.getUserID(update)
	if err != nil {
		fmt.Printf("Cannot delete movie: %v", err)
		return false
	}

	command, exists := b.getDeleteMovieState(userID)
	if !exists {
		return false
	}

	switch update.CallbackQuery.Data {
	case DeleteMovieConfirm:
		return b.processMovieSelectionForDelete(update, command)
	case DeleteMovieYes:
		return b.handleDeleteMovieYes(update, command)
	case DeleteMovieGoBack:
		return b.showDeleteMovieSelection(update, command)
	case DeleteMovieCancel:
		b.clearState(update)
		b.sendMessageWithEdit(command, CommandsCleared)
		return false
	default:
		// Check if it starts with "TMDBID_"
		if strings.HasPrefix(update.CallbackQuery.Data, "TMDBID_") {
			return b.handleLDeleteMovieSelection(update, command)
		}
		return false
	}
}

func (b *Bot) showDeleteMovieSelection(update tgbotapi.Update, command *userDeleteMovie) bool {
	var keyboard tgbotapi.InlineKeyboardMarkup

	// Convert the map values (movies) to a slice
	var movies []*radarr.Movie
	moviesLibrary := command.library
	if len(command.searchResultsInLibrary) > 0 {
		moviesLibrary = command.searchResultsInLibrary
	}
	for _, movie := range moviesLibrary {
		movies = append(movies, movie)
	}

	// Sort the movies alphabetically based on their titles
	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})

	var movieKeyboard [][]tgbotapi.InlineKeyboardButton
	for _, movie := range movies {
		// Check if the movie is selected
		isSelected := isSelectedMovie(command.selectedMovies, movie.ID)

		// Create button text with or without check mark
		buttonText := movie.Title
		if isSelected {
			buttonText += " \u2705"
		}

		row := []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(buttonText, "TMDBID_"+strconv.Itoa(int(movie.TmdbID))),
		}
		movieKeyboard = append(movieKeyboard, row)
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, movieKeyboard...)

	var keyboardConfirmCancel tgbotapi.InlineKeyboardMarkup
	if len(command.selectedMovies) > 0 {
		keyboardConfirmCancel = b.createKeyboard(
			[]string{"Submit - Confirm Movies", "Cancel - clear command"},
			[]string{DeleteMovieConfirm, DeleteMovieCancel},
		)
	} else {
		keyboardConfirmCancel = b.createKeyboard(
			[]string{"Cancel - clear command"},
			[]string{DeleteMovieCancel},
		)
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, keyboardConfirmCancel.InlineKeyboard...)

	// Send the message containing movie details along with the keyboard
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		"Select the movie\\(s\\) you want to delete",
		keyboard,
	)
	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = true
	b.setDeleteMovieState(command.chatID, command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) handleDeleteSearchResults(update tgbotapi.Update, searchResults []*radarr.Movie, command *userDeleteMovie) {
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

	if len(moviesInLibrary) == 1 {
		command.selectedMovies = make([]*radarr.Movie, len(moviesInLibrary))
		command.selectedMovies[0] = moviesInLibrary[0]
		b.setDeleteMovieState(command.chatID, command)
		b.processMovieSelectionForDelete(update, command)
	} else {
		b.setDeleteMovieState(command.chatID, command)
		b.showDeleteMovieSelection(update, command)
	}
}
func (b *Bot) processMovieSelectionForDelete(update tgbotapi.Update, command *userDeleteMovie) bool {
	var keyboard tgbotapi.InlineKeyboardMarkup
	var messageText strings.Builder
	var disablePreview bool
	switch len(command.selectedMovies) {
	case 1:
		keyboard = b.createKeyboard(
			[]string{"Yes, delete this movie", "Cancel, clear command", "\U0001F519"},
			[]string{DeleteMovieYes, DeleteMovieCancel, DeleteMovieGoBack},
		)
		messageText.WriteString("Do you want to delete the following movie including all files?\n\n")
		messageText.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n",
			utils.Escape(command.selectedMovies[0].Title), command.selectedMovies[0].ImdbID, command.selectedMovies[0].Year))
		disablePreview = false
	case 0:
		return b.showDeleteMovieSelection(update, command)
	default:
		keyboard = b.createKeyboard(
			[]string{"Yes, delete these movies", "Cancel, clear command", "\U0001F519"},
			[]string{DeleteMovieYes, DeleteMovieCancel, DeleteMovieGoBack},
		)
		messageText.WriteString("Do you want to delete the following movies including all files?\n\n")
		for _, movie := range command.selectedMovies {
			messageText.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n",
				utils.Escape(movie.Title), movie.ImdbID, movie.Year))
		}
		disablePreview = true
	}

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		messageText.String(),
		keyboard,
	)

	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = disablePreview
	editMsg.ReplyMarkup = &keyboard

	b.setDeleteMovieState(command.chatID, command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) handleDeleteMovieYes(update tgbotapi.Update, command *userDeleteMovie) bool {
	var movieIDs []int64
	var deletedMovies []string
	for _, movie := range command.selectedMovies {
		movieIDs = append(movieIDs, movie.ID)
		deletedMovies = append(deletedMovies, movie.Title)
	}
	bulkEdit := radarr.BulkEdit{
		MovieIDs: movieIDs,
	}

	err := b.RadarrServer.DeleteMovies(&bulkEdit)
	if err != nil {
		msg := tgbotapi.NewMessage(command.chatID, err.Error())
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}

	messageText := fmt.Sprintf("Deleted movies:\n- %v", strings.Join(deletedMovies, "\n- "))
	editMsg := tgbotapi.NewEditMessageText(
		command.chatID,
		command.messageID,
		messageText,
	)

	b.clearState(update)
	b.sendMessage(editMsg)
	return true
}

func (b *Bot) handleDeleteMovieSelection(update tgbotapi.Update, command *userDeleteMovie) bool {
	movieIDStr := strings.TrimPrefix(update.CallbackQuery.Data, "TMDBID_")
	movie := command.library[movieIDStr]

	// Check if the movie is already selected
	// If not selected, add the movie to selectedMovies (select)
	command.selectedMovies = append(command.selectedMovies, movie)
	// If selected, remove the movie from selectedMovies (deselect)
	if isSelectedMovie(command.selectedMovies, movie.ID) {
		// If selected, remove the movie from selectedMovies (deselect)
		command.selectedMovies = removeMovie(command.selectedMovies, movie.ID)
	}

	b.setDeleteMovieState(command.chatID, command)

	if command.searchResultsInLibrary != nil {
		return b.showDeleteMovieSelection(update, command)
	} else {
		return b.showDeleteMovieSelection(update, command)
	}
}

func isSelectedMovie(selectedMovies []*radarr.Movie, MovieID int64) bool {
	for _, selectedMovie := range selectedMovies {
		if selectedMovie.ID == MovieID {
			return true
		}
	}
	return false
}

func removeMovie(selectedMovies []*radarr.Movie, MovieID int64) []*radarr.Movie {
	var updatedMovies []*radarr.Movie
	for _, movie := range selectedMovies {
		if movie.ID != MovieID {
			updatedMovies = append(updatedMovies, movie)
		}
	}
	return updatedMovies
}
