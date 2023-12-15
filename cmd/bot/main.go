package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"golift.io/starr"
	"golift.io/starr/radarr"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/internal/config"
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

var maxItems int

var userActiveCommand = make(map[int64]string)
var addMovieUserStates = make(map[int64]userAddMovie)
var deleteMovieUserStates = make(map[int64]userDeleteMovie)

func main() {
	fmt.Println("Starting bot...")

	// get config from environment variables
	config, err := config.LoadConfig()
	if err != nil {
		// Handle error: configuration is incomplete or invalid
		log.Fatal(err)
	}

	maxItems = config.MaxItems

	bot, err := tgbotapi.NewBotAPI(config.TelegramBotToken)

	//bot.Debug = true
	if err != nil {
		log.Fatal("Error while starting bot: ", err)
	}

	fmt.Printf("Authorized on account %v\n", bot.Self.UserName)

	// Get a starr.Config that can plug into any Starr app.
	// starr.New(apiKey, appURL string, timeout time.Duration)
	c := starr.New(config.RadarrAPIKey, fmt.Sprintf("%v://%v:%v", config.RadarrProtocol, config.RadarrHostname, config.RadarrPort), 0)
	// Lets make a radarr server with the default starr Config.
	r := radarr.New(c)

	lastOffset := 0
	u := tgbotapi.NewUpdate(lastOffset + 1)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	time.Sleep(time.Millisecond * 500)
	updates.Clear()

	for update := range updates {
		lastOffset = update.UpdateID

		if update.Message != nil {
			if !config.AllowedUserIDs[update.Message.From.ID] {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Access denied. You are not authorized.")
				sendMessage(msg, bot)
				continue
			}
		}

		if update.CallbackQuery != nil {
			switch userActiveCommand[update.CallbackQuery.From.ID] {
			case "ADDMOVIE":
				command := addMovieUserStates[update.CallbackQuery.From.ID]

				if command.movie == nil {
					movie := command.searchResults[update.CallbackQuery.Data]
					command.movie = movie

					buttons := make([][]tgbotapi.InlineKeyboardButton, 3)
					buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Yes, add this movie", "ADDMOVIE_YES"))
					buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("No, show search results", "ADDMOVIE_NO"))
					buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "ADDMOVIE_CANCEL"))

					msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Is this the correct movie?\n\n")
					msg.Text += fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n", escape(command.movie.Title), command.movie.ImdbID, command.movie.Year)
					msg.ParseMode = "MarkdownV2"
					msg.DisableWebPagePreview = false
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)

					sendMessage(msg, bot)
					addMovieUserStates[update.CallbackQuery.From.ID] = command
					continue
				}
				if !command.confirmation {
					switch update.CallbackQuery.Data {
					case "ADDMOVIE_YES":
						command.confirmation = true
						//movie already in library...
						if command.movie.ID != 0 {
							clearState()
							msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Movie already exists in your library.\nAll commands have been cleared.")
							sendMessage(msg, bot)
							continue
						} else {
							msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
							profiles, err := r.GetQualityProfiles()
							if err != nil {
								msg.Text = err.Error()
								fmt.Println(err)
								sendMessage(msg, bot)
							}
							if len(profiles) > 1 {
								buttons := make([][]tgbotapi.InlineKeyboardButton, len(profiles))
								for i, profile := range profiles {
									button := tgbotapi.NewInlineKeyboardButtonData(profile.Name, strconv.Itoa(int(profile.ID)))
									buttons[i] = tgbotapi.NewInlineKeyboardRow(button)
								}
								msg.Text = "Please choose your quality profile"
								msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
								addMovieUserStates[update.CallbackQuery.From.ID] = command
								sendMessage(msg, bot)
								continue
							} else if len(profiles) == 1 {
								profileID := profiles[0].ID
								update.CallbackQuery.Data = strconv.FormatInt(profileID, 10)
							} else {
								addMovieUserStates[update.CallbackQuery.From.ID] = command
								msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, escape("No quality profile(s) found on your radarr server.\nAll commands have been cleared."))
								clearState()
								sendMessage(msg, bot)
								continue
							}
						}
					case "ADDMOVIE_NO":
						command.confirmation = false
						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
						command.movie = nil
						addMovieUserStates[update.CallbackQuery.From.ID] = command
						sendSearchResults(command.searchResults, &msg, bot)
						continue
					case "ADDMOVIE_CANCEL":
						clearState()
						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
						sendMessage(msg, bot)
						continue
					default:
						command.confirmation = false
						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
						command.movie = nil
						addMovieUserStates[update.CallbackQuery.From.ID] = command
						sendSearchResults(command.searchResults, &msg, bot)
						continue
					}
				}
				if command.profileID == nil {
					profileID, _ := strconv.Atoi(update.CallbackQuery.Data)
					command.movie.QualityProfileID = int64(profileID)
					command.profileID = &command.movie.QualityProfileID

					msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
					rootFolders, err := r.GetRootFolders()
					if err != nil {
						msg.Text = err.Error()
						fmt.Println(err)
						sendMessage(msg, bot)
						continue
					}

					buttons := make([][]tgbotapi.InlineKeyboardButton, len(rootFolders))
					if len(rootFolders) > 1 {
						for i, folder := range rootFolders {
							path := folder.Path
							button := tgbotapi.NewInlineKeyboardButtonData(path, path)
							buttons[i] = tgbotapi.NewInlineKeyboardRow(button)
						}

						addMovieUserStates[update.CallbackQuery.From.ID] = command
						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, escape(fmt.Sprintf("Please choose the root folder for '%v'\n", command.movie.Title)))
						msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
						sendMessage(msg, bot)
						continue
					} else if len(rootFolders) == 1 {
						path := rootFolders[0].Path
						update.CallbackQuery.Data = path
					} else {
						addMovieUserStates[update.CallbackQuery.From.ID] = command
						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, escape("No root folder(s) found on your radarr server.\nAll commands have been cleared."))
						clearState()
						sendMessage(msg, bot)
						continue
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

					addMovieUserStates[update.CallbackQuery.From.ID] = command
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
					sendMessage(msg, bot)
					continue
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
						clearState()
						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
						sendMessage(msg, bot)
						continue
					}
					command.movie.AddOptions = &addOptions
					addMovieInput.TmdbID = command.movie.TmdbID
					addMovieInput.Title = command.movie.Title
					addMovieInput.QualityProfileID = *command.profileID
					addMovieInput.RootFolderPath = command.movie.Path
					addMovieInput.AddOptions = &addOptions

					msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
					var _, err = r.AddMovie(&addMovieInput)
					if err != nil {
						msg.Text = err.Error()
						fmt.Println(err)
						sendMessage(msg, bot)
						continue
					}
					movies, err := r.GetMovie((command.movie.TmdbID))
					if err != nil {
						msg.Text = err.Error()
						fmt.Println(err)
						sendMessage(msg, bot)
						continue
					}

					movieTitle := movies[0].Title
					command.searchResults = nil
					command.movie = nil
					msg.Text = fmt.Sprintf("Movie '%v' added\n", movieTitle)
					sendMessage(msg, bot)
				}
			case "DELETEMOVIE":
				command := deleteMovieUserStates[update.CallbackQuery.From.ID]

				if command.movie == nil {
					movie := command.library[update.CallbackQuery.Data]
					command.movie = movie

					buttons := make([][]tgbotapi.InlineKeyboardButton, 3)
					buttons[0] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Yes, delete this movie", "DELETEMOVIE_YES"))
					buttons[1] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("No, show library again", "DELETEMOVIE_NO"))
					buttons[2] = tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Cancel, clear command", "DELETEMOVIE_CANCEL"))

					msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Do you want to delete the following movie including all files?\n\n")
					msg.Text += fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n", escape(command.movie.Title), command.movie.ImdbID, command.movie.Year)
					msg.ParseMode = "MarkdownV2"
					msg.DisableWebPagePreview = false
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)

					deleteMovieUserStates[update.CallbackQuery.From.ID] = command
					sendMessage(msg, bot)
					continue
				}
				if !command.confirmation {
					if update.CallbackQuery.Data == "DELETEMOVIE_YES" {

						command.confirmation = true
						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
						//delete movie, delete files, no import exclusion
						var err = r.DeleteMovie(command.movie.ID, *starr.True(), *starr.False())
						if err != nil {
							msg.Text = err.Error()
							fmt.Println(err)
							sendMessage(msg, bot)
							continue
						}

						msg.Text = fmt.Sprintf("Movie '%v' deleted\n", command.movie.Title)
						command.library = nil
						command.movie = nil
						sendMessage(msg, bot)

					} else if update.CallbackQuery.Data == "DELETEMOVIE_NO" {
						library := command.library
						command.confirmation = false
						command.movie = nil
						movies := make([]*radarr.Movie, 0, len(library))
						for _, value := range library {
							movies = append(movies, value)
						}
						deleteMovieUserStates[update.CallbackQuery.From.ID] = command
						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
						msg.Text = "Which movie would you like to delete?\n"
						sendLibraryAsInlineKeyboard(movies, &msg, bot)
						continue
					} else if update.CallbackQuery.Data == "DELETEMOVIE_CANCEL" {
						clearState()
						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "All commands have been cleared")
						sendMessage(msg, bot)
						continue
					}
				}
			default:
				// Handle unexpected callback queries
				clearState()
				msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "I am not sure what you mean.\nAll commands have been cleared")
				sendMessage(msg, bot)
				break
			}
		}

		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		if update.Message.IsCommand() {
			handleCommand(bot, update, r)
		}
	}
}

func handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, r *radarr.Radarr) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	switch update.Message.Command() {

	case "q", "query", "add", "Q", "Query", "Add":
		criteria := update.Message.CommandArguments()
		if len(criteria) < 1 {
			msg.Text = "Please provide a search criteria /q [query]"
			sendMessage(msg, bot)
			break
		}
		searchResults, err := r.Lookup(criteria)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			sendMessage(msg, bot)
			break
		}
		if len(searchResults) == 0 {
			msg.Text = "No movies found"
			sendMessage(msg, bot)
			break
		}
		if len(searchResults) > 25 {
			msg.Text = "Result size too large, please narrow down your search criteria"
			sendMessage(msg, bot)
			break
		}
		command := userAddMovie{
			searchResults: make(map[string]*radarr.Movie, len(searchResults)),
		}
		for _, movie := range searchResults {
			tmdbID := strconv.Itoa(int(movie.TmdbID))
			command.searchResults[tmdbID] = movie
		}
		addMovieUserStates[update.Message.From.ID] = command
		userActiveCommand[update.Message.From.ID] = "ADDMOVIE"
		sendSearchResults(command.searchResults, &msg, bot)

	case "clear", "cancel", "stop":
		clearState()
		msg.Text = "All commands have been cleared"
		sendMessage(msg, bot)

	case "diskspace", "disk", "free", "rootfolder", "rootfolders":
		rootFolders, err := r.GetRootFolders()
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			sendMessage(msg, bot)
			break
		}
		msg.Text = prepareRootFolders(rootFolders)
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = true
		sendMessage(msg, bot)

	case "delete", "remove":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			sendMessage(msg, bot)
			break
		}

		command := userDeleteMovie{
			library: make(map[string]*radarr.Movie, len(movies)),
		}

		for _, movie := range movies {
			tmdbID := strconv.Itoa(int(movie.TmdbID))
			command.library[tmdbID] = movie
		}
		deleteMovieUserStates[update.Message.From.ID] = command
		userActiveCommand[update.Message.From.ID] = "DELETEMOVIE"
		msg.Text = "Which movie would you like to delete?\n"
		sendLibraryAsInlineKeyboard(movies, &msg, bot)

	case "rss", "RSS":
		command := radarr.CommandRequest{
			Name:     "RssSync",
			MovieIDs: []int64{},
		}
		_, err := r.SendCommand(&command)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			sendMessage(msg, bot)
			break
		}
		msg.Text = "RSS sync started"
		sendMessage(msg, bot)

	case "wanted":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			sendMessage(msg, bot)
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
			sendMessage(msg, bot)
			break
		}
		msg.Text = "Search for monitored movies started"
		sendMessage(msg, bot)

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
			sendMessage(msg, bot)
			break
		}
		if len(upcoming) == 0 {
			msg.Text = "no upcoming releases in the next 30 days"
			msg.ParseMode = "MarkdownV2"
			msg.DisableWebPagePreview = true
			sendMessage(msg, bot)
			break
		}
		sendUpcoming(upcoming, &msg, bot)

	case "dl", "download", "downloads", "downloaded", "available":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			sendMessage(msg, bot)
			break
		}
		// Only downloaded movies with size information
		sendLibraryDownloaded(movies, &msg, bot)

	case "movies", "library":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			sendMessage(msg, bot)
			break
		}
		// All movies without size information
		sendLibrary(movies, &msg, bot)

	case "updateAll", "updateall":
		movies, err := r.GetMovie(0)
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			sendMessage(msg, bot)
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
			sendMessage(msg, bot)
			break
		}
		msg.Text = "Update All started"
		sendMessage(msg, bot)

	case "getid", "id":
		userID := update.Message.From.ID
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Your user ID: %d", userID))
		sendMessage(msg, bot)

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
		sendMessage(msg, bot)
	}
}

