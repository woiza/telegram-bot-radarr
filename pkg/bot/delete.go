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

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Which movie would you like to delete?")
	message, _ := b.sendMessage(msg)

	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})

	inlineKeyboard := b.getMoviesAsInlineKeyboard(movies)
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		message.Chat.ID,
		message.MessageID,
		message.Text,
		tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	)
	b.setDeleteMovieState(userID, &command)
	b.sendMessage(editMsg)
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
	movie := command.library[update.CallbackQuery.Data]
	command.movie = movie

	buttons := make([][]tgbotapi.InlineKeyboardButton, 3)
	buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Yes, delete this movie", "DELETEMOVIE_YES"))
	buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("No, show library again", "DELETEMOVIE_NO"))
	buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "DELETEMOVIE_CANCEL"))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)

	// Create an edit message request.
	editMsg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.Message.Chat.ID,
		update.CallbackQuery.Message.MessageID,
		fmt.Sprintf("Do you want to delete the following movie including all files?\n\n[%v](https://www.imdb.com/title/%v) \\- _%v_\n",
			utils.Escape(command.movie.Title), command.movie.ImdbID, command.movie.Year),
	)
	editMsg.ParseMode = "MarkdownV2"
	editMsg.DisableWebPagePreview = false
	editMsg.ReplyMarkup = &keyboard

	b.setDeleteMovieState(update.CallbackQuery.From.ID, command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) processConfirmationForDelete(update tgbotapi.Update, command *userDeleteMovie) bool {
	switch update.CallbackQuery.Data {
	case "DELETEMOVIE_YES":
		return b.handleDeleteConfirmationYes(update, command)
	case "DELETEMOVIE_NO":
		return b.handleDeleteConfirmationNo(update, command)
	case "DELETEMOVIE_CANCEL":
		return b.handleDeleteConfirmationCancel(update)
	default:
		command.confirmation = false
		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
		command.movie = nil
		b.setDeleteMovieState(update.CallbackQuery.From.ID, command)
		b.sendMessage(msg)
		return false
	}
}

func (b *Bot) handleDeleteConfirmationYes(update tgbotapi.Update, command *userDeleteMovie) bool {
	command.confirmation = true
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")

	err := b.RadarrServer.DeleteMovie(command.movie.ID, *starr.True(), *starr.False())
	if err != nil {
		msg.Text = err.Error()
		fmt.Println(err)
		b.sendMessage(msg)
		return false
	}

	editMsg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.Message.Chat.ID,
		update.CallbackQuery.Message.MessageID,
		fmt.Sprintf("Movie '%v' deleted\n", command.movie.Title),
	)

	b.clearState(update)
	b.sendMessage(editMsg)
	return true
}

func (b *Bot) handleDeleteConfirmationNo(update tgbotapi.Update, command *userDeleteMovie) bool {
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
		update.CallbackQuery.From.ID,
		update.CallbackQuery.Message.MessageID,
		"Which movie would you like to delete?",
		tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: inlineKeyboard,
		},
	)
	b.setDeleteMovieState(update.CallbackQuery.From.ID, command)
	b.sendMessage(editMsg)
	return false
}

func (b *Bot) handleDeleteConfirmationCancel(update tgbotapi.Update) bool {
	b.clearState(update)
	editMsg := tgbotapi.NewEditMessageText(
		update.CallbackQuery.From.ID,
		update.CallbackQuery.Message.MessageID,
		"All commands have been cleared",
	)
	b.sendMessage(editMsg)
	return false
}
