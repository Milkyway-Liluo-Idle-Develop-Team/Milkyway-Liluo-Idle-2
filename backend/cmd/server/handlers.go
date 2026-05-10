package main

import (
	"context"
	"fmt"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/apperror"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/battle"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
	pb "github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/pb"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/session"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/wsx"
)

func registerGameHandlers(hub *wsx.Hub, mgr *session.Manager) {
	session.HandleCommandTyped(mgr, hub, "inventory.equip", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.EquipReq) error {
		it := item.Item{ID: item.ID(req.ItemId), State: item.State(req.ItemState)}
		if err := sess.Equip(ctx, it, req.Slot); err != nil {
			return err
		}
		c.Send(wsx.Outbound{Type: "inventory.equip.ok", Payload: buildEquipResponse(sess)})
		return nil
	})

	session.HandleCommandTyped(mgr, hub, "inventory.unequip", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.UnequipReq) error {
		if err := sess.Unequip(ctx, req.Slot); err != nil {
			return err
		}
		c.Send(wsx.Outbound{Type: "inventory.unequip.ok", Payload: buildEquipResponse(sess)})
		return nil
	})

	session.HandleCommandTyped(mgr, hub, "queue.append", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.QueueAppendReq) error {
		ev := sess.Events()
		if ev == nil {
			return apperror.Unavailable("event system not loaded")
		}
		queueID := int(req.QueueId)
		if queueID < 0 {
			queueID = 0
		}
		eventID := gameconfig.EventID(req.EventId)
		evCfg, ok := gameconfig.GetEventByID(eventID)
		if ok && (evCfg.Type == gameconfig.EventTypeInstant || evCfg.Type == gameconfig.EventTypeUpgrade) {
			// Instant/upgrade events execute immediately without queuing.
			if !ev.CheckReqs(sess, evCfg) {
				return apperror.BadRequest("requirements not met")
			}
			ev.ExecuteInstant(sess, eventID, evCfg)
			return nil
		}
		ev.Enqueue(queueID, eventID, int(req.TargetCycles))
		return nil
	})

	session.HandleCommandTyped(mgr, hub, "queue.remove", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.QueueRemoveReq) error {
		ev := sess.Events()
		if ev == nil {
			return apperror.Unavailable("event system not loaded")
		}
		queueID := int(req.QueueId)
		if queueID < 0 {
			queueID = 0
		}
		ev.RemoveEntry(queueID, int(req.Position))
		return nil
	})

	session.HandleCommandTyped(mgr, hub, "queue.move", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.QueueMoveReq) error {
		ev := sess.Events()
		if ev == nil {
			return apperror.Unavailable("event system not loaded")
		}
		queueID := int(req.QueueId)
		if queueID < 0 {
			queueID = 0
		}
		ev.MoveEntry(queueID, int(req.FromPosition), int(req.ToPosition))
		return nil
	})

	session.HandleCommandTyped(mgr, hub, "queue.set", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.QueueSetReq) error {
		ev := sess.Events()
		if ev == nil {
			return apperror.Unavailable("event system not loaded")
		}
		queueID := int(req.QueueId)
		if queueID < 0 {
			queueID = 0
		}
		ev.ClearQueue(queueID)
		for _, entry := range req.Entries {
			ev.Enqueue(queueID, gameconfig.EventID(entry.EventId), int(entry.TargetCycles))
		}
		return nil
	})

	// --- Battle handlers ---

	session.HandleCommandTyped(mgr, hub, "battle.start", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.BattleStartReq) error {
		if sess.BattleSession() != nil {
			return apperror.BadRequest("battle already active")
		}
		def, ok := gameconfig.GetBattle(req.BattleId)
		if !ok {
			return apperror.BadRequest("battle not found")
		}

		player := battle.NewPlayerBattleEntity(sess.UserID, fmt.Sprintf("Player%d", sess.UserID), sess.Attr())
		player.SetHP(player.MaxHP())
		player.SetMP(player.MaxMP())
		player.SetSP(player.MaxSP())
		basicSkillID, _ := gameconfig.StringToBattleSkillID("basic_attack")
		player.SetSkills(map[gameconfig.BattleSkillID]*battle.BattleSkill{
			basicSkillID: {
				ID:   basicSkillID,
				Name: "基础攻击",
				Damage: &battle.DamageProfile{
					Type:       "physical",
					Flat:       0,
					Multiplier: 1.0,
				},
				CastTime: 2.0,
				IsBasic:  true,
			},
		})
		player.SetBasicSkillID(basicSkillID)
		player.SetSkillPlan([]battle.SkillPlanEntry{{SkillID: basicSkillID, Priority: 0}})

		cfg := battle.BattleConfig{
			NumericID:               def.NumericID,
			ID:                      def.ID,
			Name:                    def.Name,
			Map:                     def.Map,
			Interval:                def.Interval,
			CombinationLoop:         def.CombinationLoop,
			WeakEnemyCombinations:   convertCombinations(def.WeakEnemyCombinations),
			StrongEnemyCombinations: convertCombinations(def.StrongEnemyCombinations),
			BossEnemyCombinations:   convertCombinations(def.BossEnemyCombinations),
		}

		bs := battle.NewBattleSession(cfg, []*battle.PlayerBattleEntity{player})
		sess.SetBattleSession(bs)
		mgr.AddBattle(bs)

		snap := bs.BuildSnapshot()
		c.Send(wsx.Outbound{Type: "battle.start.ok", Payload: &pb.BattleStartResp{
			Snapshot: session.BattleSnapshotToProto(&snap),
		}})
		return nil
	})

	session.HandleCommandTyped(mgr, hub, "battle.stop", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.BattleStopReq) error {
		bs := sess.BattleSession()
		if bs == nil {
			return apperror.BadRequest("no active battle")
		}
		bs.Running = false
		snap := bs.BuildSnapshot()
		sess.SetBattleSession(nil)
		mgr.RemoveBattle(bs.Config.NumericID)
		c.Send(wsx.Outbound{Type: "battle.stop.ok", Payload: &pb.BattleStopResp{
			Snapshot: session.BattleSnapshotToProto(&snap),
		}})
		return nil
	})

	session.HandleCommandTyped(mgr, hub, "battle.state", func(ctx context.Context, c *wsx.Conn, sess *session.PlayerSession, req *pb.BattleStateReq) error {
		bs := sess.BattleSession()
		if bs == nil {
			c.Send(wsx.Outbound{Type: "battle.state.ok", Payload: &pb.BattleStateResp{}})
			return nil
		}
		snap := bs.BuildSnapshot()
		c.Send(wsx.Outbound{Type: "battle.state.ok", Payload: &pb.BattleStateResp{
			Snapshot: session.BattleSnapshotToProto(&snap),
		}})
		return nil
	})
}

func buildEquipResponse(sess *session.PlayerSession) *pb.EquipUnequipResponse {
	resp := &pb.EquipUnequipResponse{Equipped: make(map[string]*pb.ItemIdentity)}
	for slot, entry := range sess.Equipment().All() {
		resp.Equipped[slot] = &pb.ItemIdentity{ItemId: int32(entry.Item.ID), ItemState: int32(entry.Item.State)}
	}
	return resp
}

func convertCombinations(in []gameconfig.EnemyWaveCombination) []battle.EnemyWaveCombination {
	out := make([]battle.EnemyWaveCombination, len(in))
	for i, c := range in {
		out[i] = battle.EnemyWaveCombination{
			Enemies: append([]string(nil), c.Enemies...),
			Weight:  c.Weight,
		}
	}
	return out
}


