package bot

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golift.io/starr"
	"golift.io/starr/radarr"

	"github.com/woiza/telegram-bot-radarr/pkg/config"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
)

type userAddMovie struct {
	searchResults map[string]*radarr.Movie
	movie         *radarr.Movie
	confirmation  bool
	profileID     *int64
	path          *string
	addStatus     *string
}

type userDeleteMovie struct {
	library      map[string]*radarr.Movie
	movie        *radarr.Movie
	confirmation bool
}

type Bot struct {
	Config       config.Config
	Bot          *tgbotapi.BotAPI
	RadarrServer *radarr.Radarr

	UserActiveCommand     map[int64]string
	AddMovieUserStates    map[int64]userAddMovie
	DeleteMovieUserStates map[int64]userDeleteMovie
}

func (b Bot) StartBot() {
	lastOffset := 0
	updateConfig := tgbotapi.NewUpdate(lastOffset + 1)
	updateConfig.Timeout = 60

	updatesChannel := b.Bot.GetUpdatesChan(updateConfig)

	time.Sleep(time.Millisecond * 500)
	updatesChannel.Clear()

	for update := range updatesChannel {
		lastOffset = update.UpdateID

		if update.Message != nil {
			if !b.Config.AllowedUserIDs[update.Message.From.ID] {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Access denied. You are not authorized.")
				b.sendMessage(msg)
				continue
			}
		}

		if update.CallbackQuery != nil {
			switch b.UserActiveCommand[update.CallbackQuery.From.ID] {
			case "ADDMOVIE":
				if !b.addMovie(update) {
					continue
				}
			case "DELETEMOVIE":
				if !b.deleteMovie(update) {
					continue
				}
			default:
				// Handle unexpected callback queries
				b.clearState()
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "I am not sure what you mean.\nAll commands have been cleared")
				b.sendMessage(msg)
				break
			}
		}

		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		if update.Message.IsCommand() {
			b.handleCommand(b.Bot, update, b.RadarrServer)
		}
	}
}

func (b Bot) addMovie(update tgbotapi.Update) bool {
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
		case "ADDMOVIE_NO":
			command.confirmation = false
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
			command.movie = nil
			b.AddMovieUserStates[update.CallbackQuery.From.ID] = command
			b.sendSearchResults(command.searchResults, &msg)
			return false
		case "ADDMOVIE_CANCEL":
			b.clearState()
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
			b.sendMessage(msg)
			return false
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

func (b Bot) deleteMovie(update tgbotapi.Update) bool {
	command := b.DeleteMovieUserStates[update.CallbackQuery.From.ID]

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

		b.DeleteMovieUserStates[update.CallbackQuery.From.ID] = command
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
			b.DeleteMovieUserStates[update.CallbackQuery.From.ID] = command
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
			msg.Text = "Which movie would you like to delete?\n"
			b.sendLibraryAsInlineKeyboard(movies, &msg)
			return false
		} else if update.CallbackQuery.Data == "DELETEMOVIE_CANCEL" {
			b.clearState()
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
			b.sendMessage(msg)
			return false
		}
	}

	return true
}

