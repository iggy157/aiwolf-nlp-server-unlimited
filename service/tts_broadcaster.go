package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/iggy157/aiwolf-nlp-server-unlimited/model"
	"github.com/iggy157/aiwolf-nlp-server-unlimited/util"
	"github.com/gin-gonic/gin"
	"github.com/grafov/m3u8"
)

type TTSBroadcaster struct {
	config    model.Config
	baseURL   *url.URL
	client    *http.Client
	streamsMu sync.RWMutex
	streams   map[string]*Stream
}

type Stream struct {
	isStreaming     bool
	lastSegmentTime time.Time
	segmentCounter  int
	playlist        *m3u8.MediaPlaylist
	streamingMu     sync.Mutex
	segmentsMu      sync.RWMutex
	playlistMu      sync.Mutex
}

const (
	SILENCE_TEMPLATE_FILE = "silence.ts"
	PLAYLIST_FILE         = "playlist.m3u8"
)

func NewTTSBroadcaster(config model.Config) *TTSBroadcaster {
	baseURL, err := url.Parse(config.TTSBroadcaster.Host)
	if err != nil {
		slog.Error("音声合成サーバのURLの解析に失敗しました", "error", err)
		baseURL = &url.URL{
			Scheme: "http",
			Host:   "localhost:50021",
		}
	}
	return &TTSBroadcaster{
		config:  config,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: config.TTSBroadcaster.Timeout,
		},
		streams: make(map[string]*Stream),
	}
}

func (t *TTSBroadcaster) Start() {
	if _, err := os.Stat(t.config.TTSBroadcaster.SegmentDir); os.IsNotExist(err) {
		os.MkdirAll(t.config.TTSBroadcaster.SegmentDir, 0755)
	}
	t.cleanupSegments()

	outputPath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, SILENCE_TEMPLATE_FILE)

	if err := util.BuildSilenceTemplate(t.config.TTSBroadcaster.FfmpegPath, t.config.TTSBroadcaster.SilenceArgs, t.config.TTSBroadcaster.TargetDuration.Seconds(), outputPath); err != nil {
		return
	}

	go t.streamManager()
}

func (t *TTSBroadcaster) getStream(id string) *Stream {
	t.streamsMu.RLock()
	stream, exists := t.streams[id]
	t.streamsMu.RUnlock()

	if exists {
		return stream
	}

	t.streamsMu.Lock()
	defer t.streamsMu.Unlock()

	if stream, exists = t.streams[id]; exists {
		return stream
	}

	stream = &Stream{
		isStreaming:     false,
		lastSegmentTime: time.Now(),
		segmentCounter:  0,
	}

	streamDir := filepath.Join(t.config.TTSBroadcaster.SegmentDir, id)
	if _, err := os.Stat(streamDir); os.IsNotExist(err) {
		os.MkdirAll(streamDir, 0755)
	}

	playlist, err := m3u8.NewMediaPlaylist(math.MaxInt16, math.MaxInt16)
	if err != nil {
		slog.Error("プレイリストの作成に失敗しました", "error", err, "id", id)
		return nil
	}

	playlist.TargetDuration = float64(t.config.TTSBroadcaster.TargetDuration.Seconds())
	playlist.SetVersion(3)
	playlist.Closed = false
	stream.playlist = playlist

	for range t.config.TTSBroadcaster.MinBufferSegments {
		t.addSilenceSegment(id, stream)
	}

	t.streams[id] = stream
	return stream
}

func (t *TTSBroadcaster) cleanupSegments() {
	if err := os.RemoveAll(t.config.TTSBroadcaster.SegmentDir); err != nil {
		slog.Error("セグメントディレクトリの削除に失敗しました", "error", err)
		return
	}
	slog.Info("セグメントディレクトリのクリーンアップが完了しました")
	if err := os.MkdirAll(t.config.TTSBroadcaster.SegmentDir, 0755); err != nil {
		slog.Error("セグメントディレクトリの作成に失敗しました", "error", err)
		return
	}
}

func (t *TTSBroadcaster) getSegmentDir(id string) string {
	return filepath.Join(t.config.TTSBroadcaster.SegmentDir, id)
}

