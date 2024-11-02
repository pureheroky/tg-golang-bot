package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/pureheroky/tg-golang-bot/models"
)

func makeRequest(method, url, token string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return responseBody, nil
}

func getJSONData(url, token string, target interface{}) error {
	response, err := makeRequest("GET", url, token, nil)
	if err != nil {
		return err
	}

	err = json.Unmarshal(response, target)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return nil
}

func GetGitConcurrently(apiUrl string, username string, token string, dataStore *models.DataStore) (map[string][]map[string]string, error) {
	projects := dataStore.ProjectsData
	projectNames := []string{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	output := make(map[string][]map[string]string)

	if len(dataStore.GitData) == 0 {
		for _, value := range projects {
			if name, ok := value["name"].(string); ok {
				projectNames = append(projectNames, name)
			}
		}

		for _, repoName := range projectNames {
			wg.Add(1)

			go func(repoName string) {
				defer wg.Done()
				url := fmt.Sprintf("%s/repos/%s/%s/commits", apiUrl, username, repoName)
				var data []map[string]interface{}

				err := getJSONData(url, token, &data)
				if err != nil {
					log.Printf("Error fetching commits for %s: %v", repoName, err)
					return
				}

				mu.Lock()
				defer mu.Unlock()

				for index, val := range data {
					if index >= 5 {
						break
					}
					commit, ok := val["commit"].(map[string]interface{})
					if !ok {
						continue
					}
					authorMap, ok := commit["author"].(map[string]interface{})
					if !ok {
						continue
					}
					committerMap, ok := commit["committer"].(map[string]interface{})
					if !ok {
						continue
					}
					authorName, _ := authorMap["name"].(string)
					message, _ := commit["message"].(string)
					date, _ := committerMap["date"].(string)

					if output[repoName] == nil {
						output[repoName] = make([]map[string]string, 0)
					}

					output[repoName] = append(output[repoName], map[string]string{
						"author":  authorName,
						"message": message,
						"date":    date,
					})
				}
			}(repoName)
		}

		wg.Wait()
		dataStore.GitData = output
	} else {
		output = dataStore.GitData
	}

	return output, nil
}

func Request(text string, id int64, username string, dataStore *models.DataStore) {
	dataStore.RequestData = []string{username, fmt.Sprint(id), text}
}

func GetSkills(url string) ([]string, error) {
	var responseObj models.SkillsResponse
	err := getJSONData(url, "", &responseObj)
	if err != nil {
		log.Printf("Error fetching skills data: %v", err)
		return nil, err
	}

	skillsMap := make(map[string]struct{})
	re := regexp.MustCompile(`'[^']+'|\S+`)
	matches := re.FindAllString(responseObj.Data, -1)
	for _, match := range matches {
		cleanMatch := strings.Trim(match, "[]'")
		skillsMap[cleanMatch] = struct{}{}
	}

	skills := make([]string, 0, len(skillsMap))
	for skill := range skillsMap {
		skills = append(skills, skill)
	}

	return skills, nil
}

func GetProjects(apiUrl string, username string, token string, dataStore *models.DataStore) (map[int][]string, error) {
	output := make(map[int][]string)

	dataUrl := fmt.Sprintf("%s/users/%s/repos", apiUrl, username)
	var data []map[string]interface{}

	if len(dataStore.ProjectsData) == 0 {
		err := getJSONData(dataUrl, token, &data)
		if err != nil {
			log.Printf("Error fetching projects data: %v", err)
			return nil, err
		}
		dataStore.ProjectsData = data
	} else {
		data = dataStore.ProjectsData
	}

	for index, value := range data {
		id, _ := value["node_id"].(string)
		name, _ := value["name"].(string)
		url, _ := value["url"].(string)
		createdAt, _ := value["created_at"].(string)
		defaultBranch, _ := value["default_branch"].(string)
		language, _ := value["language"].(string)

		output[index] = []string{name, id, url, language, createdAt, defaultBranch}
	}

	return output, nil
}

func FormatProjectMessage(project []string) string {
	if len(project) < 6 {
		return "Invalid project data."
	}
	return fmt.Sprintf(
		`
<b><i>Title: <code>%s</code></i></b>
<b>ID: %s</b>
<b>URL: <a href='https://github.com/pureheroky/%s'>link</a></b>
<b>Language: %s</b>
<b>Creation date: %s</b>
<b>Default branch: %s</b>
`,
		project[0], // name
		project[1], // id
		project[2], // url
		project[3], // language
		project[4], // created_at
		project[5], // default_branch
	)
}

func GetWelcomeMessage() string {
	return `
<b><i>pureheroky</i></b> was created to help people contact/learn about me.

It has a couple of different <strong>buttons</strong> that show any information (knowledge stacks, projects, etc.).

Command list:

<code><b>Request:</b>
create a job request</code>

<code><b>Git:</b>
get last commits/accessible repos</code>

<code><b>Skills:</b>
get knowledge stack</code>

<code><b>Projects:</b>
get list of complete/under development projects</code>

Bot will be open source someday (look on my <a href='https://pureheroky.com'>website</a> or in the bot description)
`
}

func FormatGitMessagePage(git map[string][]map[string]string, pageIndex int, pageSize int) string {
	message := ""
	keys := make([]string, 0, len(git))

	for key := range git {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	start := pageIndex * pageSize
	end := start + pageSize
	if start >= len(keys) {
		return "No more commits on this page."
	}
	if end > len(keys) {
		end = len(keys)
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

func SetupLogging() (*log.Logger, *log.Logger) {
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		os.Mkdir("logs", os.ModePerm)
	}

	errorLogFile, err := os.OpenFile("logs/error.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Failed to open error log file:", err)
		os.Exit(1)
	}
	errorLogger := log.New(errorLogFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	workLogFile, err := os.OpenFile("logs/work.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		errorLogger.Println("Failed to open work log file:", err)
		os.Exit(1)
	}
	workLogger := log.New(workLogFile, "INFO: ", log.Ldate|log.Ltime)

	return errorLogger, workLogger
}

func LoadData(dataStore *models.DataStore, apiUrl, username, token string, errorLogger *log.Logger) error {
	dataStore.Lock()
	defer dataStore.Unlock()

	var err error
	dataStore.Projects, err = GetProjects(apiUrl, username, token, dataStore)
	if err != nil {
		errorLogger.Println("failed to get projects: %w", err)
		return fmt.Errorf("failed to get projects: %w", err)
	}

	dataStore.Git, err = GetGitConcurrently(apiUrl, username, token, dataStore)
	if err != nil {
		errorLogger.Println("failed to get git data: %w", err)
		return fmt.Errorf("failed to get git data: %w", err)
	}

	return nil
}
