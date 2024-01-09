package bot

import (
	"fmt"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golift.io/starr"
	"golift.io/starr/radarr"

	"github.com/woiza/telegram-bot-radarr/pkg/config"
)

type userAddMovie struct {
	searchResults map[string]*radarr.Movie
	movie         *radarr.Movie
	confirmation  bool
	profileID     *int64
	path          *string
	allTags       []*starr.Tag
	selectedTags  []*starr.Tag
	tagDone       bool
	movieAdded    bool
}

type userDeleteMovie struct {
	library                map[string]*radarr.Movie
	searchResultsInLibrary map[string]*radarr.Movie
	movie                  *radarr.Movie
	confirmation           bool
	chatID                 int64
	messageID              int
}

type userLibrary struct {
	libraryFiltered        map[string]*radarr.Movie
	searchResultsInLibrary map[string]*radarr.Movie
	filter                 string
	qualityProfiles        []*radarr.QualityProfile
	allTags                []*starr.Tag
	movie                  *radarr.Movie
	lastSearch             time.Time
	confirmation           bool
	chatID                 int64
	messageID              int
}

type Bot struct {
	Config            *config.Config
	Bot               *tgbotapi.BotAPI
	RadarrServer      *radarr.Radarr
	ActiveCommand     map[int64]string
	AddMovieStates    map[int64]*userAddMovie
	DeleteMovieStates map[int64]*userDeleteMovie
	LibraryStates     map[int64]*userLibrary
	// Mutexes for synchronization
	muActiveCommand     sync.Mutex
	muAddMovieStates    sync.Mutex
	muDeleteMovieStates sync.Mutex
	muLibraryStates     sync.Mutex
}

func New(config *config.Config, botAPI *tgbotapi.BotAPI, radarrServer *radarr.Radarr) *Bot {
	return &Bot{
		Config:            config,
		Bot:               botAPI,
		RadarrServer:      radarrServer,
		ActiveCommand:     make(map[int64]string),
		AddMovieStates:    make(map[int64]*userAddMovie),
		DeleteMovieStates: make(map[int64]*userDeleteMovie),
		LibraryStates:     make(map[int64]*userLibrary),
	}
}

func (b *Bot) HandleUpdates(updates <-chan tgbotapi.Update) {
	for update := range updates {
		b.HandleUpdate(update)
	}
}

func (b *Bot) HandleUpdate(update tgbotapi.Update) {
	userID, err := b.getUserID(update)
	if err != nil {
		fmt.Printf("Cannot handle update: %v", err)
		return
	}

	if update.Message != nil && !b.Config.AllowedUserIDs[userID] {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Access denied. You are not authorized.")
		b.sendMessage(msg)
		return
	}

	activeCommand, _ := b.getActiveCommand(userID)

	if update.CallbackQuery != nil {
		switch activeCommand {
		case "ADDMOVIE":
			if !b.addMovie(update) {
				return
			}
		case "DELETEMOVIE":
			if !b.deleteMovie(update) {
				return
			}
		case "LIBRARYMENU":
			if !b.libraryMenu(update) {
				return
			}
		case "LIBRARYFILTERED":
			if !b.libraryFiltered(update) {
				return
			}
		default:
			b.clearState(update)
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "I am not sure what you mean.\nAll commands have been cleared")
			b.sendMessage(msg)
			break
		}
	}

	if update.Message == nil { // ignore any non-Message Updates
		return
	}

	// If no command was passed, handle a search command.
	if update.Message.Entities == nil {
		update.Message.Text = fmt.Sprintf("/q %s", update.Message.Text)
		update.Message.Entities = []tgbotapi.MessageEntity{{
			Type:   "bot_command",
			Length: 2, // length of the command `/q`
		}}
	}

	if update.Message.IsCommand() {
		b.handleCommand(b.Bot, update, b.RadarrServer)
	}
}

func (b *Bot) clearState(update tgbotapi.Update) {
	userID, err := b.getUserID(update)
	if err != nil {
		fmt.Printf("Cannot clear state: %v", err)
		return
	}

	// Safely clear states using mutexes
	b.muActiveCommand.Lock()
	defer b.muActiveCommand.Unlock()

	delete(b.ActiveCommand, userID)

	b.muAddMovieStates.Lock()
	defer b.muAddMovieStates.Unlock()

	delete(b.AddMovieStates, userID)

	b.muDeleteMovieStates.Lock()
	defer b.muDeleteMovieStates.Unlock()

	delete(b.DeleteMovieStates, userID)

	b.muLibraryStates.Lock()
	defer b.muLibraryStates.Unlock()

	delete(b.LibraryStates, userID)
}

func (b *Bot) getUserID(update tgbotapi.Update) (int64, error) {
	var userID int64
	if update.Message != nil {
		userID = update.Message.From.ID
	}
	if update.CallbackQuery != nil {
		userID = update.CallbackQuery.From.ID
	}
	if userID == 0 {
		return 0, fmt.Errorf("no user ID found in Message and CallbackQuery")
	}
	return userID, nil
}

func (b *Bot) getActiveCommand(userID int64) (string, bool) {
	b.muActiveCommand.Lock()
	defer b.muActiveCommand.Unlock()
	cmd, exists := b.ActiveCommand[userID]
	return cmd, exists
}

func (b *Bot) setActiveCommand(userID int64, command string) {
	b.muActiveCommand.Lock()
	defer b.muActiveCommand.Unlock()
	b.ActiveCommand[userID] = command
}

func (b *Bot) getAddMovieState(userID int64) (*userAddMovie, bool) {
	b.muAddMovieStates.Lock()
	defer b.muAddMovieStates.Unlock()
	state, exists := b.AddMovieStates[userID]
	return state, exists
}

func (b *Bot) setAddMovieState(userID int64, state *userAddMovie) {
	b.muAddMovieStates.Lock()
	defer b.muAddMovieStates.Unlock()
	b.AddMovieStates[userID] = state
}

func (b *Bot) getDeleteMovieState(userID int64) (*userDeleteMovie, bool) {
	b.muDeleteMovieStates.Lock()
	defer b.muDeleteMovieStates.Unlock()
	state, exists := b.DeleteMovieStates[userID]
	return state, exists
}

func (b *Bot) setDeleteMovieState(userID int64, state *userDeleteMovie) {
	b.muDeleteMovieStates.Lock()
	defer b.muDeleteMovieStates.Unlock()
	b.DeleteMovieStates[userID] = state
}

func (b *Bot) getLibraryState(userID int64) (*userLibrary, bool) {
	b.muLibraryStates.Lock()
	defer b.muLibraryStates.Unlock()
	state, exists := b.LibraryStates[userID]
	return state, exists
}

func (b *Bot) setLibraryState(userID int64, state *userLibrary) {
	b.muLibraryStates.Lock()
	defer b.muLibraryStates.Unlock()
	b.LibraryStates[userID] = state
}
