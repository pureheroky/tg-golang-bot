package models

import "sync"

type DataStore struct {
	sync.RWMutex
	ProjectsData       []map[string]interface{}
	GitData            map[string][]map[string]string
	RequestData        []string
	UserProjectIndex   map[int]int
	UserGitCommitIndex map[int]int
	Projects           map[int][]string
	Git                map[string][]map[string]string
}

type SkillsResponse struct {
	Data   string `json:"data"`
	Status int    `json:"status"`
}

type AwaitingRequests struct {
	sync.RWMutex
	M map[int64]bool
}