func (b Bot) handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, r *radarr.Radarr) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	switch update.Message.Command() {

	case "q", "query", "add", "Q", "Query", "Add":
		criteria := update.Message.CommandArguments()
		if len(criteria) < 1 {
			msg.Text = "Please provide a search criteria /q [query]"
			b.sendMessage(msg)
			break
		}
		searchResults, err := r.Lookup(criteria)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		if len(searchResults) == 0 {
			msg.Text = "No movies found"
			b.sendMessage(msg)
			break
		}
		if len(searchResults) > 25 {
			msg.Text = "Result size too large, please narrow down your search criteria"
			b.sendMessage(msg)
			break
		}
		command := userAddMovie{
			searchResults: make(map[string]*radarr.Movie, len(searchResults)),
		}
		for _, movie := range searchResults {
			tmdbID := strconv.Itoa(int(movie.TmdbID))
			command.searchResults[tmdbID] = movie
		}
		b.AddMovieUserStates[update.Message.From.ID] = command
		b.UserActiveCommand[update.Message.From.ID] = "ADDMOVIE"
		b.sendSearchResults(command.searchResults, &msg)

	case "clear", "cancel", "stop":
		b.clearState()
		msg.Text = "All commands have been cleared"
		b.sendMessage(msg)

	case "diskspace", "disk", "free", "rootfolder", "rootfolders":
		rootFolders, err := r.GetRootFolders()
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		msg.Text = utils.PrepareRootFolders(rootFolders)
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = true
		b.sendMessage(msg)

	case "delete", "remove":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}

		command := userDeleteMovie{
			library: make(map[string]*radarr.Movie, len(movies)),
		}

		for _, movie := range movies {
			tmdbID := strconv.Itoa(int(movie.TmdbID))
			command.library[tmdbID] = movie
		}
		b.DeleteMovieUserStates[update.Message.From.ID] = command
		b.UserActiveCommand[update.Message.From.ID] = "DELETEMOVIE"
		msg.Text = "Which movie would you like to delete?\n"
		b.sendLibraryAsInlineKeyboard(movies, &msg)

	case "rss", "RSS":
		command := radarr.CommandRequest{
			Name:     "RssSync",
			MovieIDs: []int64{},
		}
		_, err := r.SendCommand(&command)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		msg.Text = "RSS sync started"
		b.sendMessage(msg)

	case "wanted":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		var monitoredMoviesIDs []int64
		for _, movie := range movies {
			if movie.Monitored == true {
				monitoredMoviesIDs = append(monitoredMoviesIDs, movie.ID)
			}
		}
		command := radarr.CommandRequest{
			Name:     "MoviesSearch",
			MovieIDs: monitoredMoviesIDs,
		}
		_, err = r.SendCommand(&command)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		msg.Text = "Search for monitored movies started"
		b.sendMessage(msg)

	case "up", "upcoming":
		calendar := radarr.Calendar{
			Start:       time.Now(),
			End:         time.Now().AddDate(0, 0, 30), // 30 days
			Unmonitored: *starr.True(),
		}
		upcoming, err := r.GetCalendar(calendar)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		if len(upcoming) == 0 {
			msg.Text = "no upcoming releases in the next 30 days"
			msg.ParseMode = "MarkdownV2"
			msg.DisableWebPagePreview = true
			b.sendMessage(msg)
			break
		}
		b.sendUpcoming(upcoming, &msg, bot)

	case "dl", "download", "downloads", "downloaded", "available":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		// Only downloaded movies with size information
		b.sendLibraryDownloaded(movies, &msg, bot)

	case "movies", "library":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		// All movies without size information
		b.sendLibrary(movies, &msg, bot)

	case "updateAll", "updateall":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		var allMoviesIDs []int64
		for _, movie := range movies {
			allMoviesIDs = append(allMoviesIDs, movie.ID)
		}
		command := radarr.CommandRequest{
			Name:     "RefreshMovie",
			MovieIDs: allMoviesIDs,
		}
		_, err = r.SendCommand(&command)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		msg.Text = "Update All started"
		b.sendMessage(msg)

	case "getid", "id":
		userID := update.Message.From.ID
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Your user ID: %d", userID))
		b.sendMessage(msg)

	default:
		msg.Text = fmt.Sprintf("Hello %v!\n", update.Message.From)
		msg.Text += "Here's a list of commands at your disposal:\n\n"
		msg.Text += "/q [movie] - searches a movie \n"
		msg.Text += "/clear \t\t - deletes all previously sent commands\n"
		msg.Text += "/free \t\t\t\t - lists the free space of your disks\n"
		msg.Text += "/delete - delete a movie - WARNING: can be large\n"
		msg.Text += "/rss \t\t\t\t   - perform a RSS sync\n"
		msg.Text += "/wanted - searches all monitored movies\n"
		msg.Text += "/upcoming - lists upcoming movies in the next 30 days\n"
		msg.Text += "/dl \t\t\t\t\t\t\t - lists downloaded movies - WARNING: can be large\n"
		msg.Text += "/library - lists all movies - WARNING: can be large\n"
		msg.Text += "/updateall - update metadata and rescan files/folders\n"
		msg.Text += "/id \t\t\t\t\t\t\t - shows your Telegram user ID"
		b.sendMessage(msg)
	}
}

