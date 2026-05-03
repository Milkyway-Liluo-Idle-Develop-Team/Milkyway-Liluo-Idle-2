package main

import (
	"context"

	"github.com/edrowsluo/new-mli/backend/internal/item"
	pb "github.com/edrowsluo/new-mli/backend/pb"
	"github.com/edrowsluo/new-mli/backend/internal/session"
	"github.com/edrowsluo/new-mli/backend/internal/wsx"
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
}

func buildEquipResponse(sess *session.PlayerSession) *pb.EquipUnequipResponse {
	resp := &pb.EquipUnequipResponse{Equipped: make(map[string]*pb.ItemIdentity)}
	for slot, it := range sess.Equipment().All() {
		resp.Equipped[slot] = &pb.ItemIdentity{ItemId: int32(it.ID), ItemState: int32(it.State)}
	}
	return resp
}
