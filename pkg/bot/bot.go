package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golift.io/starr"
	"golift.io/starr/radarr"

	"github.com/woiza/telegram-bot-radarr/pkg/config"
)

const (
	AddMovieCommand         = "ADDMOVIE"
	DeleteMovieCommand      = "DELETEMOVIE"
	LibraryMenuCommand      = "LIBRARYMENU"
	LibraryFilteredCommand  = "LIBRARYFILTERED"
	LibraryMovieEditCommand = "LIBRARYMOVIEEDIT"
	CommandsClearedMessage  = "I am not sure what you mean.\nAll commands have been cleared"
)

type userAddMovie struct {
	searchResults   map[string]*radarr.Movie
	movie           *radarr.Movie
	allProfiles     []*radarr.QualityProfile
	profileID       int64
	allRootFolders  []*radarr.RootFolder
	rootFolder      *radarr.RootFolder
	allTags         []*starr.Tag
	selectedTags    []int
	monitored       bool
	addMovieOptions *radarr.AddMovieOptions
	chatID          int64
	messageID       int
}

type userDeleteMovie struct {
	library            map[string]*radarr.Movie
	moviesForSelection []*radarr.Movie // Movies to select from, either whole library or search results
	selectedMovies     []*radarr.Movie
	chatID             int64
	messageID          int
	page               int
}

type userLibrary struct {
	library                []*radarr.Movie
	libraryFiltered        map[string]*radarr.Movie
	searchResultsInLibrary []*radarr.Movie
	filter                 string
	qualityProfiles        []*radarr.QualityProfile
	selectedQualityProfile int64
	allTags                []*starr.Tag
	selectedTags           []int
	selectedMonitoring     bool
	movie                  *radarr.Movie
	lastSearch             time.Time
	chatID                 int64
	messageID              int
	page                   int
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

type Command interface {
	GetChatID() int64
	GetMessageID() int
}

// Implement the interface for userLibrary
func (c *userLibrary) GetChatID() int64 {
	return c.chatID
}

func (c *userLibrary) GetMessageID() int {
	return c.messageID
}

// Implement the interface for userDelete
func (c *userDeleteMovie) GetChatID() int64 {
	return c.chatID
}

func (c *userDeleteMovie) GetMessageID() int {
	return c.messageID
}

// Implement the interface for userAddMovie
func (c *userAddMovie) GetChatID() int64 {
	return c.chatID
}

func (c *userAddMovie) GetMessageID() int {
	return c.messageID
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
	chatID, err := b.getChatID(update)
	if err != nil {
		fmt.Printf("Cannot handle update: %v", err)
		return
	}

	if update.Message != nil && !b.Config.AllowedChatIDs[chatID] {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Access denied. You are not authorized.")
		b.sendMessage(msg)
		return
	}

	activeCommand, _ := b.getActiveCommand(chatID)

	if update.CallbackQuery != nil {
		switch activeCommand {
		case AddMovieCommand:
			if !b.addMovie(update) {
				return
			}
		case DeleteMovieCommand:
			if !b.deleteMovie(update) {
				return
			}
		case LibraryMenuCommand:
			if !b.libraryMenu(update) {
				return
			}
		case LibraryFilteredCommand:
			if !b.libraryFiltered(update) {
				return
			}
		case LibraryMovieEditCommand:
			if !b.libraryMovieEdit(update) {
				return
			}
		default:
			b.clearState(update)
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, CommandsClearedMessage)
			b.sendMessage(msg)
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
		b.handleCommand(update, b.RadarrServer)
	}
}

func (b *Bot) clearState(update tgbotapi.Update) {
	chatID, err := b.getChatID(update)
	if err != nil {
		fmt.Printf("Cannot clear state: %v", err)
		return
	}

	// Safely clear states using mutexes
	b.muActiveCommand.Lock()
	defer b.muActiveCommand.Unlock()

	delete(b.ActiveCommand, chatID)

	b.muAddMovieStates.Lock()
	defer b.muAddMovieStates.Unlock()

	delete(b.AddMovieStates, chatID)

	b.muDeleteMovieStates.Lock()
	defer b.muDeleteMovieStates.Unlock()

	delete(b.DeleteMovieStates, chatID)

	b.muLibraryStates.Lock()
	defer b.muLibraryStates.Unlock()

	delete(b.LibraryStates, chatID)
}

func (b *Bot) getChatID(update tgbotapi.Update) (int64, error) {
	var chatID int64
	if update.Message != nil {
		chatID = update.Message.Chat.ID
	}
	if update.CallbackQuery != nil {
		chatID = update.CallbackQuery.Message.Chat.ID
	}
	if chatID == 0 {
		return 0, fmt.Errorf("no user ID found in Message and CallbackQuery")
	}
	return chatID, nil
}

func (b *Bot) getActiveCommand(chatID int64) (string, bool) {
	b.muActiveCommand.Lock()
	defer b.muActiveCommand.Unlock()
	cmd, exists := b.ActiveCommand[chatID]
	return cmd, exists
}

func (b *Bot) setActiveCommand(chatID int64, command string) {
	b.muActiveCommand.Lock()
	defer b.muActiveCommand.Unlock()
	b.ActiveCommand[chatID] = command
}

func (b *Bot) getAddMovieState(chatID int64) (*userAddMovie, bool) {
	b.muAddMovieStates.Lock()
	defer b.muAddMovieStates.Unlock()
	state, exists := b.AddMovieStates[chatID]
	return state, exists
}

func (b *Bot) setAddMovieState(chatID int64, state *userAddMovie) {
	b.muAddMovieStates.Lock()
	defer b.muAddMovieStates.Unlock()
	b.AddMovieStates[chatID] = state
}

func (b *Bot) getDeleteMovieState(chatID int64) (*userDeleteMovie, bool) {
	b.muDeleteMovieStates.Lock()
	defer b.muDeleteMovieStates.Unlock()
	state, exists := b.DeleteMovieStates[chatID]
	return state, exists
}

func (b *Bot) setDeleteMovieState(chatID int64, state *userDeleteMovie) {
	b.muDeleteMovieStates.Lock()
	defer b.muDeleteMovieStates.Unlock()
	b.DeleteMovieStates[chatID] = state
}

func (b *Bot) getLibraryState(chatID int64) (*userLibrary, bool) {
	b.muLibraryStates.Lock()
	defer b.muLibraryStates.Unlock()
	state, exists := b.LibraryStates[chatID]
	return state, exists
}

func (b *Bot) setLibraryState(chatID int64, state *userLibrary) {
	b.muLibraryStates.Lock()
	defer b.muLibraryStates.Unlock()
	b.LibraryStates[chatID] = state
}

func (b *Bot) sendMessage(msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	message, err := b.Bot.Send(msg)
	if err != nil {
		log.Println("Error sending message:", err)
	}
	return message, err
}

func (b *Bot) sendMessageWithEdit(command Command, text string) {
	editMsg := tgbotapi.NewEditMessageText(
		command.GetChatID(),
		command.GetMessageID(),
		text,
	)
	_, err := b.sendMessage(editMsg)
	if err != nil {
		log.Printf("Error editing message: %v", err)
	}
}

func (b *Bot) sendMessageWithEditAndKeyboard(command Command, keyboard tgbotapi.InlineKeyboardMarkup, text string) {
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		command.GetChatID(),
		command.GetMessageID(),
		text,
		keyboard,
	)
	_, err := b.sendMessage(editMsg)
	if err != nil {
		log.Printf("Error editing message with keyboard: %v", err)
	}
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}
