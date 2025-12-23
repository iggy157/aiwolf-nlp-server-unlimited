package logic

import (
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"unicode/utf8"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/aiwolfdial/aiwolf-nlp-server/util"
)

func (g *Game) doWhisper() {
	slog.Info("囁きフェーズを開始します", "id", g.id, "day", g.currentDay)
	g.conductCommunication(model.R_WHISPER)
}

func (g *Game) doTalk() {
	slog.Info("トークフェーズを開始します", "id", g.id, "day", g.currentDay)
	g.conductCommunication(model.R_TALK)
}

func (g *Game) conductCommunication(request model.Request) {
	var agents []*model.Agent
	var talkSetting *model.TalkSetting
	var talkList *[]model.Talk

	switch request {
	case model.R_TALK:
		agents = g.getAliveAgents()
		talkSetting = &g.setting.Talk.TalkSetting
		talkList = &g.getCurrentGameStatus().Talks
	case model.R_WHISPER:
		agents = g.getAliveWerewolves()
		talkSetting = &g.setting.Whisper.TalkSetting
		talkList = &g.getCurrentGameStatus().Whispers
	default:
		return
	}
	if len(agents) < 2 {
		slog.Warn("エージェント数が2未満のため、通信を行いません", "id", g.id, "agentNum", len(agents))
		return
	}

	remainCountMap := make(map[model.Agent]int)
	remainLengthMap := make(map[model.Agent]int)
	remainSkipMap := make(map[model.Agent]int)
	for _, agent := range agents {
		remainCountMap[*agent] = talkSetting.MaxCount.PerAgent
		if talkSetting.MaxLength.PerAgent != nil {
			remainLengthMap[*agent] = *talkSetting.MaxLength.PerAgent
		}
		remainSkipMap[*agent] = talkSetting.MaxSkip
	}
	g.getCurrentGameStatus().RemainCountMap = &remainCountMap
	g.getCurrentGameStatus().RemainLengthMap = &remainLengthMap
	g.getCurrentGameStatus().RemainSkipMap = &remainSkipMap

	rand.Shuffle(len(agents), func(i, j int) {
		agents[i], agents[j] = agents[j], agents[i]
	})

	idx := 0
	for i := range talkSetting.MaxCount.PerDay {
		cnt := false
		for _, agent := range agents {
			if remainCountMap[*agent] <= 0 {
				continue
			}
			if value, exists := remainLengthMap[*agent]; exists {
				if value <= 0 {
					continue
				}
			}
			remainCountMap[*agent]--
			text := g.getTalkWhisperText(agent, request)
			switch text {
			case model.T_SKIP:
				if remainSkipMap[*agent] <= 0 {
					text = model.T_OVER
					slog.Warn("スキップ回数が上限に達したため、発言をオーバーに置換しました", "id", g.id, "agent", agent.String())
				} else {
					remainSkipMap[*agent]--
					slog.Info("発言をスキップしました", "id", g.id, "agent", agent.String())
				}
			case model.T_FORCE_SKIP:
				text = model.T_SKIP
				slog.Warn("強制スキップが指定されたため、発言をスキップに置換しました", "id", g.id, "agent", agent.String())
			}
			if text != model.T_OVER && text != model.T_SKIP {
				remainSkipMap[*agent] = talkSetting.MaxSkip
				slog.Info("発言がオーバーもしくはスキップではないため、スキップ回数をリセットしました", "id", g.id, "agent", agent.String())
			}

			if text != model.T_OVER && text != model.T_SKIP && text != model.T_FORCE_SKIP {
				if talkSetting.MaxLength.PerAgent != nil || talkSetting.MaxLength.BaseLength != nil || talkSetting.MaxLength.PerTalk != nil {
					mention := ""
					commonText := ""
					mentionText := ""

					if talkSetting.MaxLength.PerAgent != nil || talkSetting.MaxLength.BaseLength != nil {
						baseLength := 0
						if talkSetting.MaxLength.BaseLength != nil {
							baseLength = *talkSetting.MaxLength.BaseLength
						}

						mentionIdx := -1
						if talkSetting.MaxLength.MentionLength != nil {
							for _, a := range g.agents {
								if a != agent {
									if strings.Contains(text, "@"+a.String()) {
										if mentionIdx == -1 {
											mention = "@" + a.String()
											mentionIdx = strings.Index(text, mention)
										}
										if strings.Index(text, mention) < mentionIdx {
											mention = "@" + a.String()
											mentionIdx = strings.Index(text, mention)
										}
									}
								}
							}
						}

						if mentionIdx != -1 {
							remainLength := baseLength
							if value, exists := remainLengthMap[*agent]; exists {
								remainLength += value
							}
							mentionBefore := text[:mentionIdx]
							mentionAfter := text[mentionIdx+len(mention):]

							mention = " " + mention + " "

							commonText = util.TrimLength(mentionBefore, remainLength, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces)
							cost := util.CountLength(mentionBefore, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces) - baseLength
							if cost > 0 {
								if _, exists := remainLengthMap[*agent]; exists {
									remainLengthMap[*agent] -= cost
								}
							}

							remainLength = *talkSetting.MaxLength.MentionLength
							if value, exists := remainLengthMap[*agent]; exists {
								remainLength += value
							}
							mentionText = util.TrimLength(mentionAfter, remainLength, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces)
							mentionCost := util.CountLength(mentionText, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces) - *talkSetting.MaxLength.MentionLength
							if mentionCost > 0 {
								if _, exists := remainLengthMap[*agent]; exists {
									remainLengthMap[*agent] -= mentionCost
								}
							}
						} else {
							remainLength := baseLength
							if value, exists := remainLengthMap[*agent]; exists {
								remainLength += value
							}
							commonText = util.TrimLength(text, remainLength, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces)
							cost := util.CountLength(text, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces) - baseLength
							if cost > 0 {
								if _, exists := remainLengthMap[*agent]; exists {
									remainLengthMap[*agent] -= cost
								}
							}
						}
					} else {
						// PerTalkのみが有効な場合、textをそのままcommonTextに設定
						commonText = text
					}
					if talkSetting.MaxLength.PerTalk != nil {
						commonLength := util.CountLength(commonText, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces)
						mentionLength := util.CountLength(mentionText, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces)
						totalLength := commonLength + mentionLength

						if totalLength > *talkSetting.MaxLength.PerTalk {
							if commonLength > *talkSetting.MaxLength.PerTalk {
								commonText = util.TrimLength(commonText, *talkSetting.MaxLength.PerTalk, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces)
								mention = ""
								mentionText = ""
							} else {
								mentionText = util.TrimLength(mentionText, *talkSetting.MaxLength.PerTalk-commonLength, *talkSetting.MaxLength.CountInWord, *talkSetting.MaxLength.CountSpaces)
							}
							slog.Warn("発言が最大文字数を超えたため、切り捨てました", "id", g.id, "agent", agent.String())
						}
					}
					text = commonText + mention + mentionText
					if utf8.RuneCountInString(text) == 0 {
						text = model.T_OVER
						slog.Warn("文字数が0のため、発言をオーバーに置換しました", "id", g.id, "agent", agent.String())
					}
				}
			}

			talk := model.Talk{
				Idx:   idx,
				Day:   g.getCurrentGameStatus().Day,
				Turn:  i,
				Agent: *agent,
				Text:  text,
			}
			idx++
			*talkList = append(*talkList, talk)
			if text != model.T_OVER {
				cnt = true
			} else {
				remainCountMap[*agent] = 0
				slog.Info("発言がオーバーであるため、残り発言回数を0にしました", "id", g.id, "agent", agent.String())
			}
			if g.gameLogger != nil {
				if request == model.R_TALK {
					g.gameLogger.AppendLog(g.id, fmt.Sprintf("%d,talk,%d,%d,%d,%s", g.currentDay, talk.Idx, talk.Turn, talk.Agent.Idx, talk.Text))
				} else {
					g.gameLogger.AppendLog(g.id, fmt.Sprintf("%d,whisper,%d,%d,%d,%s", g.currentDay, talk.Idx, talk.Turn, talk.Agent.Idx, talk.Text))
				}
			}
			if g.realtimeBroadcaster != nil {
				if request == model.R_TALK {
					packet := g.getRealtimeBroadcastPacket()
					packet.Event = "トーク"
					packet.Message = &talk.Text
					packet.BubbleIdx = &agent.Idx
					g.realtimeBroadcaster.Broadcast(packet)
				} else {
					packet := g.getRealtimeBroadcastPacket()
					packet.Event = "囁き"
					packet.Message = &talk.Text
					packet.BubbleIdx = &agent.Idx
					g.realtimeBroadcaster.Broadcast(packet)
				}
			}
			if g.ttsBroadcaster != nil {
				g.ttsBroadcaster.BroadcastText(g.id, talk.Text, agent.Profile.VoiceID)
			}
			slog.Info("発言を受信しました", "id", g.id, "agent", agent.String(), "text", text, "count", remainCountMap[*agent], "length", remainLengthMap[*agent], "skip", remainSkipMap[*agent])
		}
		if !cnt {
			break
		}
	}

	g.getCurrentGameStatus().RemainCountMap = nil
	g.getCurrentGameStatus().RemainLengthMap = nil
	g.getCurrentGameStatus().RemainSkipMap = nil
}

func (g *Game) getTalkWhisperText(agent *model.Agent, request model.Request) string {
	text, err := g.requestToAgent(agent, request)
	if text == model.T_FORCE_SKIP {
		text = model.T_SKIP
		slog.Warn("クライアントから強制スキップが指定されたため、発言をスキップに置換しました", "id", g.id, "agent", agent.String())
	}
	if err != nil {
		text = model.T_FORCE_SKIP
		slog.Warn("リクエストの送受信に失敗したため、発言をスキップに置換しました", "id", g.id, "agent", agent.String())
	}
	return text
}