func sendSearchResults(searchResults map[string]*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
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
		text.WriteString(fmt.Sprintf("[%v](https://www.themoviedb.org/movie/%v) \\- _%v_\n", escape(movie.Title), movie.TmdbID, movie.Year))
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
	sendMessage(msg, bot)
}

func sendUpcoming(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool { return movies[i].Title < movies[j].Title })
	for i := 0; i < len(movies); i += maxItems {
		end := i + maxItems
		if end > len(movies) {
			end = len(movies)
		}

		var text strings.Builder
		for _, movie := range movies[i:end] {
			if !movie.InCinemas.IsZero() {
				text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- cinema %v\n", escape(movie.Title), movie.ImdbID, escape(movie.InCinemas.Format("02 Jan 2006"))))
			}
			if !movie.DigitalRelease.IsZero() {
				text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- digital %v\n", escape(movie.Title), movie.ImdbID, escape(movie.DigitalRelease.Format("02 Jan 2006"))))
			}
			if !movie.PhysicalRelease.IsZero() {
				text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- physical %v\n", escape(movie.Title), movie.ImdbID, escape(movie.PhysicalRelease.Format("02 Jan 2006"))))
			}
		}

		msg.Text = text.String()
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = true
		sendMessage(msg, bot)
	}
}

func sendLibrary(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool { return movies[i].Title < movies[j].Title })

	for i := 0; i < len(movies); i += maxItems {
		end := i + maxItems
		if end > len(movies) {
			end = len(movies)
		}

		var text strings.Builder
		for _, movie := range movies[i:end] {
			text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_\n", escape(movie.Title), movie.ImdbID, movie.Year))
		}

		msg.Text = text.String()
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = true
		sendMessage(msg, bot)
	}
}

