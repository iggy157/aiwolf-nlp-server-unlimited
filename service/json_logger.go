package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/iggy157/aiwolf-nlp-server-unlimited/model"
)

type JSONLogger struct {
	data             map[string]*JSONLog
	outputDir        string
	templateFilename string
	mu               sync.RWMutex
}

type JSONLog struct {
	id           string
	filename     string
	agents       []any
	winSide      model.Team
	entries      []any
	timestampMap map[string]int64
	requestMap   map[string]any
	mu           sync.Mutex
}

func NewJSONLogger(config model.Config) *JSONLogger {
	return &JSONLogger{
		data:             make(map[string]*JSONLog),
		outputDir:        config.JSONLogger.OutputDir,
		templateFilename: config.JSONLogger.Filename,
	}
}

func (j *JSONLogger) TrackStartGame(id string, agents []*model.Agent) {
	data := &JSONLog{
		id:           id,
		agents:       make([]any, 0),
		entries:      make([]any, 0),
		timestampMap: make(map[string]int64),
		requestMap:   make(map[string]any),
		winSide:      model.T_NONE,
	}
	for _, agent := range agents {
		data.agents = append(data.agents,
			map[string]any{
				"idx":  agent.Idx,
				"team": agent.TeamName,
				"name": agent.OriginalName,
				"role": agent.Role,
			},
		)
	}
	filename := strings.ReplaceAll(j.templateFilename, "{game_id}", data.id)
	filename = strings.ReplaceAll(filename, "{timestamp}", fmt.Sprintf("%d", time.Now().Unix()))
	teams := make(map[string]struct{})
	for _, agent := range data.agents {
		team := agent.(map[string]any)["team"].(string)
		teams[team] = struct{}{}
	}
	teamStr := ""
	for team := range teams {
		if teamStr != "" {
			teamStr += "_"
		}
		teamStr += team
	}
	filename = strings.ReplaceAll(filename, "{teams}", teamStr)
	data.filename = filename

	j.mu.Lock()
	j.data[id] = data
	j.mu.Unlock()
}

func (j *JSONLogger) TrackEndGame(id string, winSide model.Team) {
	j.mu.RLock()
	_, exists := j.data[id]
	j.mu.RUnlock()

	if exists {
		j.saveGameData(id)

		j.mu.Lock()
		delete(j.data, id)
		j.mu.Unlock()
	}
}

func (j *JSONLogger) TrackStartRequest(id string, agent model.Agent, packet model.Packet) {
	j.mu.RLock()
	data, exists := j.data[id]
	j.mu.RUnlock()

	if exists {
		data.mu.Lock()
		data.timestampMap[agent.OriginalName] = time.Now().UnixNano()
		data.requestMap[agent.OriginalName] = packet
		data.mu.Unlock()
	}
}

func (j *JSONLogger) TrackEndRequest(id string, agent model.Agent, response string, err error) {
	j.mu.RLock()
	data, exists := j.data[id]
	j.mu.RUnlock()

	if exists {
		data.mu.Lock()
		timestamp := time.Now().UnixNano()
		entry := map[string]any{
			"agent":              agent.String(),
			"request_timestamp":  data.timestampMap[agent.OriginalName] / 1e6,
			"response_timestamp": timestamp / 1e6,
		}
		if request, exists := data.requestMap[agent.OriginalName]; exists {
			jsonData, err := json.Marshal(request)
			if err == nil {
				entry["request"] = string(jsonData)
			}
		}
		if response != "" {
			entry["response"] = response
		}
		if err != nil {
			entry["error"] = err.Error()
		}
		data.entries = append(data.entries, entry)
		delete(data.timestampMap, agent.OriginalName)
		delete(data.requestMap, agent.OriginalName)
		data.mu.Unlock()

		j.saveGameData(id)
	}
}

func (j *JSONLogger) saveGameData(id string) {
	j.mu.RLock()
	data, exists := j.data[id]
	j.mu.RUnlock()

	if exists {
		data.mu.Lock()
		game := map[string]any{
			"game_id":  id,
			"win_side": data.winSide,
			"agents":   data.agents,
			"entries":  data.entries,
		}
		jsonData, err := json.Marshal(game)
		data.mu.Unlock()
		if err != nil {
			return
		}

		if _, err := os.Stat(j.outputDir); os.IsNotExist(err) {
			os.Mkdir(j.outputDir, 0755)
		}
		filePath := filepath.Join(j.outputDir, fmt.Sprintf("%s.json", data.filename))
		file, err := os.Create(filePath)
		if err != nil {
			return
		}
		defer file.Close()
		file.Write(jsonData)
	}
}
