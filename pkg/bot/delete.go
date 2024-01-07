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
		sort.SliceStable(movies, func(i, j int) bool {
			return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
		})

		inlineKeyboard := b.getMoviesAsInlineKeyboard(movies)
		editMsg := tgbotapi.NewEditMessageTextAndMarkup(
			command.chatID,
			command.messageID,
			"Which movie would you like to delete?",
			tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: inlineKeyboard,
			},
		)
		b.sendMessage(editMsg)
		return
	}

	searchResults, err := r.Lookup(criteria)
	if err != nil {
		msg := tgbotapi.NewMessage(userID, err.Error())
		b.sendMessage(msg)
		return
	}
	if len(searchResults) == 0 {
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"No movies found matching your search criteria",
		)
		b.sendMessage(editMsg)
		return
	}
	if len(searchResults) > 25 {
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"Result size too large, please narrow down your search criteria",
		)
		b.sendMessage(editMsg)
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
		editMsg := tgbotapi.NewEditMessageText(
			command.chatID,
			command.messageID,
			"No movies found in your library",
		)
		b.sendMessage(editMsg)
		return
	}

	command.searchResultsInLibrary = make(map[string]*radarr.Movie, len(moviesInLibrary))
	for _, movie := range moviesInLibrary {
		tmdbID := strconv.Itoa(int(movie.TmdbID))
		command.searchResultsInLibrary[tmdbID] = movie
	}
	// go to confirmation step
	if len(moviesInLibrary) == 1 {
		command.movie = moviesInLibrary[0]
		b.setDeleteMovieState(userID, &command)
		b.processMovieSelectionForDelete(update, &command)
		return
	}

	sort.SliceStable(moviesInLibrary, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(moviesInLibrary[i].Title)) < utils.IgnoreArticles(strings.ToLower(moviesInLibrary[j].Title))
	})

	inlineKeyboard := b.getMoviesAsInlineKeyboard(moviesInLibrary)
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		message.Chat.ID,
		message.MessageID,
		"Which movie would you like to delete?",
		tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	)
	b.setDeleteMovieState(userID, &command)
	b.sendMessage(editMsg)
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

	switch {
	case command.movie == nil:
		return b.processMovieSelectionForDelete(update, command)
	case !command.confirmation:
		return b.processConfirmationForDelete(update, command)
	default:
		return true
	}
}

func (b *Bot) processMovieSelectionForDelete(update tgbotapi.Update, command *userDeleteMovie) bool {
	// if called in processDeleteCommand update has no CallbackQuery and command.movie is set in inprocessDeleteCommand
	if command.movie == nil {
		movie := command.library[update.CallbackQuery.Data]
		command.movie = movie
	}

	var keyboard tgbotapi.InlineKeyboardMarkup
	if len(command.searchResultsInLibrary) > 0 {
		buttons := make([][]tgbotapi.InlineKeyboardButton, 4)
		buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Yes, delete this movie", "DELETEMOVIE_YES"))
		buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("No, show search results", "DELETEMOVIE_NO_SEARCH"))
		buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("No, show complete library", "DELETEMOVIE_NO_LIBRARY"))
		buttons[3] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "DELETEMOVIE_CANCEL"))
		keyboard = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	} else {
		buttons := make([][]tgbotapi.InlineKeyboardButton, 3)
		buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Yes, delete this movie", "DELETEMOVIE_YES"))
		buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("No, show complete library", "DELETEMOVIE_NO_LIBRARY"))
		buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "DELETEMOVIE_CANCEL"))
		keyboard = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	}

	// Create an edit message request.
	editMsg := tgbotapi.NewEditMessageText(
		command.chatID,
		command.messageID,
		fmt.Sprintf("Do you want to delete the following movie including all files?\n\n[%v](https://www.imdb.com/title/%v) \\- _%v_\n",
			utils.Escape(command.movie.Title), command.movie.ImdbID, command.movie.Year),
	)
	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = false
	editMsg.ReplyMarkup = &keyboard

	b.setDeleteMovieState(command.chatID, command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) processConfirmationForDelete(update tgbotapi.Update, command *userDeleteMovie) bool {
	switch update.CallbackQuery.Data {
	case "DELETEMOVIE_YES":
		return b.handleDeleteConfirmationYes(update, command)
	case "DELETEMOVIE_NO_SEARCH":
		return b.handleDeleteConfirmationNoSearchResults(update, command)
	case "DELETEMOVIE_NO_LIBRARY":
		return b.handleDeleteConfirmationNoLibrary(update, command)
	case "DELETEMOVIE_CANCEL":
		return b.handleDeleteConfirmationCancel(update, command)
	default:
		command.confirmation = false
		command.movie = nil
		b.setDeleteMovieState(command.chatID, command)
		return false
	}
}

func (b *Bot) handleDeleteConfirmationYes(update tgbotapi.Update, command *userDeleteMovie) bool {
	command.confirmation = true
	msg := tgbotapi.NewMessage(command.chatID, "")

	err := b.RadarrServer.DeleteMovie(command.movie.ID, *starr.True(), *starr.False())
	if err != nil {
		msg.Text = err.Error()
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}

	editMsg := tgbotapi.NewEditMessageText(
		command.chatID,
		command.messageID,
		fmt.Sprintf("Movie '%v' deleted\n", command.movie.Title),
	)

	b.clearState(update)
	b.sendMessage(editMsg)
	return true
}

func (b *Bot) handleDeleteConfirmationNoSearchResults(update tgbotapi.Update, command *userDeleteMovie) bool {
	searchResults := command.searchResultsInLibrary
	command.confirmation = false
	command.movie = nil
	movies := make([]*radarr.Movie, 0, len(searchResults))
	for _, value := range searchResults {
		movies = append(movies, value)
	}

	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})

	inlineKeyboard := b.getMoviesAsInlineKeyboard(movies)
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		"Which movie would you like to delete?",
		tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	)
	b.setDeleteMovieState(command.chatID, command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) handleDeleteConfirmationNoLibrary(update tgbotapi.Update, command *userDeleteMovie) bool {
	library := command.library
	command.confirmation = false
	command.movie = nil
	movies := make([]*radarr.Movie, 0, len(library))
	for _, value := range library {
		movies = append(movies, value)
	}

	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})

	inlineKeyboard := b.getMoviesAsInlineKeyboard(movies)
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.chatID,
		command.messageID,
		"Which movie would you like to delete?",
		tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	)
	b.setDeleteMovieState(command.chatID, command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) handleDeleteConfirmationCancel(update tgbotapi.Update, command *userDeleteMovie) bool {
	b.clearState(update)
	editMsg := tgbotapi.NewEditMessageText(
		command.chatID,
		command.messageID,
		"All commands have been cleared",
	)
	b.sendMessage(editMsg)
	return false
}