func sendLibraryDownloaded(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool { return movies[i].Title < movies[j].Title })

	var filteredMovies []*radarr.Movie
	for _, movie := range movies {
		if movie.SizeOnDisk > 0 {
			filteredMovies = append(filteredMovies, movie)
		}
	}

	for i := 0; i < len(filteredMovies); i += maxItems {
		end := i + maxItems
		if end > len(filteredMovies) {
			end = len(filteredMovies)
		}

		var text strings.Builder
		for _, movie := range filteredMovies[i:end] {
			text.WriteString(fmt.Sprintf("[%v](https://www.imdb.com/title/%v) \\- _%v_ \\- _%v_\n", escape(movie.Title), movie.ImdbID, movie.Year, escape(byteCountSI(int64(movie.SizeOnDisk)))))
		}

		msg.Text = text.String()
		msg.ParseMode = "MarkdownV2"
		msg.DisableWebPagePreview = true
		sendMessage(msg, bot)
	}
}

func sendLibraryAsInlineKeyboard(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool { return movies[i].Title < movies[j].Title })

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i, movie := range movies {
		if i > 0 && i%maxItems == 0 {
			inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
			msg.ReplyMarkup = inlineKeyboard
			sendMessage(msg, bot)
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
		sendMessage(msg, bot)
	}
}

func prepareRootFolders(rootFolders []*radarr.RootFolder) (msgtext string) {
	maxLength := 0
	var text strings.Builder
	disks := make(map[string]string, len(rootFolders))
	for _, disk := range rootFolders {
		path := disk.Path
		freeSpace := disk.FreeSpace
		disks[fmt.Sprintf("%v:", path)] = escape(byteCountSI(freeSpace))

		length := len(path)
		if maxLength < length {
			maxLength = length
		}
	}

	formatter := fmt.Sprintf("`%%-%dv%%11v`\n", maxLength+1)
	for key, value := range disks {
		text.WriteString(fmt.Sprintf(formatter, key, value))
	}
	return text.String()
}

func clearState() {
	userActiveCommand = make(map[int64]string)
	addMovieUserStates = make(map[int64]userAddMovie)
	deleteMovieUserStates = make(map[int64]userDeleteMovie)
}

func sendMessage(msg tgbotapi.Chattable, bot *tgbotapi.BotAPI) {
	_, err := bot.Send(msg)
	if err != nil {
		log.Println("Error sending message:", err)
	}
}

func escape(text string) string {
	var specialChars = "()[]{}_-*~`><&#+=|!.\\"
	var escaped strings.Builder
	for _, ch := range text {
		if strings.ContainsRune(specialChars, ch) {
			escaped.WriteRune('\\')
		}
		escaped.WriteRune(ch)
	}
	return escaped.String()
}

func byteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
