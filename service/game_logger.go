package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/iggy157/aiwolf-nlp-server-unlimited/model"
)

type GameLogger struct {
	data             map[string]*GameLog
	outputDir        string
	templateFilename string
	mu               sync.RWMutex
}

type GameLog struct {
	id       string
	filename string
	agents   []any
	logs     []string
	mu       sync.Mutex
}

func NewGameLogger(config model.Config) *GameLogger {
	return &GameLogger{
		data:             make(map[string]*GameLog),
		outputDir:        config.GameLogger.OutputDir,
		templateFilename: config.GameLogger.Filename,
	}
}

func (g *GameLogger) TrackStartGame(id string, agents []*model.Agent) {
	data := &GameLog{
		id:   id,
		logs: make([]string, 0),
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
	filename := strings.ReplaceAll(g.templateFilename, "{game_id}", data.id)
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

	g.mu.Lock()
	g.data[id] = data
	g.mu.Unlock()
}

func (g *GameLogger) TrackEndGame(id string) {
	g.mu.RLock()
	_, exists := g.data[id]
	g.mu.RUnlock()

	if exists {
		g.saveLog(id)

		g.mu.Lock()
		delete(g.data, id)
		g.mu.Unlock()
	}
}

func (g *GameLogger) AppendLog(id string, log string) {
	g.mu.RLock()
	data, exists := g.data[id]
	g.mu.RUnlock()

	if exists {
		data.mu.Lock()
		data.logs = append(data.logs, log)
		data.mu.Unlock()

		g.saveLog(id)
	}
}

func (g *GameLogger) saveLog(id string) {
	g.mu.RLock()
	data, exists := g.data[id]
	g.mu.RUnlock()

	if exists {
		data.mu.Lock()
		str := strings.Join(data.logs, "\n")
		data.mu.Unlock()

		if _, err := os.Stat(g.outputDir); os.IsNotExist(err) {
			os.MkdirAll(g.outputDir, 0755)
		}

		filePath := filepath.Join(g.outputDir, fmt.Sprintf("%s.log", data.filename))
		file, err := os.Create(filePath)
		if err != nil {
			return
		}
		defer file.Close()
		file.WriteString(str)
	}
}
