package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var ProjectsData []map[string]interface{}
var GitData map[string][]map[string]string
var RequestData []string

type SkillsResponse struct {
	Data   string `json:"data"`
	Status int    `json:"status"`
}

func getGit(apiUrl string, username string, token string) map[string][]map[string]string {
	projects := ProjectsData
	projectNames := []string{}
	headers := make(http.Header)
	headers.Add("Authorization", "token "+token)
	var data []map[string]interface{}
	output := make(map[string][]map[string]string)

	if len(GitData) <= 0 {
		for _, value := range projects {
			projectNames = append(projectNames, value["name"].(string))
		}

		for _, repo_name := range projectNames {
			url := apiUrl + "/repos/" + username + "/" + repo_name + "/commits"
			response, err := http.Get(url)

			if err != nil {
				fmt.Println("Error - ", err)
				return map[string][]map[string]string{}
			}

			defer response.Body.Close()

			body, err := io.ReadAll(response.Body)
			if err != nil {
				fmt.Println("Error - ", err)
				return map[string][]map[string]string{}
			}

			err = json.Unmarshal(body, &data)

			if err != nil {
				fmt.Println("Error - ", err)
				return map[string][]map[string]string{}
			}

			for index, val := range data {
				commit := val["commit"].(map[string]interface{})
				if index != 5 {
					author := commit["author"].(map[string]interface{})["name"].(string)
					message := commit["message"].(string)
					date := commit["committer"].(map[string]interface{})["date"].(string)

					if output[repo_name] == nil {
						output[repo_name] = make([]map[string]string, 0)
					}

					output[repo_name] = append(output[repo_name], map[string]string{
						"author":  author,
						"message": message,
						"date":    date,
					})
				}
			}
		}
		GitData = output
	} else {
		output = GitData
	}

	return output
}

func Request(text string, id int64, username string) {
	RequestData = []string{username, fmt.Sprint(id), text}
}

func getSkills(url string) []string {
	var responseObj SkillsResponse

	response, err := http.Get(url)
	if err != nil {
		fmt.Println("Error - ", err)
		return []string{}
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Error reading response body - ", err)
		return []string{}
	}

	err = json.Unmarshal(body, &responseObj)
	if err != nil {
		fmt.Println("Error in JSON unmarshalling - ", err)
		return []string{}
	}

	re := regexp.MustCompile(`'[^']+'|\S+`)
	matches := re.FindAllString(responseObj.Data, -1)

	var skills []string
	for _, match := range matches {
		cleanMatch := strings.Trim(match, "[]'")
		skills = append(skills, cleanMatch)
	}

	return skills
}

func getProjects(apiUrl string, username string, token string) map[int][]string {
	output := make(map[int][]string)

	data_url := apiUrl + "/users/" + username + "/repos"
	headers := make(http.Header)
	headers.Add("Authorization", "token "+token)
	var data []map[string]interface{}

	if len(ProjectsData) <= 0 {
		response, err := http.Get(data_url)
		if err != nil {
			fmt.Println("Error - ", err)
			return map[int][]string{}
		}

		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			fmt.Println("Error - ", err)
			return map[int][]string{}
		}

		err = json.Unmarshal(body, &data)
		if err != nil {
			fmt.Println("Error in json - ", err)
		}
		ProjectsData = data
	} else {
		data = ProjectsData
	}

	for index, value := range data {
		id, _ := value["node_id"].(string)
		name, _ := value["name"].(string)
		url, _ := value["url"].(string)
		created_at, _ := value["created_at"].(string)
		default_branch, _ := value["default_branch"].(string)
		language, _ := value["language"].(string)

		output[index] = []string{name, id, url, language, created_at, default_branch}
	}

	return output
}

func formatProjectMessage(project []string) string {
	if len(project) < 6 {
		return "Invalid project data."
	}
	return fmt.Sprintf(
		"\n\n\n<b><i>Title: <code>%s</code></i></b>\n"+
			"<b>ID: %s</b>\n"+
			"<b>URL: <a href='https://github.com/pureheroky/%s'>link</a></b>\n"+
			"<b>Language: %s</b>\n"+
			"<b>Creation date: %s</b>\n"+
			"<b>Default branch: %s</b>\n\n\n",
		project[0], // name
		project[1], // id
		project[2], // url
		project[3], // language
		project[4], // created_at
		project[5], // default_branch,
	)
}

