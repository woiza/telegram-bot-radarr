package bot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woiza/telegram-bot-radarr/pkg/utils"
	"golift.io/starr/radarr"
)

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

func (b *Bot) sendMovies(movies []*radarr.Movie, msg *tgbotapi.MessageConfig, bot *tgbotapi.BotAPI) {
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

func (b *Bot) sendMoviesAsInlineKeyboard(movies []*radarr.Movie, msg *tgbotapi.EditMessageTextConfig) {
	sort.SliceStable(movies, func(i, j int) bool {
		return utils.IgnoreArticles(strings.ToLower(movies[i].Title)) < utils.IgnoreArticles(strings.ToLower(movies[j].Title))
	})

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for i, movie := range movies {
		if i > 0 && i%b.Config.MaxItems == 0 {
			inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
			msg.ReplyMarkup = &inlineKeyboard
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
		msg.ReplyMarkup = &inlineKeyboard
		b.sendMessage(msg)
	}
}

func (b *Bot) getMoviesAsInlineKeyboard(movies []*radarr.Movie) [][]tgbotapi.InlineKeyboardButton {

	var inlineKeyboard [][]tgbotapi.InlineKeyboardButton

	for _, movie := range movies {
		button := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%v - %v", movie.Title, movie.Year),
			strconv.Itoa(int(movie.TmdbID)),
		)
		row := []tgbotapi.InlineKeyboardButton{button}
		inlineKeyboard = append(inlineKeyboard, row)
	}

	return inlineKeyboard
}
