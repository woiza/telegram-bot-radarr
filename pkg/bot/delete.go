package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

func (b *Bot) deleteMovie(update tgbotapi.Update) bool {
	userID, err := getUserID(update)
	if err != nil {
		fmt.Printf("Cannot delete movie: %v", err)
		return false
	}
	command := b.DeleteMovieUserStates[userID]

	if command.movie == nil {
		movie := command.library[update.CallbackQuery.Data]
		command.movie = movie

		buttons := make([][]tgbotapi.InlineKeyboardButton, 3)
		buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Yes, delete this movie", "DELETEMOVIE_YES"))
		buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("No, show library again", "DELETEMOVIE_NO"))
		buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "DELETEMOVIE_CANCEL"))

		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Do you want to delete the following movie including all files?\n\n")
		msg.Text += fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n", utils.Escape(command.movie.Title), command.movie.ImdbID, command.movie.Year)
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = false
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)

		b.DeleteMovieUserStates[userID] = command
		b.sendMessage(msg)
		return false
	}
	if !command.confirmation {
		if update.CallbackQuery.Data == "DELETEMOVIE_YES" {

			command.confirmation = true
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
			//delete movie, delete files, no import exclusion
			var err = b.RadarrServer.DeleteMovie(command.movie.ID, *starr.True(), *starr.False())
			if err != nil {
				msg.Text = err.Error()
				fmt.Println(err)
				b.sendMessage(msg)
				return false
			}

			msg.Text = fmt.Sprintf("Movie '%v' deleted\n", command.movie.Title)
			command.library = nil
			command.movie = nil
			b.sendMessage(msg)

		} else if update.CallbackQuery.Data == "DELETEMOVIE_NO" {
			library := command.library
			command.confirmation = false
			command.movie = nil
			movies := make([]*radarr.Movie, 0, len(library))
			for _, value := range library {
				movies = append(movies, value)
			}
			b.DeleteMovieUserStates[userID] = command
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
			msg.Text = "Which movie would you like to delete?\n"
			b.sendMoviesAsInlineKeyboard(movies, &msg)
			return false
		} else if update.CallbackQuery.Data == "DELETEMOVIE_CANCEL" {
			b.clearState(update)
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
			b.sendMessage(msg)
			return false
		}
	}

	return true
}
