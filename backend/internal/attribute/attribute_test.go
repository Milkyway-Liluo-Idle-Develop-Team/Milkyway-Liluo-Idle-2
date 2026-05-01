package attribute_test

import (
	"testing"

	"github.com/edrowsluo/new-mli/backend/internal/attribute"
	"github.com/edrowsluo/new-mli/backend/internal/record"
)

func init() {
	if !attribute.IsLoaded() {
		if err := attribute.Load(); err != nil {
			panic(err)
		}
	}
}

func TestLoadAndRegistry(t *testing.T) {
	r := attribute.Get()
	if r.Count() == 0 {
		t.Fatal("registry is empty")
	}

	// Verify the attributes from our data file exist.
	for _, name := range []string{"physical_power", "accuracy", "attack_interval", "defense"} {
		if _, ok := r.DefByString(name); !ok {
			t.Errorf("attribute %q not found", name)
		}
	}
}

func TestGetFinalDefault(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	id, _ := r.AttrID("physical_power")
	val := inst.GetFinal(id)
	def, _ := r.Def(id)

	if val != def.DefaultValue {
		t.Errorf("physical_power: want %v, got %v", def.DefaultValue, val)
	}
}

func TestAddModifier(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	id, _ := r.AttrID("physical_power")

	// Add +15 from equipment.
	inst.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})

	val := inst.GetFinal(id)
	if val != 25 { // 10 (default) + 15
		t.Errorf("want 25, got %v", val)
	}

	// Should be cached now.
	if inst.Dirty(id) {
		t.Error("should be clean after GetFinal")
	}
}

func TestRemoveModifier(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	id, _ := r.AttrID("physical_power")

	inst.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	inst.RemoveModifiers("equipment:sword")

	val := inst.GetFinal(id)
	if val != 10 { // back to default
		t.Errorf("want 10, got %v", val)
	}
}

func TestUpdateModifier(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	id, _ := r.AttrID("physical_power")

	// First equip.
	inst.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	// Replace with better sword.
	inst.UpdateModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: 30, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})

	val := inst.GetFinal(id)
	if val != 40 { // 10 + 30
		t.Errorf("want 40, got %v", val)
	}
}

func TestMultiplyModifier(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	id, _ := r.AttrID("physical_power")

	inst.AddModifiers("buff:berserk", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: 20, Display: attribute.DisplayFixed, Source: "buff:berserk"},
		{AttrID: id, Op: attribute.OpMultiply, Value: 0.3, Display: attribute.DisplayPercent, Source: "buff:berserk"},
	})

	val := inst.GetFinal(id)
	// (10 + 20) * (1 + 0.3) = 39
	if val != 39 {
		t.Errorf("want 39, got %v", val)
	}
}

func TestOverrideModifier(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	id, _ := r.AttrID("physical_power")

	inst.AddModifiers("base", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: 100, Display: attribute.DisplayFixed, Source: "base"},
		{AttrID: id, Op: attribute.OpOverride, Value: 50, Display: attribute.DisplayFixed, Source: "base"},
	})

	val := inst.GetFinal(id)
	// Override sets to exactly 50 (then clamp to min 0)
	if val != 50 {
		t.Errorf("want 50, got %v", val)
	}
}

func TestMinMaxClamp(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	id, _ := r.AttrID("attack_interval")

	// attack_interval has min_value=0.1, default=2.
	// Add massive negative to try to go below min.
	inst.AddModifiers("test", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: -10, Display: attribute.DisplayFixed, Source: "test"},
	})

	val := inst.GetFinal(id)
	if val != 0.1 { // clamped to min
		t.Errorf("want 0.1, got %v", val)
	}
}

func TestContextTempModifiers(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	id, _ := r.AttrID("physical_power")

	// Persistent: +10 from equipment.
	inst.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: 10, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})

	ctx := attribute.NewContext()
	ctx.AddMult(id, 0.5) // temp +50% multiplier

	// With context: (10 + 10) * 1.5 = 30
	valCtx := inst.GetFinalWithContext(id, ctx)
	if valCtx != 30 {
		t.Errorf("with context: want 30, got %v", valCtx)
	}

	// Without context: should still be 20 and cached.
	val := inst.GetFinal(id)
	if val != 20 {
		t.Errorf("without context: want 20, got %v", val)
	}

	// Cache should not have been polluted by context.
	if inst.Dirty(id) {
		t.Error("cache should be clean")
	}
}

func TestBucketDiff(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	reg := record.NewRegistry()
	reg.Register(attribute.Provider)

	rec := record.NewRecorder(reg)
	inst.SetRecorder(rec)

	id, _ := r.AttrID("physical_power")

	rec.PushNamespace("event_execution")
	inst.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})
	rec.PopNamespace()

	inst.ClearRecorder()

	diff, err := reg.BuildDiff(rec)
	if err != nil {
		t.Fatal(err)
	}

	if len(diff.Attribute) == 0 {
		t.Fatal("no attribute changes")
	}

	found := false
	for _, c := range diff.Attribute {
		if c.AttrId == "physical_power" {
			found = true
			if c.FinalValue != 25 {
				t.Errorf("final_value: want 25, got %v", c.FinalValue)
			}
		}
	}
	if !found {
		t.Error("physical_power not in diff")
	}
}

func TestFullSnapshot(t *testing.T) {
	inst := attribute.NewInstance()
	r := attribute.Get()

	id, _ := r.AttrID("physical_power")
	inst.AddModifiers("equipment:sword", []attribute.Modifier{
		{AttrID: id, Op: attribute.OpAdd, Value: 15, Display: attribute.DisplayFixed, Source: "equipment:sword"},
	})

	reg := record.NewRegistry()
	reg.Register(attribute.Provider)

	data, err := reg.BuildFullSnapshot(map[string]any{
		"attribute": inst,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(data.Attribute) != r.Count() {
		t.Errorf("want %d attributes, got %d", r.Count(), len(data.Attribute))
	}
}
