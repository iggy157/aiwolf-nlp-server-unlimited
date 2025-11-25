package test

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/iggy157/aiwolf-nlp-server-unlimited/model"
	"github.com/gorilla/websocket"
)

type DummyClient struct {
	conn        *websocket.Conn
	done        chan struct{}
	name        string
	role        model.Role
	info        map[string]any
	setting     map[string]any
	talkIndex   int
	prevRequest model.Request
}

func NewDummyClient(u url.URL, name string, t *testing.T) (*DummyClient, error) {
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial: %v", err)
	}
	client := &DummyClient{
		conn:        c,
		done:        make(chan struct{}),
		name:        name,
		role:        model.Role{},
		info:        make(map[string]any),
		setting:     make(map[string]any),
		talkIndex:   0,
		prevRequest: model.Request{},
	}
	go client.listen(t)
	return client, nil
}

func (dc *DummyClient) listen(t *testing.T) {
	defer close(dc.done)
	for {
		_, message, err := dc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				t.Logf("connection closed: %v", err)
				return
			}
			t.Logf("read: %v", err)
			return
		}
		t.Logf("recv: %s", message)

		var recv map[string]any
		if err := json.Unmarshal(message, &recv); err != nil {
			t.Logf("unmarshal: %v", err)
			continue
		}

		request := model.RequestFromString(recv["request"].(string))
		resp, err := dc.handleRequest(request, recv)
		if err != nil {
			t.Error(err)
		}
		dc.prevRequest = request

		if resp != "" {
			err = dc.conn.WriteMessage(websocket.TextMessage, []byte(resp))
			if err != nil {
				if websocket.IsUnexpectedCloseError(err) || websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Logf("connection closed: %v", err)
					return
				}
				t.Logf("write: %v", err)
				continue
			}
			t.Logf("send: %s", resp)
		}
	}
}

func (dc *DummyClient) setInfo(recv map[string]any) error {
	if info, exists := recv["info"].(map[string]any); exists {
		dc.info = info
		if dc.role.String() == "" {
			if roleMap, exists := info["role_map"].(map[string]any); exists {
				for _, v := range roleMap {
					dc.role = model.RoleFromString(v.(string))
					break
				}
			}
		}
	} else {
		return errors.New("info not found")
	}
	return nil
}

func (dc *DummyClient) setSetting(recv map[string]any) error {
	if setting, exists := recv["setting"].(map[string]any); exists {
		dc.setting = setting
	} else {
		return errors.New("setting not found")
	}
	return nil
}

func (dc *DummyClient) handleName(_ map[string]any) (string, error) {
	return dc.name, nil
}

func (dc *DummyClient) handleInitialize(recv map[string]any) (string, error) {
	err := dc.setInfo(recv)
	if err != nil {
		return "", err
	}
	err = dc.setSetting(recv)
	if err != nil {
		return "", err
	}
	return "", nil
}

func (dc *DummyClient) handleCommunication(recv map[string]any) (string, error) {
	request := recv["request"].(string)
	if _, exists := recv[strings.ToLower(request)+"_history"].([]any); exists {
	} else {
		return "", errors.New("history not found")
	}
	dc.talkIndex++
	if dc.talkIndex < 3 {
		return fmt.Sprintf("%x", md5.Sum([]byte(time.Now().String()))), nil
	}
	return model.T_OVER, nil
}

func (dc *DummyClient) handleTarget(_ map[string]any) (string, error) {
	if statusMap, exists := dc.info["status_map"].(map[string]any); exists {
		for k, v := range statusMap {
			if k == dc.info["agent"].(string) {
				continue
			}
			if v == model.S_ALIVE.String() {
				return k, nil
			}
		}
		return "", errors.New("target not found")
	}
	return "", errors.New("status_map not found")
}

func (dc *DummyClient) handleDailyFinish(recv map[string]any) (string, error) {
	if _, exists := recv["talk_history"].([]any); exists {
	} else {
		return "", errors.New("talk_history not found")
	}
	if dc.role == model.R_WEREWOLF {
		if _, exists := recv["whisper_history"].([]any); exists {
		} else {
			return "", errors.New("whisper_history not found")
		}
	} else {
		if _, exists := recv["whisper_history"]; exists {
			return "", errors.New("whisper_history found")
		}
	}
	return "", nil
}

func (dc *DummyClient) handleFinish(recv map[string]any) (string, error) {
	err := dc.setInfo(recv)
	if err != nil {
		return "", err
	}
	return "", nil
}

func (dc *DummyClient) handleRequest(request model.Request, recv map[string]any) (string, error) {
	switch request {
	case model.R_NAME:
		return dc.handleName(recv)
	case model.R_INITIALIZE:
		return dc.handleInitialize(recv)
	case model.R_DAILY_INITIALIZE:
		return dc.handleInitialize(recv)
	case model.R_TALK, model.R_WHISPER:
		return dc.handleCommunication(recv)
	case model.R_VOTE, model.R_DIVINE, model.R_GUARD, model.R_ATTACK:
		return dc.handleTarget(recv)
	case model.R_DAILY_FINISH:
		dc.talkIndex = 0
		return dc.handleDailyFinish(recv)
	case model.R_FINISH:
		return dc.handleFinish(recv)
	}
	return "", errors.New("request not found")
}

func (dc *DummyClient) Close() {
	dc.conn.Close()
	select {
	case <-dc.done:
	case <-time.After(time.Second):
	}
}
