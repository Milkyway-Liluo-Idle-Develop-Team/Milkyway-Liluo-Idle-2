package main

import (
	"context"

	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/apperror"
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
		ev.Enqueue(queueID, gameconfig.EventID(req.EventId), int(req.TargetCycles))
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
}

func buildEquipResponse(sess *session.PlayerSession) *pb.EquipUnequipResponse {
	resp := &pb.EquipUnequipResponse{Equipped: make(map[string]*pb.ItemIdentity)}
	for slot, it := range sess.Equipment().All() {
		resp.Equipped[slot] = &pb.ItemIdentity{ItemId: int32(it.ID), ItemState: int32(it.State)}
	}
	return resp
}