func (t *TTSBroadcaster) addSilenceSegment(id string, stream *Stream) {
	stream.segmentsMu.Lock()
	silenceSegmentName := fmt.Sprintf("segment_%d.ts", stream.segmentCounter)
	stream.segmentCounter++
	stream.segmentsMu.Unlock()

	streamDir := t.getSegmentDir(id)
	silenceTemplatePath := filepath.Join(t.config.TTSBroadcaster.SegmentDir, SILENCE_TEMPLATE_FILE)
	silenceSegmentPath := filepath.Join(streamDir, silenceSegmentName)

	if err := util.CopyFile(silenceTemplatePath, silenceSegmentPath); err != nil {
		slog.Error("無音セグメントのコピーに失敗しました", "error", err, "id", id)
		return
	}

	stream.playlistMu.Lock()
	defer stream.playlistMu.Unlock()

	if err := stream.playlist.AppendSegment(&m3u8.MediaSegment{
		URI:      silenceSegmentName,
		Duration: t.config.TTSBroadcaster.TargetDuration.Seconds(),
	}); err != nil {
		slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err, "id", id)
	}

	t.writePlaylist(id, stream)
}

func (t *TTSBroadcaster) writePlaylist(id string, stream *Stream) {
	streamDir := t.getSegmentDir(id)
	playlistPath := filepath.Join(streamDir, PLAYLIST_FILE)

	if err := os.MkdirAll(streamDir, 0755); err != nil {
		slog.Error("プレイリストディレクトリの作成に失敗しました", "error", err, "id", id)
		return
	}

	if err := os.WriteFile(playlistPath, stream.playlist.Encode().Bytes(), 0644); err != nil {
		slog.Error("プレイリストの書き込みに失敗しました", "error", err, "id", id)
	}
}

func (t *TTSBroadcaster) streamManager() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		t.streamsMu.RLock()
		streamIDs := make([]string, 0, len(t.streams))
		for id := range t.streams {
			streamIDs = append(streamIDs, id)
		}
		t.streamsMu.RUnlock()

		for _, id := range streamIDs {
			t.streamsMu.RLock()
			stream, exists := t.streams[id]
			t.streamsMu.RUnlock()

			if !exists {
				continue
			}

			stream.streamingMu.Lock()
			isStreaming := stream.isStreaming
			lastTime := stream.lastSegmentTime
			stream.streamingMu.Unlock()

			if !isStreaming {
				elapsed := time.Since(lastTime).Seconds()

				if elapsed >= t.config.TTSBroadcaster.TargetDuration.Seconds()*0.8 {
					t.addSilenceSegment(id, stream)
					stream.streamingMu.Lock()
					stream.lastSegmentTime = time.Now()
					stream.streamingMu.Unlock()
				}
			}
		}
	}
}

func (t *TTSBroadcaster) HandlePlaylist(c *gin.Context) {
	id := c.Param("id")
	if id == "" || strings.ContainsAny(id, "/\\") {
		c.Status(http.StatusBadRequest)
		return
	}

	if !isValidID(id) {
		c.Status(http.StatusBadRequest)
		return
	}

	streamDir := t.getSegmentDir(id)
	playlistPath := filepath.Join(streamDir, PLAYLIST_FILE)

	if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Access-Control-Allow-Origin", "*")
	c.File(playlistPath)
}

func (t *TTSBroadcaster) HandleSegment(c *gin.Context) {
	id := c.Param("id")
	if id == "" || strings.ContainsAny(id, "/\\") {
		c.Status(http.StatusBadRequest)
		return
	}

	if !isValidID(id) {
		c.Status(http.StatusBadRequest)
		return
	}

	segment := c.Param("segment")
	segmentName := strings.TrimPrefix(segment, "/")

	if !strings.HasSuffix(segmentName, ".ts") || strings.ContainsAny(segmentName, "/\\") {
		c.Status(http.StatusNotFound)
		return
	}

	if !isValidSegmentName(segmentName) {
		c.Status(http.StatusBadRequest)
		return
	}

	streamDir := t.getSegmentDir(id)
	segmentPath := filepath.Join(streamDir, segmentName)

	if _, err := os.Stat(segmentPath); os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "video/MP2T")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Access-Control-Allow-Origin", "*")
	c.File(segmentPath)
}

func isValidID(id string) bool {
	match, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", id)
	return match
}

func isValidSegmentName(name string) bool {
	match, _ := regexp.MatchString("^[a-zA-Z0-9_-]+\\.ts$", name)
	return match
}

func (t *TTSBroadcaster) BroadcastText(id string, text string, speaker int) {
	stream := t.getStream(id)
	if stream == nil {
		return
	}

	stream.streamingMu.Lock()
	stream.isStreaming = true
	stream.streamingMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), t.config.TTSBroadcaster.Timeout)
		defer cancel()

		audioQueryCh := t.fetchAudioQueryAsync(ctx, text, speaker)
		if err := t.processTextToSpeech(ctx, audioQueryCh, id, stream, speaker); err != nil {
			slog.Error("音声合成に失敗しました", "error", err, "id", id)
		}

		stream.streamingMu.Lock()
		stream.isStreaming = false
		stream.lastSegmentTime = time.Now()
		stream.streamingMu.Unlock()
	}()
}

