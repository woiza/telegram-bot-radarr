package bot

import (
	"fmt"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr"
	"golift.io/starr/radarr"
)

func (b *Bot) handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, r *radarr.Radarr) {

	userID, err := b.getUserID(update)
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
		b.AddMovieStates[userID] = &command
		b.setActiveCommand(userID, AddMovieCommand)
		b.sendSearchResults(command.searchResults, &msg)

	case "movies", "library", "l":
		b.setActiveCommand(userID, LibraryMenuCommand)
		b.processLibraryCommand(update, userID, r)

	case "delete", "remove", "Delete", "Remove", "d":
		b.setActiveCommand(userID, DeleteMovieCommand)
		b.processDeleteCommand(update, userID, r)

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
			b.sendMessage(msg)
			break
		}
		b.sendUpcoming(upcoming, &msg, bot)

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

	case "searchmonitored":
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

	case "system", "System", "systemstatus", "Systemstatus":
		status, err := r.GetSystemStatus()
		if err != nil {
			msg.Text = err.Error()
			fmt.Println(err)
			b.sendMessage(msg)
			break
		}
		message := prettyPrint(status)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
		b.sendMessage(msg)

	case "getid", "id":
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Your user ID: %d", userID))
		b.sendMessage(msg)

	default:
		msg.Text = fmt.Sprintf("Hello %v!\n", update.Message.From)
		msg.Text += "Here's a list of commands at your disposal:\n\n"
		msg.Text += "/q [movie] - searches a movie \n"
		msg.Text += "/library [movie] - manage movie(s)\n"
		msg.Text += "/delete [movie] - deletes a movie\n"
		msg.Text += "/clear - deletes all sent commands\n"
		msg.Text += "/free  - lists free disk space \n"
		msg.Text += "/up\t\t\t\t - lists upcoming movies in the next 30 days\n"
		msg.Text += "/rss \t\t - performs a RSS sync\n"
		msg.Text += "/searchmonitored - searches all monitored movies\n"
		msg.Text += "/updateall - updates metadata and rescans files/folders\n"
		msg.Text += "/system - shows your Radarr configuration\n"
		msg.Text += "/id - shows your Telegram user ID"
		b.sendMessage(msg)
	}
}
