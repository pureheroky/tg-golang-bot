package markup

import (
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

func GetMainMenuMarkup() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardCols(2,
			tu.InlineKeyboardButton("request").WithCallbackData("request"),
			tu.InlineKeyboardButton("git").WithCallbackData("git"),
			tu.InlineKeyboardButton("skills").WithCallbackData("skills"),
			tu.InlineKeyboardButton("projects").WithCallbackData("projects"),
		)...,
	)
}

func GetProjectMarkup() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("previous").WithCallbackData("previous_project"),
			tu.InlineKeyboardButton("next").WithCallbackData("next_project"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("back").WithCallbackData("back"),
		),
	)
}

func GetBackMarkup() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("back").WithCallbackData("back"),
		),
	)
}

func GetGitMarkup() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("previous").WithCallbackData("previous_git"),
			tu.InlineKeyboardButton("next").WithCallbackData("next_git"),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("back").WithCallbackData("back"),
		),
	)
}
