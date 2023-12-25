package bot

import (
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

func (b *Bot) addMovie(update tgbotapi.Update) bool {
	command := b.AddMovieUserStates[update.CallbackQuery.From.ID]

	if command.movie == nil {
		movie := command.searchResults[update.CallbackQuery.Data]
		command.movie = movie

		buttons := make([][]tgbotapi.InlineKeyboardButton, 3)
		buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Yes, add this movie", "ADDMOVIE_YES"))
		buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("No, show search results", "ADDMOVIE_NO"))
		buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "ADDMOVIE_CANCEL"))

		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Is this the correct movie?\n\n")
		msg.Text += fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n", utils.Escape(command.movie.Title), command.movie.ImdbID, command.movie.Year)
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = false
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)

		b.sendMessage(msg)
		b.AddMovieUserStates[update.CallbackQuery.From.ID] = command
		return false
	}
	if !command.confirmation {
		switch update.CallbackQuery.Data {
		case "ADDMOVIE_YES":
			command.confirmation = true
			//movie already in library...
			if command.movie.ID != 0 {
				b.clearState()
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Movie already exists in your library.\nAll commands have been cleared.")
				b.sendMessage(msg)
				return false
			} else {
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
				profiles, err := b.RadarrServer.GetQualityProfiles()
				if err != nil {
					msg.Text = err.Error()
					fmt.Println(err)
					b.sendMessage(msg)
				}
				if len(profiles) > 1 {
					buttons := make([][]tgbotapi.InlineKeyboardButton, len(profiles))
					for i, profile := range profiles {
						button := tgbotapi.NewInlineKeyboardButtonData(profile.Name, strconv.Itoa(int(profile.ID)))
						buttons[i] = tgbotapi.NewInlineKeyboardRow(button)
					}
					msg.Text = "Please choose your quality profile"
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
					b.AddMovieUserStates[update.CallbackQuery.From.ID] = command
					b.sendMessage(msg)
					return false
				} else if len(profiles) == 1 {
					profileID := profiles[0].ID
					update.CallbackQuery.Data = strconv.FormatInt(profileID, 10)
				} else {
					b.AddMovieUserStates[update.CallbackQuery.From.ID] = command
					msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, utils.Escape("No quality profile(s) found on your radarr server.\nAll commands have been cleared."))
					b.clearState()
					b.sendMessage(msg)
					return false
				}
			}
		case "ADDMOVIE_CANCEL":
			b.clearState()
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
			b.sendMessage(msg)
			return false
		// ADDMOVIE_NO is the same as the default
		default:
			command.confirmation = false
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
			command.movie = nil
			b.AddMovieUserStates[update.CallbackQuery.From.ID] = command
			b.sendSearchResults(command.searchResults, &msg)
			return false
		}
	}
	if command.profileID == nil {
		profileID, _ := strconv.Atoi(update.CallbackQuery.Data)
		command.movie.QualityProfileID = int64(profileID)
		command.profileID = &command.movie.QualityProfileID

		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
		rootFolders, err := b.RadarrServer.GetRootFolders()
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			return false
		}

		buttons := make([][]tgbotapi.InlineKeyboardButton, len(rootFolders))
		if len(rootFolders) > 1 {
			for i, folder := range rootFolders {
				path := folder.Path
				button := tgbotapi.NewInlineKeyboardButtonData(path, path)
				buttons[i] = tgbotapi.NewInlineKeyboardRow(button)
			}

			b.AddMovieUserStates[update.CallbackQuery.From.ID] = command
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, utils.Escape(fmt.Sprintf("Please choose the root folder for '%v'\n", command.movie.Title)))
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
			b.sendMessage(msg)
			return false
		} else if len(rootFolders) == 1 {
			path := rootFolders[0].Path
			update.CallbackQuery.Data = path
		} else {
			b.AddMovieUserStates[update.CallbackQuery.From.ID] = command
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, utils.Escape("No root folder(s) found on your radarr server.\nAll commands have been cleared."))
			b.clearState()
			b.sendMessage(msg)
			return false
		}
	}
	if command.path == nil {
		command.movie.Path = update.CallbackQuery.Data // there is no rootFolderPath in movie struct --> misuse path
		command.path = &command.movie.Path

		msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "How would you like to add the movie?\n")
		buttons := make([][]tgbotapi.InlineKeyboardButton, 4)
		buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Add monitored + search now", "MONITORED_MONSEA"))
		buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Add monitored", "MONITORED_MON"))
		buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Add unmonitored", "MONITORED_UNMON"))
		buttons[3] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "MONITORED_CANCEL"))

		b.AddMovieUserStates[update.CallbackQuery.From.ID] = command
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
		b.sendMessage(msg)
		return false
	}
	if command.addStatus == nil {
		var addMovieInput radarr.AddMovieInput
		var addOptions radarr.AddMovieOptions
		switch update.CallbackQuery.Data {
		case "MONITORED_MONSEA":
			addMovieInput.Monitored = *starr.True()
			addOptions = radarr.AddMovieOptions{
				SearchForMovie: *starr.True(),
				Monitor:        "movieOnly",
			}
		case "MONITORED_MON":
			addMovieInput.Monitored = *starr.True()
			addOptions = radarr.AddMovieOptions{
				SearchForMovie: *starr.False(),
				Monitor:        "movieOnly",
			}
		case "MONITORED_UNMON":
			addMovieInput.Monitored = *starr.False()
			addOptions = radarr.AddMovieOptions{
				SearchForMovie: *starr.False(),
				Monitor:        "none",
			}
		case "MONITORED_CANCEL":
			b.clearState()
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
			b.sendMessage(msg)
			return false
		}
		command.movie.AddOptions = &addOptions
		addMovieInput.TmdbID = command.movie.TmdbID
		addMovieInput.Title = command.movie.Title
		addMovieInput.QualityProfileID = *command.profileID
		addMovieInput.RootFolderPath = command.movie.Path
		addMovieInput.AddOptions = &addOptions

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

		movieTitle := movies[0].Title
		command.searchResults = nil
		command.movie = nil
		msg.Text = fmt.Sprintf("Movie '%v' added\n", movieTitle)
		b.sendMessage(msg)
	}

	return true
}
