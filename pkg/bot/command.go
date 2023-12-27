package bot

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

func (b *Bot) handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, r *radarr.Radarr) {
	userID, err := getUserID(update)
	if err != nil {
		fmt.Printf("Cannot handle command: %v", err)
		return
	}
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
		b.AddMovieUserStates[userID] = command
		b.UserActiveCommand[userID] = "ADDMOVIE"
		b.sendSearchResults(command.searchResults, &msg)

	case "clear", "cancel", "stop":
		b.clearState(update)
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
		b.DeleteMovieUserStates[userID] = command
		b.UserActiveCommand[userID] = "DELETEMOVIE"
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

func (b *Bot) sendSearchResults(searchResults map[string]*radarr.Movie, msg *tgbotapi.MessageConfig) {
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

func (b *Bot) sendUpcoming(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})
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

func (b *Bot) sendLibrary(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})

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

func (b *Bot) sendLibraryDownloaded(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})

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

func (b *Bot) sendLibraryAsInlineKeyboard(movies []*radarr.Movie, msg *tgbotapi.MessageConfig) {
	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})

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

func (b *Bot) sendMessage(msg tgbotapi.Chattable) {
	_, err := b.Bot.Send(msg)
	if err != nil {
		log.Println("Error sending message:", err)
	}
}
