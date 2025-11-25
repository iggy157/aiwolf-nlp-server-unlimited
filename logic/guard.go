package logic

import (
	"fmt"
	"log/slog"

	"github.com/iggy157/aiwolf-nlp-server-unlimited/model"
)

func (g *Game) doGuard() {
	slog.Info("護衛フェーズを開始します", "id", g.ID, "day", g.currentDay)
	for _, agent := range g.getAliveAgents() {
		if agent.Role == model.R_BODYGUARD {
			g.conductGuard(agent)
			break
		}
	}
}

func (g *Game) conductGuard(agent *model.Agent) {
	slog.Info("護衛アクションを実行します", "id", g.ID, "agent", agent.String())
	target, err := g.findTargetByRequest(agent, model.R_GUARD)
	if err != nil {
		slog.Warn("護衛対象が見つからなかったため、護衛対象を設定しません", "id", g.ID)
		return
	}
	if !g.isAlive(target) {
		slog.Warn("護衛対象が死亡しているため、護衛対象を設定しません", "id", g.ID, "target", target.String())
		return
	}
	if agent == target {
		slog.Warn("護衛対象が自分自身であるため、護衛対象を設定しません", "id", g.ID, "target", target.String())
		return
	}
	g.getCurrentGameStatus().Guard = &model.Guard{
		Day:    g.getCurrentGameStatus().Day,
		Agent:  *agent,
		Target: *target,
	}
	if g.GameLogger != nil {
		g.GameLogger.AppendLog(g.ID, fmt.Sprintf("%d,guard,%d,%d,%s", g.currentDay, agent.Idx, target.Idx, target.Role.Name))
	}
	if g.RealtimeBroadcaster != nil {
		packet := g.getRealtimeBroadcastPacket()
		packet.Event = "護衛"
		packet.FromIdx = &agent.Idx
		packet.ToIdx = &target.Idx
		g.RealtimeBroadcaster.Broadcast(packet)
	}
	slog.Info("護衛対象を設定しました", "id", g.ID, "target", target.String())
}
