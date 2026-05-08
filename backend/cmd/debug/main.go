package main

import (
	"fmt"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/attribute"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/gameconfig"
	"github.com/Milkyway-Liluo-Idle-Develop-Team/Milkyway-Liluo-Idle-2/backend/internal/item"
)

func main() {
	if err := gameconfig.Load(); err != nil {
		panic(err)
	}
	if err := attribute.Load(); err != nil {
		panic(err)
	}
	def, ok := gameconfig.GetItemDef("wooden_sword")
	if !ok {
		panic("not found")
	}
	mods, err := def.Modifiers(item.Item{ID: def.ID()}, attribute.Get())
	if err != nil {
		panic(err)
	}

	inst := attribute.NewInstance()
	ppID, _ := attribute.Get().AttrID("physical_power")

	fmt.Println("=== Before AddModifiers ===")
	for _, m := range inst.ModifiersFor(ppID) {
		sid, _ := attribute.Get().AttrString(m.AttrID)
		fmt.Printf("  %s = %v (source=%s)\n", sid, m.Value, m.Source)
	}

	inst.AddModifiers("equipment:wooden_sword", mods)

	fmt.Println("=== After AddModifiers ===")
	for _, m := range inst.ModifiersFor(ppID) {
		sid, _ := attribute.Get().AttrString(m.AttrID)
		fmt.Printf("  %s = %v (source=%s)\n", sid, m.Value, m.Source)
	}

	fmt.Printf("physical_power final: %v\n", inst.GetFinal(ppID))
}
