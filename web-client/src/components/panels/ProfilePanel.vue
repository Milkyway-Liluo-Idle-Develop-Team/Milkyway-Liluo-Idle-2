<template>
  <div class="profile-content">
    <h2>个人信息</h2>
    <p v-if="focusedCombatSkill" class="profile-focus">
      当前聚焦战斗技能: {{ focusedCombatSkill.name }} Lv {{ focusedCombatSkill.level }}
    </p>

    <section class="profile-card">
      <h3>生产属性</h3>
      <div class="profile-grid production-attr-grid">
        <article
          v-for="row in productionAttributes"
          :key="row.skill_id"
          class="profile-attr-cell"
        >
          <strong>{{ row.skill_name }}</strong>
          <span>等级 {{ row.base_level }} -> {{ row.effective_level }}</span>
          <span>产出倍率 x{{ row.total_output_multiplier.toFixed(3) }}</span>
          <span>速度倍率 x{{ row.total_speed_multiplier.toFixed(3) }}</span>
        </article>
      </div>
    </section>

    <section class="profile-card">
      <h3>战斗属性</h3>
      <div class="profile-grid battle-attr-grid">
        <article
          v-for="attr in battleAttributes"
          :key="attr.id"
          class="profile-attr-cell battle-attr-cell"
        >
          <strong>{{ attr.name }}</strong>
          <span class="attr-val">{{ formatBattleCurrentValue(attr) }}</span>
        </article>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import type {
  GameplayProfileProductionAttribute,
  GameplayProfileBattleAttribute,
  GameplaySkill,
} from '@/types/GameplayResponse'

defineProps<{
  productionAttributes: GameplayProfileProductionAttribute[]
  battleAttributes: GameplayProfileBattleAttribute[]
  focusedCombatSkill?: GameplaySkill | null
}>()

const formatBattleCurrentValue = (attr: GameplayProfileBattleAttribute) => {
  if (attr.as_percent) {
    return `${(attr.value * 100).toFixed(2)}%`
  }
  return `${attr.value.toFixed(2)}`
}
</script>

<style scoped>
.profile-content {
  min-height: 0;
  overflow: auto;
  display: grid;
  gap: 10px;
}

.profile-focus {
  margin: 0;
  color: var(--muted);
  font-size: 0.86rem;
}

.profile-card {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px;
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
  display: grid;
  gap: 8px;
}

.profile-grid {
  display: grid;
  gap: 8px;
}

.production-attr-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.battle-attr-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.profile-attr-cell {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 8px;
  background: color-mix(in srgb, var(--surface) 90%, transparent);
  display: grid;
  gap: 4px;
  font-size: 0.82rem;
}

.profile-attr-cell strong {
  font-size: 0.9rem;
}

.battle-attr-cell .attr-val {
  font-size: 0.9rem;
  font-weight: 800;
}

@media (max-width: 1280px) {
  .production-attr-grid {
    grid-template-columns: 1fr;
  }

  .battle-attr-grid {
    grid-template-columns: 1fr;
  }
}
</style>
