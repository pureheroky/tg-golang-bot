package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