func formatGitMessagePage(git map[string][]map[string]string, pageIndex int, pageSize int) string {
	message := ""
	keys := make([]string, 0, len(git))

	for key := range git {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	start := pageIndex * pageSize
	end := (pageIndex + 1) * pageSize
	if end > len(keys) {
		end = len(keys)
	}

	if start >= len(keys) {
		return "No more commits on this page."
	}

	for _, key := range keys[start:end] {
		message += fmt.Sprintf("\n\n<b><i>Title: <code>%s</code></i></b>\n", key)
		for _, value := range git[key] {
			message += fmt.Sprintf("\nAuthor: <b>%s</b>\n", value["author"])
			message += fmt.Sprintf("Date: <b>%s</b>\n", value["date"])
			message += fmt.Sprintf("Message: <b>%s</b>\n", value["message"])
		}
		message += "\n\n"
	}

	return message
}

func main() {
	err := godotenv.Load(".env")

	if err != nil {
		fmt.Println("Error - ", err)
	}

	botToken := os.Getenv("TOKEN")
	skillsURL := os.Getenv("SKILLS_URL")
	var userProjectIndex = map[int]int{}
	var userGitCommitIndex = map[int]int{}

	var projects map[int][]string
	var git map[string][]map[string]string

	bot, err := telego.NewBot(botToken, telego.WithDefaultDebugLogger())
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	updates, _ := bot.UpdatesViaLongPolling(nil)

	bh, _ := th.NewBotHandler(bot, updates)
	defer bh.Stop()
	defer bot.StopLongPolling()

	var awaitingRequests = make(map[int64]bool)

	username := "pureheroky"
	gitApiUrl := "https://api.github.com"
	projects = getProjects(gitApiUrl, username, os.Getenv("GIT_TOKEN"))
	git = getGit(gitApiUrl, username, os.Getenv("GIT_TOKEN"))

	// COMMANDS BLOCK
	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		_ = bot.DeleteMessage(tu.Delete(
			tu.ID(update.Message.Chat.ID),
			update.Message.MessageID,
		))

		messageText := "\n\n<b><i>pureheroky</i></b> was created to help people contact/learn about me.\n\nIt has a couple of different <strong>buttons</strong> that show any information (knowledge stacks, projects, etc.).\n\nCommand list:\n\n\n<code><b>Request:</b> \ncreate a job request</code>\n\n<code><b>Git:</b> \nget last commits/accessible repos</code>\n\n<code><b>Skills:</b> \nget knowledge stack</code>\n\n<code><b>Projects:</b> \nget list of complete/under development projects\n\n</code>\n\nBot will be opensource someday (look on my <a href='https://pureheroky.com'>website</a> or in the bot description)"

		message := tu.Message(
			tu.ID(update.Message.Chat.ID),
			messageText,
		)
		message.ParseMode = telego.ModeHTML
		message = message.WithReplyMarkup(
			tu.InlineKeyboard(
				tu.InlineKeyboardCols(2,
					tu.InlineKeyboardButton("request").WithCallbackData("request"),
					tu.InlineKeyboardButton("git").WithCallbackData("git"),
					tu.InlineKeyboardButton("skills").WithCallbackData("skills"),
					tu.InlineKeyboardButton("projects").WithCallbackData("projects"))...,
			))
		_, _ = bot.SendMessage(message)
	}, th.CommandEqual("start"))

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		parts := strings.Fields(update.Message.Text)
		userId := parts[1]
		id, _ := strconv.ParseInt(userId, 10, 64)

		message := tu.Message(
			tu.ID(id),
			"Your request was accepted!\n\nDeveloper will soon contant with you\n\nThis message will be deleted after <b>2 minutes</b>",
		)

		message.ParseMode = telego.ModeHTML

		sentMessage, _ := bot.SendMessage(message)

		time.AfterFunc(2*time.Minute, func() {
			_ = bot.DeleteMessage(tu.Delete(
				tu.ID(update.Message.Chat.ID),
				sentMessage.MessageID,
			))
		})

	}, th.CommandEqual("accept"))

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		parts := strings.Fields(update.Message.Text)
		userId := parts[1]
		answer := ""
		id, _ := strconv.ParseInt(userId, 10, 64)

		for i, value := range parts {
			if i > 1 {
				answer += value + " "
			}
		}

		message := tu.Message(
			tu.ID(id),
			"Your request was declined!\n\nDeveloper message: \n"+answer+"\n\nThis message will be deleted after <b>2 minutes</b>",
		)

		message.ParseMode = telego.ModeHTML

		sentMessage, _ := bot.SendMessage(message)

		time.AfterFunc(2*time.Minute, func() {
			_ = bot.DeleteMessage(tu.Delete(
				tu.ID(update.Message.Chat.ID),
				sentMessage.MessageID,
			))
		})

	}, th.CommandEqual("decline"))

	bh.HandleCallbackQuery(func(bot *telego.Bot, query telego.CallbackQuery) {
		messageText := ""
		chatID := query.Message.GetChat().ID
		markup := tu.InlineKeyboard(tu.InlineKeyboardRow(tu.InlineKeyboardButton("back").WithCallbackData("back")))

		const pageSize = 1
		currentIndex := userGitCommitIndex[int(chatID)]
		totalCommits := len(git)

		projectMarkup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("previous").WithCallbackData("previous_project"),
				tu.InlineKeyboardButton("next").WithCallbackData("next_project"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("back").WithCallbackData("back"),
			),
		)

		gitMarkup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("previous").WithCallbackData("previous_git"),
				tu.InlineKeyboardButton("next").WithCallbackData("next_git"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("back").WithCallbackData("back"),
			),
		)

		editedMessage := telego.EditMessageTextParams{
			ChatID:      tu.ID(query.Message.GetChat().ID),
			MessageID:   query.Message.GetMessageID(),
			Text:        messageText,
			ParseMode:   telego.ModeHTML,
			ReplyMarkup: markup,
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
		}

		switch query.Data {
		case "request":
			messageText = "You are on <b>Request</b> page.\n\nIf you want to make a job request, please follow the <b>sample</b>:\n\n\n<b>1. Your name</b>\n<b>2. Direction of the task (any web-development/python apps etc.)</b>\n<b>3. Task description</b>\n<b>4. Ways to contact you</b>\n\n\nRequests not similar to the sample <span class='tg-spoiler'><b>will be ignored.</b></span>"
			editedMessage.Text = messageText
			bot.EditMessageText(&editedMessage)

			awaitingRequests[query.Message.GetChat().ID] = true
		case "skills":
			messageText = "You are on <b>Skills</b> page\nAll my knowledge will be shown here\n\n\n<b><i>Loading skills...</i></b>"
			editedMessage.Text = messageText
			bot.EditMessageText(&editedMessage)

			messageText = "There is my knowledge stack:\n\n\n"
			output := getSkills(skillsURL)
			fmt.Println(output)

			for key, value := range output {
				messageText += fmt.Sprintf("<i>%d</i>. <b>%s</b>\n", key+1, value)
			}

			messageText += "\n\nMore information about the projects can be found <a href='https://pureheroky.com'><b>here</b></a>"
			editedMessage.Text = messageText

		case "git":
			userGitCommitIndex[int(chatID)] = 0
			currentIndex = 0
			messageText = "You are on <b>Git</b> page\nThe latest commits will be shown here\n\n\n<b><i>Loading commits...</i></b>"
			editedMessage.ReplyMarkup = gitMarkup
			bot.EditMessageText(&editedMessage)
			if len(git) > 0 {
				messageText = formatGitMessagePage(git, currentIndex, pageSize)
			} else {
				messageText = "\n\nNo git commits found."
			}

			messageText += "\n<b><i>More information about the projects can be found <a href='https://github.com/pureheroky'>here</a></i></b>\n\n\n"
			editedMessage.Text = messageText
			bot.EditMessageText(&editedMessage)

		case "projects":
			userProjectIndex[int(chatID)] = 0
			messageText = "You are on <b>Projects</b> page\n<b><i>Loading projects...</i></b>\n"
			editedMessage.ReplyMarkup = projectMarkup
			bot.EditMessageText(&editedMessage)
			if len(projects) > 0 {
				messageText += formatProjectMessage(projects[0])
			} else {
				messageText += "\n\nNo projects found."
			}

			editedMessage.Text = messageText

		case "next_git":
			currentIndex := userGitCommitIndex[int(chatID)]
			editedMessage.ReplyMarkup = gitMarkup
			if currentIndex < (totalCommits-1)/pageSize {
				currentIndex++
				userGitCommitIndex[int(chatID)] = currentIndex
				messageText = formatGitMessagePage(git, currentIndex, pageSize)
			} else {
				return
			}
			editedMessage.Text = messageText

		case "previous_git":
			currentIndex := userGitCommitIndex[int(chatID)]
			editedMessage.ReplyMarkup = gitMarkup
			if currentIndex > 0 {
				currentIndex--
				userGitCommitIndex[int(chatID)] = currentIndex
				messageText = formatGitMessagePage(git, currentIndex, pageSize)
			} else {
				return
			}
			editedMessage.Text = messageText

		case "next_project":
			currentIndex := userProjectIndex[int(chatID)]
			editedMessage.ReplyMarkup = projectMarkup

			if currentIndex < len(projects)-1 {
				currentIndex++
				userProjectIndex[int(chatID)] = currentIndex
				messageText += formatProjectMessage(projects[currentIndex])
			} else {
				return
			}

			editedMessage.Text = messageText

		case "previous_project":
			currentIndex := userProjectIndex[int(chatID)]
			editedMessage.ReplyMarkup = projectMarkup

			if currentIndex > 0 {
				currentIndex--
				userProjectIndex[int(chatID)] = currentIndex
				messageText += formatProjectMessage(projects[currentIndex])
			} else {
				return
			}

			editedMessage.Text = messageText

		case "back":
			messageText = fmt.Sprintf("\n\n<b><i>pureheroky</i></b> was created to help people contact/learn about me.\n\nIt has a couple of different <strong>buttons</strong> that show any information (knowledge stacks, projects, etc.).\n\nCommand list:\n\n\n<code><b>REQUEST:</b> \ncreate a job request</code>\n\n<code><b>GIT:</b> \nget last commits/accessible repos</code>\n\n<code><b>SKILLS:</b> \nget knowledge stack</code>\n\n<code><b>PROJECTS:</b> \nget list of complete/under development projects\n\n</code>\n\nBot will be opensource someday (look on my <a href='https://pureheroky.com'>website</a> or in the bot description)")
			editedMessage.Text = messageText
			bot.EditMessageText(&editedMessage)

			if awaitingRequests[query.Message.GetChat().ID] {
				delete(awaitingRequests, query.Message.GetChat().ID)
			}

			editedMessage.ReplyMarkup = tu.InlineKeyboard(
				tu.InlineKeyboardCols(2,
					tu.InlineKeyboardButton("request").WithCallbackData("request"),
					tu.InlineKeyboardButton("git").WithCallbackData("git"),
					tu.InlineKeyboardButton("skills").WithCallbackData("skills"),
					tu.InlineKeyboardButton("projects").WithCallbackData("projects"))...)

			bot.EditMessageText(&editedMessage)
			break
		}

		bot.EditMessageText(&editedMessage)
	})

	bh.Handle(func(bot *telego.Bot, update telego.Update) {
		if awaiting, ok := awaitingRequests[update.Message.Chat.ID]; ok && awaiting {
			requestText := update.Message.Text
			requestID := update.Message.From.ID
			requestUsername := update.Message.From.Username

			delete(awaitingRequests, update.Message.Chat.ID)
			userId, _ := strconv.ParseInt(os.Getenv("USER_ID"), 10, 64)
			messageToMe := tu.Message(
				tu.ID(userId),
				"Request from <code>"+requestUsername+"</code> | <code>"+fmt.Sprint(requestID)+"</code>\n\n\n"+requestText,
			)
			messageToMe.ParseMode = telego.ModeHTML
			_, _ = bot.SendMessage(messageToMe)

			message := tu.Message(
				tu.ID(update.Message.Chat.ID),
				"Thank you for your job request.\n\nYour and this message will be deleted after <b>2 minutes</b>\n\nI'll write you, after reviewing your request",
			)
			message.ParseMode = telego.ModeHTML

			sentMessage, _ := bot.SendMessage(message)

			time.AfterFunc(2*time.Minute, func() {
				_ = bot.DeleteMessage(tu.Delete(
					tu.ID(update.Message.Chat.ID),
					sentMessage.MessageID,
				))
				_ = bot.DeleteMessage(tu.Delete(
					tu.ID(update.Message.Chat.ID),
					update.Message.MessageID,
				))
			})
		} else {
			_ = bot.DeleteMessage(tu.Delete(
				tu.ID(update.Message.Chat.ID),
				update.Message.MessageID,
			))
		}
	})
	bh.Start()
}