func (t *TTSBroadcaster) fetchAudioQueryAsync(ctx context.Context, text string, speaker int) <-chan []byte {
	resultCh := make(chan []byte, 1)

	go func() {
		defer close(resultCh)

		baseURL := *t.baseURL
		baseURL.Path = "/audio_query"
		params := url.Values{}
		params.Add("speaker", fmt.Sprintf("%d", speaker))
		params.Add("text", text)
		baseURL.RawQuery = params.Encode()
		queryURL := baseURL.String()

		req, err := http.NewRequestWithContext(ctx, "POST", queryURL, nil)
		if err != nil {
			slog.Error("オーディオクエリリクエスト作成に失敗しました", "error", err)
			return
		}

		resp, err := t.client.Do(req)
		if err != nil {
			slog.Error("オーディオクエリ送信に失敗しました", "error", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Error("オーディオクエリに失敗しました", "status", resp.StatusCode)
			return
		}

		queryParams, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("オーディオクエリ読み取りに失敗しました", "error", err)
			return
		}

		resultCh <- queryParams
	}()
	return resultCh
}

func (t *TTSBroadcaster) processTextToSpeech(ctx context.Context, audioQueryCh <-chan []byte, id string, stream *Stream, speaker int) error {
	var queryParams []byte

	select {
	case <-ctx.Done():
		return ctx.Err()
	case params, ok := <-audioQueryCh:
		if !ok || params == nil {
			return fmt.Errorf("オーディオクエリの取得に失敗しました")
		}
		queryParams = params
	}

	baseURL := *t.baseURL
	baseURL.Path = "/synthesis"
	params := url.Values{}
	params.Add("speaker", fmt.Sprintf("%d", speaker))
	baseURL.RawQuery = params.Encode()
	queryURL := baseURL.String()

	req, err := http.NewRequestWithContext(ctx, "POST", queryURL, bytes.NewBuffer(queryParams))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("合成クエリに失敗しました: %d", resp.StatusCode)
	}

	wavData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	stream.segmentsMu.Lock()
	baseName := fmt.Sprintf("segment_%d", stream.segmentCounter)
	stream.segmentCounter++
	stream.segmentsMu.Unlock()

	segmentParams := util.ConvertWavToSegmentParams{
		FfmpegPath:      t.config.TTSBroadcaster.FfmpegPath,
		FfprobePath:     t.config.TTSBroadcaster.FfprobePath,
		DurationArgs:    t.config.TTSBroadcaster.DurationArgs,
		ConvertArgs:     t.config.TTSBroadcaster.ConvertArgs,
		PreConvertArgs:  t.config.TTSBroadcaster.PreConvertArgs,
		SplitArgs:       t.config.TTSBroadcaster.SplitArgs,
		TempDir:         t.config.TTSBroadcaster.TempDir,
		SegmentDuration: t.config.TTSBroadcaster.TargetDuration.Seconds(),
		Data:            wavData,
		BaseDir:         t.getSegmentDir(id),
		BaseName:        baseName,
	}
	segmentNames, err := util.ConvertWavToSegment(segmentParams)
	if err != nil {
		return err
	}

	t.addSegmentsToPlaylist(id, stream, segmentNames)
	return nil
}

func (t *TTSBroadcaster) addSegmentsToPlaylist(id string, stream *Stream, segmentNames []string) {
	if len(segmentNames) == 0 {
		return
	}

	streamDir := t.getSegmentDir(id)
	stream.playlistMu.Lock()
	defer stream.playlistMu.Unlock()

	for _, segmentName := range segmentNames {
		duration, err := util.GetDuration(t.config.TTSBroadcaster.FfprobePath, t.config.TTSBroadcaster.DurationArgs, filepath.Join(streamDir, segmentName))
		if err != nil {
			slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err, "id", id)
			continue
		}

		if err := stream.playlist.AppendSegment(&m3u8.MediaSegment{
			URI:      segmentName,
			Duration: duration,
		}); err != nil {
			slog.Error("プレイリストへのセグメント追加に失敗しました", "error", err, "id", id)
		}
	}

	t.writePlaylist(id, stream)
}

func (t *TTSBroadcaster) CleanupStream(id string) {
	t.streamsMu.Lock()
	delete(t.streams, id)
	t.streamsMu.Unlock()

	streamDir := t.getSegmentDir(id)
	os.RemoveAll(streamDir)
}