func (b Bot) sendSearchResults(searchResults map[string]*radarr.Movie, msg *tgbotapi.MessageConfig) {
	// Extract movies from the map
	movies := make([]*radarr.Movie, 0, len(searchResults))
	for _, movie := range searchResults {
		movies = append(movies, movie)
	}

	// Sort movies by year in ascending order
	sort.SliceStable(movies, func(i, j int) bool {
		return movies[i].Year < movies[j].Year
	})

	var rows [][]tgbotapi.InlineKeyboardButton
	var text strings.Builder
	for _, movie := range movies {
		text.WriteString(fmt.Sprintf("[%v](https://www.themoviedb.org/movie/%v) \\- _%v_\n", utils.Escape(movie.Title), movie.TmdbID, movie.Year))
		button := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%v - %v", movie.Title, movie.Year), strconv.Itoa(int(movie.TmdbID)))
		row := []tgbotapi.InlineKeyboardButton{button}
		rows = append(rows, row)
	}
	switch len(searchResults) {
	case 1:
		msg.Text = "*Movie found*\n\n"
	default:
		msg.Text = fmt.Sprintf("*Found %d movies*\n\n", len(searchResults))
	}
	msg.Text += text.String()
	inlineKeyBoardMarkup := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg.ParseMode = "MarkdownV2"
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = inlineKeyBoardMarkup
	b.sendMessage(msg)
}

func (b Bot) sendUpcoming(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool { return movies[i].Title < movies[j].Title })
	for i := 0; i < len(movies); i += b.Config.MaxItems {
		end := i + b.Config.MaxItems
		if end > len(movies) {
			end = len(movies)
		}

		var text strings.Builder
		for _, movie := range movies[i:end] {
			if !movie.InCinemas.IsZero() {
				text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- cinema %v\n", utils.Escape(movie.Title), movie.ImdbID, utils.Escape(movie.InCinemas.Format("02 Jan 2006"))))
			}
			if !movie.DigitalRelease.IsZero() {
				text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- digital %v\n", utils.Escape(movie.Title), movie.ImdbID, utils.Escape(movie.DigitalRelease.Format("02 Jan 2006"))))
			}
			if !movie.PhysicalRelease.IsZero() {
				text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- physical %v\n", utils.Escape(movie.Title), movie.ImdbID, utils.Escape(movie.PhysicalRelease.Format("02 Jan 2006"))))
			}
		}

		msg.Text = text.String()
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = true
		b.sendMessage(msg)
	}
}

func (b Bot) sendLibrary(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool { return movies[i].Title < movies[j].Title })

	for i := 0; i < len(movies); i += b.Config.MaxItems {
		end := i + b.Config.MaxItems
		if end > len(movies) {
			end = len(movies)
		}

		var text strings.Builder
		for _, movie := range movies[i:end] {
			text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n", utils.Escape(movie.Title), movie.ImdbID, movie.Year))
		}

		msg.Text = text.String()
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = true
		b.sendMessage(msg)
	}
}

func (b Bot) sendLibraryDownloaded(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool { return movies[i].Title < movies[j].Title })

	var filteredMovies []*radarr.Movie
	for _, movie := range movies {
		if movie.SizeOnDisk > 0 {
			filteredMovies = append(filteredMovies, movie)
		}
	}

	for i := 0; i < len(filteredMovies); i += b.Config.MaxItems {
		end := i + b.Config.MaxItems
		if end > len(filteredMovies) {
			end = len(filteredMovies)
		}

		var text strings.Builder
		for _, movie := range filteredMovies[i:end] {
			text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_ \\- _%v_\n", utils.Escape(movie.Title), movie.ImdbID, movie.Year, utils.Escape(utils.ByteCountSI(int64(movie.SizeOnDisk)))))
		}

		msg.Text = text.String()
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = true
		b.sendMessage(msg)
	}
}

func (b Bot) sendLibraryAsInlineKeyboard(movies []*radarr.Movie, msg *tgbotapi.MessageConfig) {
	sort.SliceStable(movies, func(i, j int) bool { return movies[i].Title < movies[j].Title })

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i, movie := range movies {
		if i > 0 && i%b.Config.MaxItems == 0 {
			inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
			msg.ReplyMarkup = inlineKeyboard
			b.sendMessage(msg)
			rows = nil
		}
		button := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%v - %v", movie.Title, movie.Year), strconv.Itoa(int(movie.TmdbID)))
		row = append(row, button)
		if len(row) > 0 {
			rows = append(rows, row)
			row = nil
		}
	}

	if len(rows) > 0 {
		inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		msg.ReplyMarkup = inlineKeyboard
		b.sendMessage(msg)
	}
}

func (b Bot) clearState() {
	b.UserActiveCommand = make(map[int64]string)
	b.AddMovieUserStates = make(map[int64]userAddMovie)
	b.DeleteMovieUserStates = make(map[int64]userDeleteMovie)
}

func (b Bot) sendMessage(msg tgbotapi.Chattable) {
	_, err := b.Bot.Send(msg)
	if err != nil {
		log.Println("Error sending message:", err)
	}
}
