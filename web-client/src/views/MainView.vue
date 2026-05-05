<template>
  <div class="main-page">
    <header class="top-bar panel">
      <div class="title-wrap">
        <div class="logo">🌌</div>
        <div>
          <h1>银河莉萝放置</h1>
          <p>Milkyway Liluo Idle</p>
        </div>
      </div>

      <div v-if="activeLoopEvent" class="loop-progress-wrap">
        <div class="loop-meta">
          <strong>{{ activeLoopEvent.name }}</strong>
          <span>{{ Math.floor(loopProgress * 100) }}%</span>
          <span v-if="activeLoopAvailableText">可循环 {{ activeLoopAvailableText }}</span>
        </div>
        <div class="loop-track">
          <div class="loop-fill" :style="{ width: `${Math.min(100, Math.max(0, loopProgress * 100))}%` }"></div>
        </div>
        <button class="loop-stop" type="button" @click="stopLoop">停止</button>
      </div>

      <QueuePanel
        :items="queueItems"
        :index="queueIndex"
        :loading="queueActionLoading"
        @remove="queueRemove"
        @swap="queueSwap"
        @bring-to-front="queueBringToFront"
      />

      <button v-if="battleRunning && battleStatusText" class="battle-chip" type="button" @click="goBattlePage">
        {{ battleStatusText }}
      </button>

      <div class="actions">
        <button class="btn primary" type="button" :disabled="loading" @click="fetchData">
          {{ loading ? '刷新中...' : '刷新' }}
        </button>

        <button
          class="theme-switch"
          type="button"
          role="switch"
          :aria-checked="theme === 'dark'"
          :class="{ 'is-dark': theme === 'dark' }"
          @click="toggleTheme"
          :aria-label="themeAriaLabel"
        >
          <span class="thumb" aria-hidden="true">
            <svg v-if="theme === 'dark'" class="theme-icon" viewBox="0 0 24 24" fill="none">
              <path d="M21 14.2A8.2 8.2 0 0 1 9.8 3a6.8 6.8 0 1 0 11.2 11.2Z" />
            </svg>
            <svg v-else class="theme-icon" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="4.5" />
              <path d="M12 2.5v2.5M12 19v2.5M21.5 12h-2.5M5 12H2.5" />
              <path d="M19.4 4.6l-1.8 1.8M6.4 17.6l-1.8 1.8" />
              <path d="M19.4 19.4l-1.8-1.8M6.4 6.4L4.6 4.6" />
            </svg>
          </span>
        </button>

        <button class="btn ghost" type="button" @click="openProfilePanel">用户</button>
        <button class="btn ghost" type="button" :disabled="logoutLoading" @click="onLogout">
          {{ logoutLoading ? '退出中...' : '登出' }}
        </button>
      </div>
    </header>

    <section class="layout">
      <aside class="left panel">
        <div class="left-scroll">
          <div class="section-head">
            <h2>生产属性</h2>
            <button class="collapse-btn" type="button" @click="productionCollapsed = !productionCollapsed">
              {{ productionCollapsed ? '展开' : '收起' }}
            </button>
          </div>

          <div v-if="!productionCollapsed" class="skills-scroll">
            <button
              v-for="tab in productionTabs"
              :key="tab.id"
              class="skill-row"
              :class="{ active: selectedMenuId === tab.id }"
              type="button"
              @mouseenter="onSkillMouseEnter(tab.id)"
              @mousemove="onSkillMouseMove"
              @mouseleave="onSkillMouseLeave"
              @click="selectedMenuId = tab.id"
            >
              <div class="skill-title-wrap">
                <span class="skill-name">{{ tab.name }}</span>
                <span v-if="tab.isUpgrade" class="skill-sub">已解锁 {{ upgradeEvents.length }} 项</span>
              </div>

              <span
                class="skill-ring"
                :class="{ 'upgrade-ring': tab.isUpgrade }"
                :style="tab.isUpgrade ? undefined : { '--p': String(tab.level_progress) }"
              >
                <span v-if="!tab.isUpgrade && tab.level_progress > 0" class="ring-cap ring-cap-start"></span>
                <span v-if="!tab.isUpgrade && tab.level_progress > 0" class="ring-cap ring-cap-end"></span>
                <span class="ring-inner">{{ tab.isUpgrade ? 'UP' : tab.level }}</span>
              </span>
            </button>
          </div>

          <div class="section-head section-title">
            <h2 class="battle-title">
              <span>战斗属性</span>
              <span class="battle-level-text">战斗等级 {{ battleDisplayLevel }}</span>
            </h2>
            <button class="collapse-btn" type="button" @click="combatCollapsed = !combatCollapsed">
              {{ combatCollapsed ? '展开' : '收起' }}
            </button>
          </div>
          <div v-if="!combatCollapsed" class="skills-scroll">
            <button
              v-for="tab in combatTabs"
              :key="tab.id"
              class="skill-row combat-row"
              :class="{ active: selectedMenuId === tab.id }"
              type="button"
              @mouseenter="onSkillMouseEnter(tab.id)"
              @mousemove="onSkillMouseMove"
              @mouseleave="onSkillMouseLeave"
              @click="selectedMenuId = tab.id"
            >
              <div class="skill-title-wrap">
                <span class="skill-name">{{ tab.name }}</span>
              </div>

              <span class="skill-ring" :style="{ '--p': String(tab.level_progress) }">
                <span v-if="tab.level_progress > 0" class="ring-cap ring-cap-start"></span>
                <span v-if="tab.level_progress > 0" class="ring-cap ring-cap-end"></span>
                <span class="ring-inner">{{ tab.level }}</span>
              </span>
            </button>
          </div>

          <button
            class="profile-menu-btn battle-module-btn"
            type="button"
            :class="{ active: selectedMenuId === BATTLE_TAB_ID }"
            @click="selectedMenuId = BATTLE_TAB_ID"
          >
            战斗
          </button>
          <button
            class="profile-menu-btn market-module-btn"
            type="button"
            :class="{ active: selectedMenuId === MARKET_TAB_ID }"
            @click="openMarketPanel"
          >
            市场
          </button>

          <button
            class="profile-menu-btn"
            type="button"
            :class="{ active: selectedMenuId === PROFILE_TAB_ID }"
            @click="openProfilePanel"
          >
            个人信息
          </button>
        </div>
      </aside>

      <main
        class="center panel"
        :class="{ 'profile-mode': showProfilePanel, 'market-mode': showMarketPanel }"
      >
        <ProfilePanel
          v-if="showProfilePanel"
          :production-attributes="productionAttributeRows"
          :battle-attributes="battleAttributeRows"
          :focused-combat-skill="focusedCombatSkill"
        />

        <template v-else-if="showBattlePanel">
          <BattlePanel
            :entries="battleEntries"
            :selected-map-id="selectedBattleMapId"
            :battle-running="battleRunning"
            :active-battle-id="activeBattleId"
            :battle-action-loading="battleActionLoading"
            :visible-error="visibleError"
            :maps="maps"
            @select-map="selectedBattleMapId = $event"
            @toggle-battle="toggleBattle"
            @go-battle-page="goBattlePage"
          />
        </template>

        <template v-else-if="showMarketPanel">
          <MarketPanel
            :snapshot="marketSnapshot"
            :loading="marketLoading"
            :error="marketError"
            :sell-candidates="marketSellCandidates"
            @refresh="refreshMarket"
            @buy="handleMarketBuy"
            @create-listing="handleMarketCreateListing"
            @cancel-listing="handleMarketCancelListing"
          />
        </template>

        <template v-else>
          <div class="scene-col">
            <h2>场景</h2>
            <button
              v-for="scene in sceneTabs"
              :key="scene.id"
              type="button"
              class="scene-btn"
              :class="{ active: selectedMapId === scene.id }"
              @click="selectedMapId = scene.id"
            >
              {{ scene.name }}
            </button>
          </div>

          <div class="loop-col">
            <template v-if="showEnhancingPanel">
              <h2>装备强化</h2>
              <p v-if="visibleError" class="error">{{ visibleError }}</p>
              <div v-else-if="!enhancingAction" class="empty">当前场景下暂无可用赋能行动</div>
              <div v-else class="enhance-panel">
                <section class="enhance-block">
                  <h3>放入装备</h3>
                  <p class="enhance-desc">{{ enhancingAction.description || '选择已装备的装备或工具进行强化。' }}</p>
                  <div class="square-grid">
                    <button
                      v-for="target in enhanceTargets"
                      :key="target.key"
                      type="button"
                      class="square-cell enhance-target-cell"
                      :class="{ active: selectedEnhanceKey === target.key }"
                      @click="selectEnhanceTarget(target.key)"
                    >
                      <img :src="`/icons/items/${target.item_id}.svg`" :alt="target.item_name" class="cell-icon-img" loading="lazy" />
                      <span class="cell-name">{{ target.item_name }}</span>
                      <span class="cell-qty">{{ target.slot_type === 'tool' ? '生产' : '战斗' }}</span>
                      <span v-if="target.enhance_level > 0" class="cell-plus-badge">+{{ target.enhance_level }}</span>
                    </button>
                  </div>
                  <div v-if="!enhanceTargets.length" class="empty">当前没有可强化的已装备项目</div>
                </section>

                <section v-if="selectedEnhanceTarget && enhancePreview" class="enhance-block">
                  <div class="event-head">
                    <strong>{{ enhancePreview.item_name }}</strong>
                    <span class="tag" :class="{ blocked: isEnhanceDisabled }">
                      {{ isEnhanceDisabled ? '不可强化' : '可强化' }}
                    </span>
                  </div>

                  <div class="event-meta">
                    <span>等级 {{ enhancePreview.current_level }} -> {{ Math.min(enhancePreview.max_upgrade, enhancePreview.current_level + 1) }}</span>
                    <span>失败累计 {{ enhancePreview.current_fail_count }}</span>
                    <span>推荐赋能等级 {{ formatPreviewNumber(enhancePreview.recommend_level) }}</span>
                    <span>基础成功率 {{ formatRate(enhancePreview.basic_success_rate) }}</span>
                    <span>当前成功率 {{ formatRate(enhancePreview.display_success_rate) }}</span>
                    <span>本次判定率 {{ formatRate(enhancePreview.real_success_rate) }}</span>
                    <span>耗时 {{ formatSeconds(enhancePreview.cast_seconds) }}</span>
                    <span>经验 {{ formatPreviewNumber(enhancePreview.exp_on_execute) }}</span>
                  </div>

                  <div class="event-req" v-if="enhancePreview.requirements.length">
                    <span
                      v-for="req in enhancePreview.requirements"
                      :key="`${enhancePreview.item_id}-req-${req.item_id}`"
                      :class="{ blocked: req.lacking > 0 }"
                    >
                      {{ req.is_protection ? '保护' : '材料' }} {{ req.item_name }} x{{ req.needed }}
                      (持有 {{ req.owned }})
                    </span>
                  </div>

                  <div class="equip-attrs" v-if="enhancePreview.attribute_preview.length">
                    <div
                      v-for="attr in enhancePreview.attribute_preview"
                      :key="`${enhancePreview.item_id}-attr-${attr.key}`"
                      class="equip-attr-row"
                    >
                      <span>{{ equipmentAttrName(attr.key) }}</span>
                      <strong>{{ formatEquipmentAttrValue(attr.key, attr.current) }} -> {{ formatEquipmentAttrValue(attr.key, attr.next) }}</strong>
                    </div>
                  </div>

                  <div v-if="enhanceCasting" class="enhance-cast-wrap">
                    <div class="loop-track">
                      <div class="loop-fill" :style="{ width: `${Math.round(enhanceProgress * 100)}%` }"></div>
                    </div>
                    <span class="enhance-cast-text">强化中 {{ Math.round(enhanceProgress * 100) }}%</span>
                  </div>

                  <p v-if="enhanceResultText" class="enhance-result">{{ enhanceResultText }}</p>

                  <div class="event-actions-row">
                    <button
                      class="event-action"
                      type="button"
                      :disabled="isEnhanceDisabled"
                      @click="executeEnhance"
                    >
                      {{ enhanceActionLabel }}
                    </button>
                  </div>
                </section>

                <div v-else-if="enhanceLoading" class="empty">正在读取强化数据...</div>
                <div v-else class="empty">请选择一个已装备项目进行强化</div>
              </div>
            </template>

            <template v-else>
              <h2>{{ eventSectionTitle }}</h2>
              <p v-if="visibleError" class="error">{{ visibleError }}</p>

              <div v-else class="events-list">
                <article
                  v-for="event in currentEvents"
                  :key="event.id"
                  class="event-card"
                  :class="{ blocked: !event.is_executable }"
                >
                  <div class="event-head">
                    <strong>{{ displayEventName(event) }}</strong>
                    <span class="tag" :class="{ blocked: !event.is_executable }">
                      {{ event.is_executable ? '可执行' : event.is_skill_blocked ? '等级不足' : '条件不足' }}
                    </span>
                  </div>

                  <p>{{ event.description || '暂无描述' }}</p>

                  <div class="event-meta">
                    <span v-if="event.loop_time">{{ loopTimeLabel(event) }}</span>
                    <span
                      v-for="reward in event.reward_preview"
                      :key="`${event.id}-reward-${reward.item_id}`"
                    >
                      {{ rewardPreviewLabel(reward.item_name, reward.base_value, reward.effective_value) }}
                    </span>
                    <span v-if="event.experience">经验 {{ event.experience }}</span>
                    <span>触发次数 {{ event.event_count }}</span>
                    <span v-if="event.max_executions && event.max_executions > 1">上限 {{ event.max_executions }}</span>
                  </div>

                  <div class="event-req" v-if="event.cost_items.length || event.required_skills.length">
                    <span v-for="cost in event.cost_items" :key="`${event.id}-cost-${cost.item_id}`">
                      消耗 {{ cost.item_name }} x{{ cost.value }}
                    </span>
                    <span v-for="skillReq in event.required_skills" :key="`${event.id}-sk-${skillReq.skill_id}`">
                      {{ skillReq.skill_name }} {{ skillReq.comparison_text }} {{ skillReq.value }}
                    </span>
                  </div>

                  <div class="event-actions-row">
                    <button
                      class="event-action"
                      type="button"
                      :disabled="isEventActionDisabled(event)"
                      @click="onEventClick(event)"
                    >
                      {{ eventButtonLabel(event) }}
                    </button>
                    <input
                      v-if="event.type === 'loop'"
                      v-model="loopIterationsInput"
                      type="number"
                      min="0"
                      placeholder="∞"
                      class="iterations-input"
                      title="执行次数（留空为无限）"
                    />
                    <button
                      v-if="event.type === 'loop'"
                      class="event-action secondary"
                      type="button"
                      :disabled="!event.is_executable || queueActionLoading || loading"
                      @click="queueAppend(event, parseInt(loopIterationsInput || '0', 10) || undefined)"
                    >
                      +队列
                    </button>
                  </div>
                </article>

                <div v-if="!currentEvents.length" class="empty">{{ emptyText }}</div>
              </div>
            </template>
          </div>
        </template>
      </main>

      <aside class="right panel">
        <div class="right-tabs">
          <button
            class="right-tab-btn"
            :class="{ active: rightTab === 'items' }"
            type="button"
            @click="rightTab = 'items'"
          >
            物品
          </button>
          <button
            class="right-tab-btn"
            :class="{ active: rightTab === 'equipment' }"
            type="button"
            @click="rightTab = 'equipment'"
          >
            装备
          </button>
        </div>

        <div v-if="rightTab === 'items'" class="item-panels">
          <section v-for="panel in itemPanels" :key="panel.classification" class="item-panel">
            <h3>{{ panel.title }}</h3>
            <div class="square-grid">
              <button
                v-for="item in panel.items"
                :key="item.id"
                type="button"
                class="square-cell item-cell"
                @mouseenter="onInventoryItemMouseEnter(item, panel, $event)"
                @mousemove="onInventoryItemMouseMove"
                @mouseleave="onInventoryItemMouseLeave"
              >
                <img :src="`/icons/items/${item.id}.svg`" :alt="item.name" class="cell-icon-img" loading="lazy" />
                <span class="cell-name">{{ item.name }}</span>
                <span class="cell-qty">x{{ formatInventoryQuantity(item.quantity) }}</span>
              </button>
            </div>
          </section>
          <div v-if="!itemPanels.length" class="empty">暂无可显示物品</div>
        </div>

        <div v-else class="equipment-pane">
          <h3>生产装备</h3>
          <div class="square-grid">
            <button
              v-for="slot in productionSlots"
              :key="`tool-${slot.slot_id}`"
              type="button"
              class="square-cell equip-slot-cell"
              :class="{ disabled: slot.is_disabled, active: selectedSlot?.slot_id === slot.slot_id && selectedSlot?.slot_type === slot.slot_type }"
              :disabled="slot.is_disabled || equipLoading"
              @mouseenter="onEquipMouseEnter(slot, $event)"
              @mousemove="onEquipMouseMove"
              @mouseleave="onEquipMouseLeave"
              @click="openSlotPanel(slot)"
            >
              <span v-if="slot.is_disabled" class="cell-icon">🚫</span>
              <img v-else-if="slot.item_id" :src="`/icons/items/${slot.item_id}.svg`" :alt="slot.item_name || ''" class="cell-icon-img" loading="lazy" />
              <span class="cell-name">{{ slot.item_name || slot.slot_name }}</span>
              <span v-if="slot.item_id && (slot.enhance_level ?? 0) > 0" class="cell-plus-badge">+{{ slot.enhance_level }}</span>
            </button>
          </div>

          <h3>战斗装备</h3>
          <div class="square-grid">
            <button
              v-for="slot in battleSlots"
              :key="`equip-${slot.slot_id}`"
              type="button"
              class="square-cell equip-slot-cell"
              :class="{ disabled: slot.is_disabled, active: selectedSlot?.slot_id === slot.slot_id && selectedSlot?.slot_type === slot.slot_type }"
              :disabled="slot.is_disabled || equipLoading"
              @mouseenter="onEquipMouseEnter(slot, $event)"
              @mousemove="onEquipMouseMove"
              @mouseleave="onEquipMouseLeave"
              @click="openSlotPanel(slot)"
            >
              <span v-if="slot.is_disabled" class="cell-icon">🚫</span>
              <img v-else-if="slot.item_id" :src="`/icons/items/${slot.item_id}.svg`" :alt="slot.item_name || ''" class="cell-icon-img" loading="lazy" />
              <span class="cell-name">{{ slot.item_name || slot.slot_name }}</span>
              <span v-if="slot.item_id && (slot.enhance_level ?? 0) > 0" class="cell-plus-badge">+{{ slot.enhance_level }}</span>
            </button>
          </div>

          <p v-if="equipError" class="error">{{ equipError }}</p>

          <div v-if="selectedSlot" class="slot-picker">
            <h3>{{ selectedSlot.slot_name }} 可装备项目</h3>
            <div class="square-grid">
              <button type="button" class="square-cell picker-action" :disabled="equipLoading" @click="unequipFromSelected">
                卸下装备
              </button>
              <button type="button" class="square-cell picker-action" :disabled="equipLoading" @click="closeSlotPanel">
                返回
              </button>

              <button
                v-for="entry in slotCandidates"
                :key="`${selectedSlot.slot_id}-${entry.id}-${entry.slot_type}`"
                type="button"
                class="square-cell"
                :class="{ disabled: !entry.canEquip }"
                :disabled="equipLoading || !entry.canEquip"
                @click="equipToSelected(entry.id)"
              >
                <img :src="`/icons/items/${entry.id}.svg`" :alt="entry.name" class="cell-icon-img" loading="lazy" />
                <span class="cell-name">{{ entry.name }}</span>
                <span class="cell-qty">x{{ entry.quantity }}</span>
              </button>
            </div>
          </div>
        </div>
      </aside>
    </section>

    <Teleport to="body">
      <div
        v-if="hoveredSkillTab"
        class="skill-float-tip"
        :style="{ left: `${skillTipPos.x}px`, top: `${skillTipPos.y}px` }"
      >
        <div class="tip-exp-row">
          <span>经验</span>
          <span>{{ hoveredSkillTab.current_level_exp.toFixed(0) }} / {{ hoveredSkillTab.required_level_exp.toFixed(0) }}</span>
        </div>
        <div class="tip-track">
          <div class="tip-fill" :style="{ width: `${Math.round(hoveredSkillTab.level_progress * 100)}%` }"></div>
        </div>
        <template v-if="hoveredSkillTab.kind === 'production'">
          <p>每级提供 <span class="tip-green">0.3%</span> 的乘算加成.</p>
          <p>
            当前累计:
            <span class="tip-green">{{ ((hoveredSkillTab.level_production_multiplier * 100) - 100).toFixed(2) }}%</span>
            的产物加成
          </p>
        </template>
        <template v-else>
          <p>{{ combatSkillHintMap[hoveredSkillTab.id] ?? '战斗技能会强化对应战斗属性。' }}</p>
        </template>
      </div>

      <div
        v-if="hoveredEquipSlot && hoveredEquipSlot.item_id"
        class="equip-float-tip"
        :style="{ left: `${equipTipPos.x}px`, top: `${equipTipPos.y}px` }"
      >
        <h4>{{ hoveredEquipSlot.item_name || hoveredEquipSlot.item_id }}</h4>
        <p v-if="(hoveredEquipSlot.enhance_level ?? 0) > 0">强化等级 Lv.{{ hoveredEquipSlot.enhance_level }}</p>
        <p v-if="(hoveredEquipSlot.enhance_fail_count ?? 0) > 0">失败累计 {{ hoveredEquipSlot.enhance_fail_count }}</p>
        <div v-if="hoveredEquipSlot.attribute_preview.length" class="equip-attrs">
          <div v-for="attr in hoveredEquipSlot.attribute_preview" :key="`${hoveredEquipSlot.slot_id}-${attr.key}`" class="equip-attr-row">
            <span>{{ equipmentAttrName(attr.key) }}</span>
            <strong>{{ formatEquipmentAttrValue(attr.key, attr.value) }}</strong>
          </div>
        </div>
        <p v-else>暂无属性加成</p>
      </div>

      <div
        v-if="hoveredInventoryItem"
        class="item-float-tip"
        :style="{ left: `${inventoryTipPos.x}px`, top: `${inventoryTipPos.y}px` }"
      >
        <h4>{{ hoveredInventoryItem.name }}</h4>
        <p>类别: {{ hoveredInventoryItem.classification_title }}</p>
        <p>数量: {{ hoveredInventoryItem.quantity_raw }}</p>
        <div v-if="hoveredInventoryItem.attribute_preview.length" class="equip-attrs">
          <div
            v-for="attr in hoveredInventoryItem.attribute_preview"
            :key="`${hoveredInventoryItem.id}-${attr.key}`"
            class="equip-attr-row"
          >
            <span>{{ equipmentAttrName(attr.key) }}</span>
            <strong>{{ formatEquipmentAttrValue(attr.key, attr.value) }}</strong>
          </div>
        </div>
      </div>

      <!-- 时间推进悬浮面板 -->
      <div class="skip-time-fab-wrap">
        <button
          class="skip-time-fab"
          type="button"
          title="时间推进"
          @click="skipTimePanelOpen = !skipTimePanelOpen"
        >
          ⏱️
        </button>

        <div v-if="skipTimePanelOpen" class="skip-time-panel">
          <div class="skip-time-header">
            <strong>时间推进</strong>
            <button class="btn icon" type="button" @click="skipTimePanelOpen = false">✕</button>
          </div>
          <div class="skip-time-body">
            <label class="skip-time-label">
              <span>推进时长</span>
              <input
                v-model.number="skipTimeSeconds"
                type="number"
                min="0"
                step="1"
                class="skip-time-input"
                @keydown.enter="handleSkipTime"
              />
              <span class="skip-time-unit">秒</span>
            </label>
            <button
              class="btn primary skip-time-btn"
              type="button"
              :disabled="skipTimeLoading"
              @click="handleSkipTime"
            >
              {{ skipTimeLoading ? '推进中...' : '跳过时间' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useTheme } from '@/composables/useTheme'
import * as actions from '@/lib/actions'
import { clearAuthCache, logout } from '@/lib/auth'
import { disconnect } from '@/lib/ws'
import {
  buyMarketListing,
  cancelMarketListing,
  createMarketListing,
  fetchMarketSnapshot,
} from '@/lib/market'
import { useGameStore } from '@/stores/game'
import QueuePanel from '@/components/panels/QueuePanel.vue'
import BattlePanel from '@/components/panels/BattlePanel.vue'
import ProfilePanel from '@/components/panels/ProfilePanel.vue'
import MarketPanel from '@/components/panels/MarketPanel.vue'
import IconChevronUp from '@/components/icons/IconChevronUp.vue'
import IconChevronDown from '@/components/icons/IconChevronDown.vue'
import IconArrowToTop from '@/components/icons/IconArrowToTop.vue'
import IconClose from '@/components/icons/IconClose.vue'
import type {
  GameplayData,
  GameplayEvent,
  QueueItem,
} from '@/types/GameplayResponse'
import type { BattleListItem, BattleState } from '@/types/BattleResponse'
import type { MarketSellCandidate, MarketSnapshot } from '@/types/Market'

const UPGRADE_TAB_ID = '__upgrade__'
const BATTLE_TAB_ID = '__battle__'
const MARKET_TAB_ID = '__market__'
const PROFILE_TAB_ID = '__profile__'
const ENHANCE_SCENE_ID = '__enhance_scene__'

const router = useRouter()
const { theme, toggleTheme, themeAriaLabel } = useTheme()
const store = useGameStore()

// Adapter: assemble store state into GameplayData shape so template needs minimal changes
const data = computed<GameplayData | null>(() => {
  if (!store.state) return null
  return {
    ...store.state as any,
    production_skills: store.productionSkills,
    combat_skills: store.combatSkills,
    profile: store.profile,
    maps: store.maps,
    loop_events: store.loopEvents,
    upgrade_events: store.upgradeEvents,
    item_panels: store.itemPanels,
    active_loop: store.activeLoop,
    queue: store.queue,
    equipment_view: store.equipmentView,
  } as GameplayData
})

const loading = computed(() => store.stateLoading || store.staticLoading)
const error = computed(() => store.stateError || store.staticError)
const actionError = computed(() => store.actionError)
const loopEvents = computed(() => store.loopEvents)
const upgradeEvents = computed(() => store.upgradeEvents)
const itemPanels = computed(() => store.itemPanels)
const queue = computed(() => store.queue)
const activeLoop = computed(() => store.activeLoop)
const profile = computed(() => store.profile)
const maps = computed(() => store.maps)
const equipmentView = computed(() => store.equipmentView)

const fetchData = () => store.fetchGameplayData()
const fetchActionsData = () => store.fetchStaticData()

const selectedMenuId = ref('')
const selectedMapId = ref('')
const productionCollapsed = ref(false)
const combatCollapsed = ref(false)
const rightTab = ref<'items' | 'equipment'>('items')
const equipLoading = computed(() => store.isActionLoading('equip'))
const equipError = ref('')
const battleActionLoading = ref(false)
const logoutLoading = ref(false)
const battleEntries = computed(() => store.battleEntries)
const battleState = computed(() => store.battleState)
const selectedBattleMapId = ref('')
const marketSnapshot = ref<MarketSnapshot | null>(null)
const marketLoading = ref(false)
const marketError = ref('')

const activeLoopEventId = ref('')
const activeLoopAvailableIterations = ref<number | null>(null)
const loopProgress = ref(0)
const loopDurationSeconds = ref(1)
const loopStartedAtMs = ref(0)
const LOOP_CLIENT_LATENCY_MS = 200
const loopTickInFlight = ref(false)
let loopTimer: number | null = null

const skipTimeSeconds = ref<number>(60)
const skipTimeLoading = ref(false)
const skipTimePanelOpen = ref(false)

const handleSkipTime = async () => {
  const seconds = Number(skipTimeSeconds.value)
  if (!Number.isFinite(seconds) || seconds < 0) return
  skipTimeLoading.value = true
  try {
    await actions.skipTime(seconds)
    await store.fetchGameplayData()
  } catch (e: any) {
    alert('跳过时间失败: ' + (e?.message || '未知错误'))
  } finally {
    skipTimeLoading.value = false
  }
}

const queueItems = computed(() => queue.value?.items ?? [])
const queueIndex = computed(() => queue.value?.index ?? 0)
const queueProgress = computed(() => queue.value?.progress_seconds ?? 0)
const queueActionLoading = computed(() => store.isActionLoading('queue'))
const loopIterationsInput = ref<string>('') // empty = unlimited

const productionSkills = computed(() => store.productionSkills)
const combatSkills = computed(() => store.combatSkills)
const productionAttributeRows = computed(() => profile.value.production_attributes)
const battleAttributeRows = computed(() => profile.value.battle_attributes)
const enhancingGateEvent = computed(
  () =>
    loopEvents.value.find((event) => event.id === 'polish_objects_for_villager')
    ?? loopEvents.value.find((event) => event.need_skill === 'enhancing')
    ?? null,
)
const sceneTabs = computed(() => {
  const out = [...maps.value]
  if (selectedEventCategoryId.value === 'enhancing' && enhancingGateEvent.value) {
    out.push({ id: ENHANCE_SCENE_ID, name: '强化物品' })
  }
  return out
})
const productionSlots = computed(() => store.equipmentView.production_slots)
const battleSlots = computed(() => store.equipmentView.battle_slots)
const actionItemMap = computed(() => store.items as Record<string, any>)
const marketSellCandidates = computed<MarketSellCandidate[]>(() => {
  const inventory = data.value?.inventory || []
  const totals: Record<string, number> = {}
  for (const entry of inventory) {
    if (entry.qty <= 0) continue
    totals[entry.id] = (totals[entry.id] || 0) + entry.qty
  }
  return Object.entries(totals)
    .map(([itemId, qty]) => {
      const item = actionItemMap.value[itemId] || {}
      return {
        itemId,
        itemName: item.name || itemId,
        quantity: Math.max(0, Math.floor(Number(qty) || 0)),
        classification: item.classification || 'other',
      }
    })
    .sort((a, b) => a.itemName.localeCompare(b.itemName, 'zh-CN'))
})

type SlotCell = {
  slot_type: 'tool' | 'equipment'
  slot_id: string
  slot_name: string
  item_id: string | null
  item_name: string | null
  anchor_slot: string | null
  is_disabled: boolean
  enhance_level: number | null
  enhance_fail_count: number | null
  attribute_preview: Array<{ key: string; value: number }>
}

const selectedSlot = ref<SlotCell | null>(null)
const hoveredSkillId = ref('')
const skillTipPos = ref({ x: 0, y: 0 })
const hoveredEquipSlot = ref<SlotCell | null>(null)
const equipTipPos = ref({ x: 0, y: 0 })
type InventoryHoverItem = {
  id: string
  name: string
  classification: string
  classification_title: string
  quantity: number
  quantity_raw: string
  attribute_preview: Array<{ key: string; value: number }>
}
const hoveredInventoryItem = ref<InventoryHoverItem | null>(null)
const inventoryTipPos = ref({ x: 0, y: 0 })

type EnhanceTarget = {
  key: string
  slot_type: 'tool' | 'equipment'
  anchor_slot: string
  slot_id: string
  slot_name: string
  item_id: string
  item_name: string
  enhance_level: number
  enhance_fail_count: number
}

type EnhanceRequirement = {
  item_id: string
  item_name: string
  needed: number
  owned: number
  lacking: number
  is_protection: boolean
}

type EnhanceAttributePreview = {
  key: string
  current: number
  next: number
}

type EnhancePreview = {
  slot_type: 'tool' | 'equipment'
  anchor_slot: string
  item_id: string
  item_name: string
  current_level: number
  current_fail_count: number
  max_upgrade: number
  at_max_level: boolean
  recommend_level: number
  basic_success_rate: number
  display_success_rate: number
  real_success_rate: number
  enhancing_level: number
  requirements: EnhanceRequirement[]
  has_enough_requirements: boolean
  attribute_preview: EnhanceAttributePreview[]
  cast_seconds: number
  exp_on_execute: number
}

const selectedEnhanceKey = ref('')
const enhancePreview = ref<EnhancePreview | null>(null)
const enhanceLoading = ref(false)
const enhanceCasting = ref(false)
const enhanceProgress = ref(0)
const enhanceResultText = ref('')
let enhanceCastTimer: number | null = null
let enhancePreviewReqSeq = 0

const activeLoopEvent = computed(() =>
  loopEvents.value.find((event) => event.id === activeLoopEventId.value) ?? null,
)

const battleRunning = computed(() => {
  const state = battleState.value
  if (!state) return false
  return state.status !== 'stopped'
})

const activeBattleId = computed(() => battleState.value?.battle_id || '')

const showBattlePanel = computed(() => selectedMenuId.value === BATTLE_TAB_ID)
const showMarketPanel = computed(() => selectedMenuId.value === MARKET_TAB_ID)

const productionBlockedByBattle = computed(() => false)

type SkillTab = {
  id: string
  name: string
  level: number
  exp: number
  level_progress: number
  current_level_total_exp: number
  next_level_total_exp: number
  current_level_exp: number
  required_level_exp: number
  level_production_multiplier: number
  isUpgrade: boolean
  kind: 'production' | 'combat' | 'upgrade'
}

const productionTabs = computed<SkillTab[]>(() => {
  const skillTabs = productionSkills.value.map((skill) => ({
    id: skill.id,
    name: skill.name,
    level: skill.level,
    exp: skill.exp,
    level_progress: skill.level_progress,
    current_level_total_exp: skill.current_level_total_exp,
    next_level_total_exp: skill.next_level_total_exp,
    current_level_exp: Math.max(0, skill.exp - skill.current_level_total_exp),
    required_level_exp: Math.max(0, skill.next_level_total_exp - skill.current_level_total_exp),
    level_production_multiplier: skill.level_production_multiplier,
    isUpgrade: false,
    kind: 'production' as const,
  }))
  return [
    ...skillTabs,
    {
      id: UPGRADE_TAB_ID,
      name: '升级行动',
      level: upgradeEvents.value.length,
      level_progress: 1,
      exp: 0,
      current_level_total_exp: 0,
      next_level_total_exp: 0,
      current_level_exp: 0,
      required_level_exp: 0,
      level_production_multiplier: 1,
      isUpgrade: true,
      kind: 'upgrade' as const,
    },
  ]
})

const combatTabs = computed<SkillTab[]>(() =>
  combatSkills.value.map((skill) => ({
    id: skill.id,
    name: skill.name,
    level: skill.level,
    exp: skill.exp,
    level_progress: skill.level_progress,
    current_level_total_exp: skill.current_level_total_exp,
    next_level_total_exp: skill.next_level_total_exp,
    current_level_exp: Math.max(0, skill.exp - skill.current_level_total_exp),
    required_level_exp: Math.max(0, skill.next_level_total_exp - skill.current_level_total_exp),
    level_production_multiplier: skill.level_production_multiplier,
    isUpgrade: false,
    kind: 'combat' as const,
  })),
)

const battleDisplayLevel = computed(() => {
  const total = combatSkills.value.reduce((sum, skill) => sum + Number(skill.level || 0), 0)
  return (total / 7).toFixed(2)
})

const allSkillTabs = computed(() => [
  ...productionTabs.value.filter((tab) => !tab.isUpgrade),
  ...combatTabs.value,
])

const hoveredSkillTab = computed(() => {
  if (!hoveredSkillId.value) return null
  return allSkillTabs.value.find((tab) => tab.id === hoveredSkillId.value) ?? null
})

const selectedEventCategoryId = computed(() => {
  if (selectedMenuId.value === UPGRADE_TAB_ID) return UPGRADE_TAB_ID
  const isProductionSkill = productionTabs.value.some(
    (tab) => !tab.isUpgrade && tab.id === selectedMenuId.value,
  )
  return isProductionSkill ? selectedMenuId.value : ''
})

const showEnhancingPanel = computed(
  () =>
    selectedEventCategoryId.value === 'enhancing'
    && selectedMapId.value === ENHANCE_SCENE_ID
    && Boolean(enhancingGateEvent.value),
)

const showProfilePanel = computed(() => {
  if (selectedMenuId.value === PROFILE_TAB_ID) return true
  return combatTabs.value.some((tab) => tab.id === selectedMenuId.value)
})

const focusedCombatSkill = computed(() =>
  combatTabs.value.find((tab) => tab.id === selectedMenuId.value) ?? null,
)

const combatSkillHintMap: Record<string, string> = {
  strength: '每级提供 +1 物理攻击，并附带物理伤害乘算成长。',
  ranging: '每级提高精准和闪避，增强命中与规避能力。',
  resilience: '每级提高生命上限。',
  stamina: '每级提高耐力上限与耐力恢复。',
  intelligence: '每级提高魔力上限与魔力恢复。',
  defense: '每级提高防御和格挡能力。',
  magic: '每级提供 +1 奥术攻击，并附带奥术伤害乘算成长。',
}

const equipmentAttrNameMap: Record<string, string> = {
  physical_power: '物理基础伤害',
  magic_power: '奥术基础伤害',
  power_multiplier: '最终基础伤害倍率',
  attack_interval: '基础攻击速度',
  attack_speed: '攻击速度加成',
  final_attack_speed_multiplier: '最终攻击速度加成',
  critical: '暴击率',
  critical_possibility_multiplier: '最终暴击率倍率',
  critical_rate: '暴击伤害',
  block: '格挡值',
  block_multiplier: '格挡值倍率',
  block_possibility_multiplier: '最终格挡值倍率',
  block_rate: '格挡减伤',
  block_rate_multiplier: '最终格挡倍率',
  hp_recovery: '生命自动回复',
  mp_recovery: '魔力自动回复',
  sp_recovery: '耐力自动回复',
  overall_recovery_speed: '自然恢复速率倍率',
  accuracy: '精准度',
  accuracy_multiplier: '最终精准度倍率',
  accuracy_possibility_multiplier: '最终命中概率倍率',
  evade: '闪避度',
  evade_multiplier: '最终闪避概率倍率',
  evade_possibility_multiplier: '最终闪避概率乘算',
  magic_instance: '奥术抵抗',
  magic_instance_multiplier: '奥术抵抗倍率',
  final_damage_multiplier: '最终伤害倍率',
  defense: '防御值',
  defense_multiplier: '防御值倍率',
  final_damage_induce: '最终伤害减少',
  final_damage_reduce: '最终伤害减少',
  hatred: '仇恨',
  hatred_multiplier: '仇恨值倍率',
  max_hp: '生命值上限',
  max_mp: '魔力值上限',
  max_sp: '耐力值上限',
  hp_multiplier: '生命值上限倍率',
  mp_multiplier: '魔力值上限倍率',
  sp_multiplier: '耐力值上限倍率',
  felling_production_multiplier: '砍伐产出加成',
  felling_level_buff: '砍伐等级加成',
  felling_speed_multiplier: '砍伐速度加成',
  mining_production_multiplier: '采矿产出加成',
  mining_level_buff: '采矿等级加成',
  mining_speed_multiplier: '采矿速度加成',
  planting_recycle_multipler: '种植回收概率加成',
  planting_speed_multiplier: '种植速度加成',
  planting_production_multiplier: '种植产出加成',
  crafting_production_multiplier: '制造产出加成',
  crafting_level_buff: '制造等级加成',
  crafting_speed_multiplier: '制造速度加成',
  forging_production_multiplier: '锻造产出加成',
  forging_level_buff: '锻造等级加成',
  forging_speed_multiplier: '锻造速度加成',
  enhancing_level_buff: '赋能等级加成',
  enhancing_success_rate_multiplier: '赋能成功率加成',
}

const equipmentPercentLikeKeys = new Set<string>([
  'power_multiplier',
  'attack_speed',
  'final_attack_speed_multiplier',
  'critical',
  'critical_possibility_multiplier',
  'block_multiplier',
  'block_possibility_multiplier',
  'block_rate',
  'block_rate_multiplier',
  'overall_recovery_speed',
  'accuracy_multiplier',
  'accuracy_possibility_multiplier',
  'evade_multiplier',
  'evade_possibility_multiplier',
  'magic_instance',
  'magic_instance_multiplier',
  'final_damage_multiplier',
  'final_damage_induce',
  'final_damage_reduce',
  'defense_multiplier',
  'hatred_multiplier',
  'hp_multiplier',
  'mp_multiplier',
  'sp_multiplier',
  'felling_production_multiplier',
  'felling_speed_multiplier',
  'mining_production_multiplier',
  'mining_speed_multiplier',
  'planting_recycle_multipler',
  'planting_speed_multiplier',
  'planting_production_multiplier',
  'crafting_production_multiplier',
  'crafting_speed_multiplier',
  'forging_production_multiplier',
  'forging_speed_multiplier',
  'enhancing_success_rate_multiplier',
])

const slotBase = (slotId: string) => {
  const base = slotId.split('#', 1)[0]
  return base ?? ''
}

const slotCandidates = computed(() => {
  const slot = selectedSlot.value
  if (!slot) return []
  const base = slotBase(slot.slot_id)
  return equipmentView.value.equipable_items
    .filter((entry) => entry.slot_type === slot.slot_type)
    .filter((entry) => entry.required_slots.includes(base))
    .map((entry) => ({
      ...entry,
      canEquip: canEquipEntryToSlot(entry, slot),
    }))
})

const enhanceTargets = computed<EnhanceTarget[]>(() => {
  const out: EnhanceTarget[] = []
  const seen = new Set<string>()
  const addFromSlots = (slots: SlotCell[]) => {
    for (const slot of slots) {
      if (!slot.item_id) continue
      const anchor = slot.anchor_slot || slot.slot_id
      const master = anchor === slot.slot_id
      if (!master) continue
      const itemDef = actionItemMap.value[slot.item_id]
      if (!itemDef || !itemDef.upgradable) continue
      const key = `${slot.slot_type}:${anchor}`
      if (seen.has(key)) continue
      seen.add(key)
      out.push({
        key,
        slot_type: slot.slot_type,
        anchor_slot: anchor,
        slot_id: slot.slot_id,
        slot_name: slot.slot_name,
        item_id: slot.item_id,
        item_name: slot.item_name || slot.item_id,
        enhance_level: Math.max(0, Number(slot.enhance_level || 0)),
        enhance_fail_count: Math.max(0, Number(slot.enhance_fail_count || 0)),
      })
    }
  }
  addFromSlots(productionSlots.value as SlotCell[])
  addFromSlots(battleSlots.value as SlotCell[])
  return out.sort((a, b) => a.item_name.localeCompare(b.item_name))
})

const selectedEnhanceTarget = computed<EnhanceTarget | null>(() => {
  if (!selectedEnhanceKey.value) return null
  return enhanceTargets.value.find((entry) => entry.key === selectedEnhanceKey.value) ?? null
})

const enhancingAction = computed(() => enhancingGateEvent.value)

const currentEvents = computed(() => {
  if (selectedEventCategoryId.value === UPGRADE_TAB_ID) {
    return upgradeEvents.value.filter((event) => event.map === selectedMapId.value)
  }
  if (selectedEventCategoryId.value === 'enhancing' && selectedMapId.value === ENHANCE_SCENE_ID) {
    return []
  }
  return loopEvents.value.filter(
    (event) => event.need_skill === selectedEventCategoryId.value && event.map === selectedMapId.value,
  )
})

const eventSectionTitle = computed(() =>
  selectedEventCategoryId.value === UPGRADE_TAB_ID ? '升级行动' : '循环行动',
)

const emptyText = computed(() =>
  selectedEventCategoryId.value === UPGRADE_TAB_ID
    ? '当前场景下暂无可见升级行动'
    : '当前场景下暂无可见循环行动',
)

const visibleError = computed(() => actionError.value || error.value)

const isEnhanceDisabled = computed(() => {
  if (enhanceCasting.value || enhanceLoading.value || loading.value) return true
  if (productionBlockedByBattle.value) return true
  if (!enhancingAction.value || !enhancingAction.value.is_executable) return true
  const preview = enhancePreview.value
  if (!preview) return true
  if (preview.at_max_level) return true
  if (!preview.has_enough_requirements) return true
  return false
})

const enhanceActionLabel = computed(() => {
  if (enhanceCasting.value) return `强化中 ${Math.round(enhanceProgress.value * 100)}%`
  const preview = enhancePreview.value
  if (!preview) return '选择装备'
  if (preview.at_max_level) return '已达满级'
  if (!preview.has_enough_requirements) return '材料不足'
  if (productionBlockedByBattle.value) return '战斗中'
  if (!enhancingAction.value?.is_executable) return '条件不足'
  return `开始强化 (${formatSeconds(preview.cast_seconds)})`
})

const activeLoopAvailableText = computed(() => {
  const value = activeLoopAvailableIterations.value
  if (value === null || value === undefined) return '∞'
  return `${Math.max(0, Math.floor(value))} 次`
})

const openProfilePanel = () => {
  selectedMenuId.value = PROFILE_TAB_ID
}

const onLogout = async () => {
  logoutLoading.value = true
  try {
    const res = await logout()
    clearAuthCache()
    store.disposeWsListeners()
    disconnect()
    store.resetState()
    if (!res.ok) {
      marketError.value = res.error
      return
    }
    window.location.href = '/login'
  } finally {
    logoutLoading.value = false
  }
}

const loadMarketSnapshot = async () => {
  marketSnapshot.value = await fetchMarketSnapshot()
}

const refreshMarket = async () => {
  marketLoading.value = true
  marketError.value = ''
  try {
    await loadMarketSnapshot()
  } catch (e: any) {
    marketError.value = e?.message || '加载市场失败'
  } finally {
    marketLoading.value = false
  }
}

const runMarketAction = async (action: () => Promise<unknown>, fallbackMessage: string) => {
  marketLoading.value = true
  marketError.value = ''
  try {
    await action()
    await Promise.all([store.fetchGameplayData(), loadMarketSnapshot()])
  } catch (e: any) {
    marketError.value = e?.message || fallbackMessage
  } finally {
    marketLoading.value = false
  }
}

const handleMarketCreateListing = async (payload: {
  itemId: string
  quantity: number
  unitPrice: number
}) => {
  await runMarketAction(
    () =>
      createMarketListing({
        itemId: payload.itemId,
        quantity: payload.quantity,
        unitPrice: payload.unitPrice,
      }),
    '发布挂单失败',
  )
}

const handleMarketBuy = async (payload: { listingId: number; quantity: number }) => {
  await runMarketAction(
    () =>
      buyMarketListing({
        listingId: payload.listingId,
        quantity: payload.quantity,
      }),
    '购买失败',
  )
}

const handleMarketCancelListing = async (listingId: number) => {
  await runMarketAction(() => cancelMarketListing(listingId), '撤销挂单失败')
}

const openMarketPanel = () => {
  selectedMenuId.value = MARKET_TAB_ID
  void refreshMarket()
}

const placeTooltip = (event: MouseEvent, width: number, height: number) => {
  const gap = 16
  const viewportW = window.innerWidth || 1920
  const viewportH = window.innerHeight || 1080
  const x = Math.max(8, Math.min(viewportW - width - 8, event.clientX + gap))
  const y = Math.max(8, Math.min(viewportH - height - 8, event.clientY + gap))
  return { x, y }
}

const onSkillMouseEnter = (skillId: string) => {
  const tab = allSkillTabs.value.find((entry) => entry.id === skillId)
  if (!tab) {
    hoveredSkillId.value = ''
    return
  }
  hoveredSkillId.value = skillId
}

const onSkillMouseMove = (event: MouseEvent) => {
  skillTipPos.value = placeTooltip(event, 320, 180)
}

const onSkillMouseLeave = () => {
  hoveredSkillId.value = ''
}

const onEquipMouseEnter = (slot: SlotCell, event: MouseEvent) => {
  if (!slot.item_id) {
    hoveredEquipSlot.value = null
    return
  }
  hoveredEquipSlot.value = slot
  equipTipPos.value = placeTooltip(event, 360, 320)
}

const onEquipMouseMove = (event: MouseEvent) => {
  if (!hoveredEquipSlot.value) return
  equipTipPos.value = placeTooltip(event, 360, 320)
}

const onEquipMouseLeave = () => {
  hoveredEquipSlot.value = null
}

const equipmentAttrName = (key: string) => equipmentAttrNameMap[key] || key

const formatEquipmentAttrValue = (key: string, value: number) => {
  if (!Number.isFinite(value)) return '0'
  if (equipmentPercentLikeKeys.has(key)) {
    return `${value >= 0 ? '+' : ''}${(value * 100).toFixed(2)}%`
  }
  const abs = Math.abs(value)
  const fixed = abs >= 10 ? value.toFixed(1) : value.toFixed(2)
  return `${value >= 0 ? '+' : ''}${fixed}`
}

const formatInventoryQuantity = (value: number) => {
  const safe = Number.isFinite(value) ? Math.max(0, value) : 0
  const units: Array<{ threshold: number; suffix: string }> = [
    { threshold: 1e12, suffix: 't' },
    { threshold: 1e9, suffix: 'b' },
    { threshold: 1e6, suffix: 'm' },
    { threshold: 1e3, suffix: 'k' },
  ]
  for (const unit of units) {
    if (safe < unit.threshold) continue
    const scaled = safe / unit.threshold
    const fixed = scaled >= 100 ? 0 : scaled >= 10 ? 1 : 2
    const text = scaled.toFixed(fixed).replace(/\.0+$|(\.\d*[1-9])0+$/, '$1')
    return `${text}${unit.suffix}`
  }
  return `${Math.floor(safe)}`
}

const resolveItemAbilityMultiplierForDisplay = (item: any, level: number) => {
  const curve = item?.upgrade_details?.upgrade_curve
  if (!Array.isArray(curve) || curve.length === 0) return 1
  const pointsMap = new Map<number, number>([[0, 1]])
  for (const row of curve) {
    const lv = Math.floor(Number(row?.level))
    const mul = Number(row?.ability_multiplier)
    if (!Number.isFinite(lv) || !Number.isFinite(mul)) continue
    pointsMap.set(lv, mul)
  }
  const points = Array.from(pointsMap.entries()).sort((a, b) => a[0] - b[0])
  if (!points.length) return 1
  const target = Math.max(0, Math.floor(level))
  if (target <= points[0]![0]) return points[0]![1]
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1]!
    const cur = points[i]!
    if (target > cur[0]) continue
    if (cur[0] === prev[0]) return cur[1]
    const ratio = (target - prev[0]) / (cur[0] - prev[0])
    return prev[1] + (cur[1] - prev[1]) * ratio
  }
  return points[points.length - 1]![1]
}

const buildDisplayAttributePreview = (itemDef: any) => {
  if (!itemDef || (!itemDef.equipment && !itemDef.tool)) return [] as Array<{ key: string; value: number }>
  const details = itemDef.equipment ? itemDef.equipment_details ?? {} : itemDef.tool_details ?? {}
  const basic = itemDef.equipment ? details.equipment_basic_data ?? {} : details.tool_basic_data ?? {}
  const upgrade = itemDef.equipment ? details.equipment_upgrade_data ?? {} : details.tool_upgrade_data ?? {}
  const ability = resolveItemAbilityMultiplierForDisplay(itemDef, 0)
  const keys = new Set<string>([...Object.keys(basic), ...Object.keys(upgrade)])
  const out: Array<{ key: string; value: number }> = []
  for (const key of keys) {
    const baseVal = Number(basic[key] ?? 0)
    const upVal = Number(upgrade[key] ?? 0)
    if (!Number.isFinite(baseVal) && !Number.isFinite(upVal)) continue
    const value = (Number.isFinite(baseVal) ? baseVal : 0) + (Number.isFinite(upVal) ? upVal : 0) * ability
    if (Math.abs(value) < 1e-12) continue
    out.push({ key, value })
  }
  return out.sort((a, b) => a.key.localeCompare(b.key))
}

const onInventoryItemMouseEnter = (
  item: { id: string; name: string; quantity: number },
  panel: { classification: string; title: string },
  event: MouseEvent,
) => {
  const itemDef = actionItemMap.value[item.id]
  hoveredInventoryItem.value = {
    id: item.id,
    name: item.name,
    classification: panel.classification,
    classification_title: panel.title,
    quantity: item.quantity,
    quantity_raw: Number(item.quantity || 0).toLocaleString('en-US'),
    attribute_preview: buildDisplayAttributePreview(itemDef),
  }
  inventoryTipPos.value = placeTooltip(event, 360, 320)
}

const onInventoryItemMouseMove = (event: MouseEvent) => {
  if (!hoveredInventoryItem.value) return
  inventoryTipPos.value = placeTooltip(event, 360, 320)
}

const onInventoryItemMouseLeave = () => {
  hoveredInventoryItem.value = null
}

const formatSeconds = (value: number) => {
  const safe = Number.isFinite(value) ? Math.max(0, value) : 0
  if (safe >= 10) return `${safe.toFixed(1)}s`
  if (safe >= 1) return `${safe.toFixed(2)}s`
  return `${safe.toFixed(3)}s`
}

const battleMapName = (mapId: string) =>
  maps.value.find((entry) => entry.id === mapId)?.name ?? mapId

const battleStatusText = computed(() => {
  const state = battleState.value
  if (!state || state.status === 'stopped') return ''
  return `战斗中…… [${battleMapName(state.map)} 第 ${Math.max(1, state.wave_number)} 波]`
})

const goBattlePage = () => {
  void router.push('/battle')
}

const formatPreviewNumber = (value: number) => {
  const safe = Number.isFinite(value) ? value : 0
  if (Math.abs(safe - Math.round(safe)) < 1e-9) return `${Math.round(safe)}`
  if (Math.abs(safe) >= 10) return safe.toFixed(1)
  return safe.toFixed(2)
}

const formatRate = (value: number) => `${(Math.max(0, Math.min(1, Number(value) || 0)) * 100).toFixed(2)}%`

const rewardPreviewLabel = (itemName: string, baseValue: number, effectiveValue: number) => {
  const base = formatPreviewNumber(baseValue)
  const effective = formatPreviewNumber(effectiveValue)
  if (Math.abs(baseValue - effectiveValue) < 1e-9) {
    return `产出 ${itemName} ${base}`
  }
  return `产出 ${itemName} ${base} -> ${effective}`
}

const loopTimeLabel = (event: GameplayEvent) => {
  const base = Number(event.loop_time ?? 0)
  if (!Number.isFinite(base) || base <= 0) return ''
  const effectiveRaw = Number(event.effective_loop_time ?? base)
  const effective = Number.isFinite(effectiveRaw) && effectiveRaw > 0 ? effectiveRaw : base
  if (Math.abs(base - effective) < 1e-6) {
    return `耗时 ${formatSeconds(base)}`
  }
  return `耗时 ${formatSeconds(base)} -> ${formatSeconds(effective)}`
}

const canEquipEntryToSlot = (
  entry: (typeof equipmentView.value.equipable_items)[number],
  slot: SlotCell,
) => {
  if (entry.quantity <= 0) return false

  const anchorToReplace = slot.item_id ? slot.anchor_slot || slot.slot_id : null
  const cells = slot.slot_type === 'tool' ? productionSlots.value : battleSlots.value

  for (const requiredBase of entry.required_slots) {
    const candidates = cells.filter((cell) => slotBase(cell.slot_id) === requiredBase)
    if (!candidates.length) return false

    const freeCount = candidates.filter((cell) => {
      if (!cell.item_id) return true
      if (anchorToReplace && cell.anchor_slot === anchorToReplace) return true
      return false
    }).length
    if (freeCount <= 0) return false
  }

  return true
}

const openSlotPanel = (slot: SlotCell) => {
  selectedSlot.value = slot
  equipError.value = ''
}

const closeSlotPanel = () => {
  selectedSlot.value = null
  equipError.value = ''
}

const syncSelectedSlot = () => {
  if (!selectedSlot.value) return
  const list = selectedSlot.value.slot_type === 'tool' ? productionSlots.value : battleSlots.value
  const next = list.find((entry) => entry.slot_id === selectedSlot.value?.slot_id)
  if (!next) {
    selectedSlot.value = null
    return
  }
  selectedSlot.value = next
}

const syncHoveredEquipSlot = () => {
  if (!hoveredEquipSlot.value) return
  const list = hoveredEquipSlot.value.slot_type === 'tool' ? productionSlots.value : battleSlots.value
  const next = list.find((entry) => entry.slot_id === hoveredEquipSlot.value?.slot_id)
  if (!next || !next.item_id) {
    hoveredEquipSlot.value = null
    return
  }
  hoveredEquipSlot.value = next
}

const syncHoveredInventoryItem = () => {
  if (!hoveredInventoryItem.value) return
  const panel = itemPanels.value.find((entry) => entry.classification === hoveredInventoryItem.value?.classification)
  if (!panel) {
    hoveredInventoryItem.value = null
    return
  }
  const item = panel.items.find((entry) => entry.id === hoveredInventoryItem.value?.id)
  if (!item) {
    hoveredInventoryItem.value = null
    return
  }
  const itemDef = actionItemMap.value[item.id]
  hoveredInventoryItem.value = {
    id: item.id,
    name: item.name,
    classification: panel.classification,
    classification_title: panel.title,
    quantity: item.quantity,
    quantity_raw: Number(item.quantity || 0).toLocaleString('en-US'),
    attribute_preview: buildDisplayAttributePreview(itemDef),
  }
}

const clearEnhanceCastTimer = () => {
  if (enhanceCastTimer !== null) {
    window.clearInterval(enhanceCastTimer)
    enhanceCastTimer = null
  }
}

const syncSelectedEnhanceTarget = () => {
  if (!enhanceTargets.value.length) {
    selectedEnhanceKey.value = ''
    enhancePreview.value = null
    return
  }
  if (!selectedEnhanceKey.value || !enhanceTargets.value.some((entry) => entry.key === selectedEnhanceKey.value)) {
    selectedEnhanceKey.value = enhanceTargets.value[0]!.key
  }
}

const selectEnhanceTarget = (targetKey: string) => {
  if (selectedEnhanceKey.value === targetKey) return
  selectedEnhanceKey.value = targetKey
  enhanceResultText.value = ''
}

const fetchEnhancePreview = async () => {
  const target = selectedEnhanceTarget.value
  if (!target || !showEnhancingPanel.value || !enhancingAction.value) {
    enhancePreview.value = null
    return
  }

  const reqSeq = ++enhancePreviewReqSeq
  enhanceLoading.value = true
  try {
    const data = await actions.enhancePreview(target.slot_type, target.anchor_slot)
    if (reqSeq !== enhancePreviewReqSeq) return
    enhancePreview.value = (data as EnhancePreview) ?? null
  } catch (e: any) {
    if (reqSeq !== enhancePreviewReqSeq) return
    enhancePreview.value = null
    store.actionError = e.message || '强化预览请求失败'
  } finally {
    if (reqSeq === enhancePreviewReqSeq) {
      enhanceLoading.value = false
    }
  }
}

const playEnhanceCast = (seconds: number) =>
  new Promise<void>((resolve) => {
    clearEnhanceCastTimer()
    const durationMs = Math.max(200, Math.floor((Number(seconds) || 5) * 1000))
    const start = Date.now()
    enhanceCasting.value = true
    enhanceProgress.value = 0
    enhanceCastTimer = window.setInterval(() => {
      const elapsed = Date.now() - start
      const progress = Math.max(0, Math.min(1, elapsed / durationMs))
      enhanceProgress.value = progress
      if (progress >= 1) {
        clearEnhanceCastTimer()
        resolve()
      }
    }, 50)
  })

const executeEnhance = async () => {
  const target = selectedEnhanceTarget.value
  if (!target || isEnhanceDisabled.value || !enhancePreview.value) return

  enhanceResultText.value = ''
  store.actionError = ''
  try {
    await playEnhanceCast(enhancePreview.value.cast_seconds)
    const data = await actions.enhanceExecute(target.slot_type, target.anchor_slot)
    const result = (data || {}) as {
      success?: boolean
      exp_gain?: number
      preview?: EnhancePreview
      patch?: Record<string, unknown>
    }

    if (result.patch) {
      store.applyDelta(result.patch)
    }
    if (result.preview) {
      enhancePreview.value = result.preview
    } else {
      await fetchEnhancePreview()
    }

    const statusText = result.success ? '强化成功' : '强化失败'
    const expText = Number.isFinite(Number(result.exp_gain))
      ? `，获得赋能经验 ${formatPreviewNumber(Number(result.exp_gain))}`
      : ''
    enhanceResultText.value = `${statusText}${expText}`
  } catch (e: any) {
    store.actionError = e.message || '强化失败'
  } finally {
    clearEnhanceCastTimer()
    enhanceCasting.value = false
    enhanceProgress.value = 0
  }
}

const toggleBattle = async (entry: BattleListItem) => {
  if (battleActionLoading.value) return
  battleActionLoading.value = true
  store.clearActionError()
  try {
    if (store.activeBattleId === entry.id && store.battleRunning) {
      await store.stopBattle()
      syncLoopFromServer()
      return
    }
    await store.startBattle(entry.id)
    syncLoopFromServer()
  } catch (e: any) {
    store.actionError = e.message || '请求失败'
  } finally {
    battleActionLoading.value = false
  }
}

const equipToSelected = async (itemId: string) => {
  const slot = selectedSlot.value
  if (!slot) return
  equipError.value = ''
  try {
    await store.equipItem(itemId, slot.slot_id)
  } catch (e: any) {
    equipError.value = e.message || '请求失败'
  }
}

const unequipFromSelected = async () => {
  const slot = selectedSlot.value
  if (!slot) return
  equipError.value = ''
  try {
    await store.unequipItem(slot.slot_id)
  } catch (e: any) {
    equipError.value = e.message || '请求失败'
  }
}

const clearLoopTimer = () => {
  if (loopTimer !== null) {
    window.clearInterval(loopTimer)
    loopTimer = null
  }
}

const BATTLE_SYNC_INTERVAL_MS = 5000
let battleSyncTimer: ReturnType<typeof setInterval> | null = null

const clearBattleSyncTimer = () => {
  if (battleSyncTimer !== null) {
    window.clearInterval(battleSyncTimer)
    battleSyncTimer = null
  }
}

const startBattleSyncTimer = () => {
  clearBattleSyncTimer()
  battleSyncTimer = window.setInterval(() => {
    if (!store.battleRunning) return
    void store.syncBattleState()
  }, BATTLE_SYNC_INTERVAL_MS)
}

const clearLoopLocal = () => {
  activeLoopEventId.value = ''
  activeLoopAvailableIterations.value = null
  loopProgress.value = 0
  loopTickInFlight.value = false
  clearLoopTimer()
}

const syncLoopFromServer = () => {
  const runtime = activeLoop.value
  if (!runtime) {
    clearLoopLocal()
    return
  }

  const activeEvent = loopEvents.value.find((entry) => entry.id === runtime.event_id)
  if (!activeEvent) {
    clearLoopLocal()
    return
  }

  const duration = Math.max(0.2, Number(runtime.duration_seconds) || Number(activeEvent.loop_time) || 1)
  const elapsed = Math.max(0, Number(runtime.elapsed_seconds) || 0)
  const isSameEvent = activeLoopEventId.value === runtime.event_id
  activeLoopEventId.value = runtime.event_id
  activeLoopAvailableIterations.value =
    typeof runtime.available_iterations === 'number'
      ? Math.max(0, Math.floor(runtime.available_iterations))
      : null
  loopDurationSeconds.value = duration

  if (loopStartedAtMs.value > 0 && isSameEvent) {
    const localElapsed = (Date.now() - loopStartedAtMs.value) / 1000
    if (elapsed > localElapsed + 0.5) {
      loopProgress.value = Math.max(0, Math.min(1, elapsed / duration))
      loopStartedAtMs.value = Date.now() - elapsed * 1000 + LOOP_CLIENT_LATENCY_MS
    }
  } else {
    loopProgress.value = Math.max(0, Math.min(1, elapsed / duration))
    loopStartedAtMs.value = Date.now() - elapsed * 1000 + LOOP_CLIENT_LATENCY_MS
  }
  if (activeLoopAvailableIterations.value !== null && activeLoopAvailableIterations.value <= 0) {
    clearLoopTimer()
    return
  }
  ensureLoopTimer()
}

const stopLoop = async () => {
  clearLoopLocal()
  try {
    await store.stopLoop()
  } catch (e: any) {
    store.actionError = e.message || '请求失败'
  }
}

const ensureLoopTimer = () => {
  if (loopTimer !== null) return
  loopTimer = window.setInterval(() => {
    const eventId = activeLoopEventId.value
    if (!eventId) {
      clearLoopLocal()
      return
    }

    const durationMs = Math.max(200, loopDurationSeconds.value * 1000)
    const elapsed = Date.now() - loopStartedAtMs.value
    const ratio = elapsed / durationMs
    loopProgress.value = Math.min(1, Math.max(0, ratio))

    if (ratio >= 1 && !loopTickInFlight.value) {
      void runLoopTick()
    }
  }, 50)
}

const runLoopTick = async () => {
  const eventId = activeLoopEventId.value
  if (!eventId || loopTickInFlight.value) return

  loopTickInFlight.value = true
  try {
    await actions.requestSync()
    loopStartedAtMs.value = Date.now() + LOOP_CLIENT_LATENCY_MS
    loopProgress.value = 0
  } catch (e: any) {
    store.actionError = e.message || '循环结算失败'
    clearLoopLocal()
  } finally {
    loopTickInFlight.value = false
  }
}

const startLoop = async (event: GameplayEvent, iterations?: number) => {
  store.clearActionError()
  try {
    await store.startLoop(event.id, iterations)
    activeLoopEventId.value = event.id
    loopDurationSeconds.value = Math.max(0.2, Number(event.effective_loop_time ?? event.loop_time ?? 1))
    loopProgress.value = 0
    loopStartedAtMs.value = Date.now() + LOOP_CLIENT_LATENCY_MS
    ensureLoopTimer()
  } catch (e: any) {
    store.actionError = e.message || '请求失败'
  }
}

const queueAppend = async (event: GameplayEvent, iterations?: number) => {
  store.clearActionError()
  try {
    await store.queueAppend(event.id, iterations)
  } catch (e: any) {
    store.actionError = e.message || '请求失败'
  }
}

const queueRemove = async (index: number) => {
  store.clearActionError()
  try {
    await store.queueRemove(index)
  } catch (e: any) {
    store.actionError = e.message || '请求失败'
  }
}

const queueSwap = async (fromIndex: number, toIndex: number) => {
  store.clearActionError()
  try {
    await store.queueSwap(fromIndex, toIndex)
  } catch (e: any) {
    store.actionError = e.message || '请求失败'
  }
}

const queueBringToFront = async (index: number) => {
  store.clearActionError()
  try {
    await store.queueBringToFront(index)
  } catch (e: any) {
    store.actionError = e.message || '请求失败'
  }
}

const isEventActionDisabled = (event: GameplayEvent) => {
  if (event.type === 'upgrade') {
    return !event.is_executable || loading.value
  }
  const isActive = activeLoopEventId.value === event.id
  if (isActive) return false
  return !event.is_executable || loading.value
}

const eventButtonLabel = (event: GameplayEvent) => {
  if (event.type === 'upgrade') {
    return '执行升级'
  }
  if (activeLoopEventId.value === event.id) {
    return loopTickInFlight.value ? '结算中...' : '循环中（点击停止）'
  }
  return '开始循环'
}

const displayEventName = (event: GameplayEvent) => {
  if (event.type !== 'upgrade') return event.name
  const maxExec = Number(event.max_executions ?? 1)
  if (!Number.isFinite(maxExec) || maxExec <= 1) return event.name
  const nextIndex = Math.max(1, Math.min(maxExec, Number(event.event_count ?? 0) + 1))
  return `${event.name}${nextIndex}`
}

const onEventClick = async (event: GameplayEvent) => {
  if (event.type === 'upgrade') {
    if (!event.is_executable) return
    store.clearActionError()
    try {
      await store.executeUpgrade(event.id)
    } catch (e: any) {
      store.actionError = e.message || '请求失败'
    }
    return
  }

  if (activeLoopEventId.value === event.id) {
    const iters = parseInt(loopIterationsInput.value || '0', 10) || undefined
    if (iters != null) {
      await stopLoop()
      await startLoop(event, iters)
    } else {
      await stopLoop()
    }
    return
  }
  if (!event.is_executable) {
    store.actionError = '当前行动条件不足，无法开始循环'
    return
  }
  const iters = parseInt(loopIterationsInput.value || '0', 10) || undefined
  await startLoop(event, iters)
}

watch(
  () => data.value,
  (next) => {
    if (!next) return

    const validMenuIds = new Set([
      ...productionTabs.value.map((entry) => entry.id),
      ...combatTabs.value.map((entry) => entry.id),
      BATTLE_TAB_ID,
      MARKET_TAB_ID,
      PROFILE_TAB_ID,
    ])
    if (!selectedMenuId.value || !validMenuIds.has(selectedMenuId.value)) {
      selectedMenuId.value = productionTabs.value[0]?.id ?? PROFILE_TAB_ID
    }

    if (!selectedMapId.value || !sceneTabs.value.some((entry) => entry.id === selectedMapId.value)) {
      selectedMapId.value = sceneTabs.value[0]?.id ?? ''
    }
    const validBattleMaps = new Set(battleEntries.value.map((e) => e.map))
    if (!selectedBattleMapId.value || !validBattleMaps.has(selectedBattleMapId.value)) {
      selectedBattleMapId.value = battleEntries.value[0]?.map ?? ''
    }

    syncLoopFromServer()
    syncSelectedSlot()
    syncSelectedEnhanceTarget()
    syncHoveredEquipSlot()
    syncHoveredInventoryItem()
  },
  { immediate: true },
)

watch(
  () => [showEnhancingPanel.value, selectedEnhanceKey.value, selectedMapId.value, enhancingAction.value?.id],
  ([show]) => {
    if (!show) {
      enhancePreview.value = null
      enhanceResultText.value = ''
      return
    }
    syncSelectedEnhanceTarget()
    if (!selectedEnhanceTarget.value || !enhancingAction.value) {
      enhancePreview.value = null
      return
    }
    void fetchEnhancePreview()
  },
  { immediate: true },
)

watch(
  () => sceneTabs.value.map((entry) => entry.id).join('|'),
  () => {
    if (!sceneTabs.value.length) {
      selectedMapId.value = ''
      return
    }
    if (!sceneTabs.value.some((entry) => entry.id === selectedMapId.value)) {
      selectedMapId.value = sceneTabs.value[0]!.id
    }
  },
  { immediate: true },
)

watch(
  () => enhanceTargets.value.map((entry) => `${entry.key}:${entry.enhance_level}:${entry.enhance_fail_count}`).join('|'),
  () => {
    if (!showEnhancingPanel.value) return
    syncSelectedEnhanceTarget()
    if (!selectedEnhanceTarget.value || !enhancingAction.value) return
    void fetchEnhancePreview()
  },
)

watch(
  () =>
    data.value
      ? data.value.inventory
          .slice()
          .sort((a, b) => `${a.id}:${a.state ?? 0}`.localeCompare(`${b.id}:${b.state ?? 0}`))
          .map((e) => `${e.id}:${e.state ?? 0}:${e.qty}`)
          .join('|')
      : '',
  () => {
    if (!showEnhancingPanel.value) return
    if (!selectedEnhanceTarget.value || !enhancingAction.value) return
    void fetchEnhancePreview()
  },
)

watch(
  () => showBattlePanel.value,
  (show) => {
    if (!show) return
    void store.fetchBattleList()
  },
)

watch(
  () => battleRunning.value,
  (running) => {
    if (running) {
      startBattleSyncTimer()
    } else {
      clearBattleSyncTimer()
    }
  },
)

onMounted(() => {
  fetchActionsData().catch((e) => console.warn('fetchActionsData failed:', e))
  fetchData().then(() => {
    syncLoopFromServer()
  }).catch((e) => console.warn('fetchData failed:', e))
  store.fetchBattleList().catch((e) => console.warn('fetchBattleList failed:', e))
  store.syncBattleState().catch((e) => console.warn('syncBattleState failed:', e))
  store.initWsListeners()
})

onBeforeUnmount(() => {
  clearLoopTimer()
  clearBattleSyncTimer()
  clearEnhanceCastTimer()
  hoveredEquipSlot.value = null
  hoveredInventoryItem.value = null
  store.disposeWsListeners()
  disconnect()
})
</script>

<style scoped>
@font-face {
  font-family: 'TorusPro';
  src: url('../UI/TorusPro-Regular.ttf') format('truetype');
  font-display: swap;
}

@font-face {
  font-family: 'TorusPro-Bold';
  src: url('../UI/TorusPro-Bold.ttf') format('truetype');
  font-display: swap;
}

.main-page {
  height: 100vh;
  width: min(1480px, 100vw);
  max-width: 100vw;
  margin: 0 auto;
  box-sizing: border-box;
  padding: 28px;
  color: var(--text);
  background:
    radial-gradient(920px 480px at 12% -12%, color-mix(in srgb, var(--brand) 20%, transparent), transparent 58%),
    radial-gradient(920px 520px at 88% -18%, color-mix(in srgb, var(--brand-2) 20%, transparent), transparent 60%),
    linear-gradient(180deg, var(--bg), var(--bg-2));
  font-family:
    'TorusPro',
    'PingFang SC',
    'Microsoft YaHei',
    ui-sans-serif,
    system-ui,
    sans-serif;
  display: grid;
  grid-template-rows: auto 1fr;
  gap: 18px;
  overflow: hidden;
}

.panel {
  border: 1px solid color-mix(in srgb, var(--brand) 35%, var(--border));
  border-radius: 18px;
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  box-shadow: var(--shadow-sm);
}

.top-bar {
  min-height: 96px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 18px;
}

.title-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo {
  width: 60px;
  height: 60px;
  display: grid;
  place-items: center;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--brand-2) 40%, var(--border));
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
  font-size: 1.5rem;
}

.loop-progress-wrap {
  flex: 1;
  min-width: 220px;
  max-width: 460px;
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  align-items: center;
}

.loop-meta {
  grid-column: 1 / span 2;
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 0.82rem;
  color: var(--muted);
}

.loop-track {
  height: 12px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-2) 86%, transparent);
  overflow: hidden;
}

.loop-fill {
  height: 100%;
  background: linear-gradient(
    90deg,
    color-mix(in srgb, var(--brand) 82%, #fff),
    color-mix(in srgb, var(--brand-2) 80%, #fff)
  );
  transition: width 0.05s linear;
}

.loop-stop {
  height: 24px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-2) 84%, transparent);
  color: var(--text);
  padding: 0 10px;
  font-size: 0.74rem;
  font-weight: 700;
  cursor: pointer;
}

.battle-chip {
  min-width: 210px;
  max-width: 320px;
  min-height: 52px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--brand) 45%, var(--border));
  background: color-mix(in srgb, var(--surface-2) 88%, transparent);
  color: var(--text);
  font-weight: 800;
  padding: 8px 10px;
  cursor: pointer;
  text-align: left;
  line-height: 1.35;
}

h1 {
  margin: 0;
  font-size: 2rem;
  letter-spacing: 0.5px;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.title-wrap p {
  margin: 2px 0 0;
  color: var(--muted);
  font-size: 0.88rem;
}

.actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.btn {
  height: 36px;
  border-radius: 999px;
  padding: 0 14px;
  border: 1px solid var(--border);
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  color: var(--text);
  font-weight: 800;
  cursor: pointer;
  background: color-mix(in srgb, var(--surface-2) 84%, transparent);
}

.btn.primary {
  border-color: rgba(255, 255, 255, 0.12);
  color: rgba(255, 255, 255, 0.96);
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--brand) 82%, #000),
    color-mix(in srgb, var(--brand-2) 78%, #000)
  );
}

.btn.ghost {
  background: color-mix(in srgb, var(--surface-2) 84%, transparent);
}

.btn:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.theme-switch {
  border: none;
  border-radius: 999px;
  width: 54px;
  height: 34px;
  padding: 0;
  background: var(--head-btn-bg);
  color: var(--text);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
}

.theme-switch.is-dark {
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--purple) 62%, var(--head-btn-bg)),
    color-mix(in srgb, var(--brand) 54%, var(--head-btn-bg))
  );
}

.thumb {
  width: 28px;
  height: 28px;
  border-radius: 999px;
  margin-left: 3px;
  background: color-mix(in srgb, var(--surface) 88%, transparent);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transform: translateX(0px);
  transition: transform 220ms cubic-bezier(0.2, 0.85, 0.15, 1);
}

.theme-switch.is-dark .thumb {
  transform: translateX(20px);
}

.theme-icon {
  width: 18px;
  height: 18px;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.layout {
  display: grid;
  grid-template-columns: minmax(170px, 210px) minmax(540px, 1fr) minmax(240px, 300px);
  gap: 16px;
  min-height: 0;
  overflow: hidden;
}

.left,
.center,
.right {
  padding: 14px;
  min-height: 0;
  min-width: 0;
}

.left {
  display: flex;
  flex-direction: column;
}

.left-scroll {
  min-height: 0;
  overflow: auto;
  display: grid;
  gap: 8px;
  padding-right: 4px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.collapse-btn {
  min-height: 26px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-2) 84%, transparent);
  color: var(--muted);
  font-size: 0.75rem;
  font-weight: 800;
  padding: 0 10px;
  cursor: pointer;
}

.collapse-btn:hover {
  color: var(--text);
}

h2 {
  margin: 0;
  font-size: 1.1rem;
  letter-spacing: 0.3px;
}

.battle-level-text {
  margin-top: 2px;
  font-size: 0.78rem;
  color: var(--muted);
  font-weight: 700;
  line-height: 1.2;
}

.battle-title {
  display: inline-grid;
  gap: 2px;
}

h3 {
  margin: 0;
  font-size: 1rem;
  letter-spacing: 0.2px;
}

.skills-scroll {
  display: grid;
  gap: 8px;
}

.skill-row {
  border: 1px solid var(--border);
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface-2) 82%, transparent);
  color: var(--text);
  height: 62px;
  padding: 0 10px;
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  position: relative;
}

.skill-row.active {
  border-color: color-mix(in srgb, var(--brand) 42%, var(--border));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--brand-2) 32%, transparent);
}

.skill-title-wrap {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.skill-name {
  font-size: 1.05rem;
  font-weight: 800;
  text-align: left;
}

.section-title {
  margin-top: 4px;
}

.skill-sub {
  font-size: 0.78rem;
  color: var(--muted);
  text-align: left;
}

.tip-exp-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.74rem;
  color: var(--muted);
}

.tip-track {
  height: 8px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-2) 86%, transparent);
  overflow: hidden;
}

.tip-fill {
  height: 100%;
  background: linear-gradient(90deg, #80ff80, #4040ff);
}

.skill-float-tip p {
  margin: 0;
  text-align: left;
  font-size: 0.74rem;
  color: var(--muted);
  line-height: 1.35;
}

.tip-green {
  color: #00ff00;
  font-weight: 700;
}

.skill-ring {
  --p: 0;
  --ring-track: color-mix(in srgb, var(--surface-3) 90%, transparent);
  --ring-start: rgb(128 255 128 / calc(1 - var(--p)));
  --ring-end: rgb(64 64 255 / 1);
  width: 42px;
  height: 42px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  position: relative;
  background: conic-gradient(
    from 0deg,
    var(--ring-start) 0turn,
    var(--ring-end) calc(var(--p) * 1turn),
    var(--ring-track) calc(var(--p) * 1turn),
    var(--ring-track) 1turn
  );
}

.skill-ring.upgrade-ring {
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--warning) 64%, var(--surface-2)),
    color-mix(in srgb, var(--brand-2) 44%, var(--surface-2))
  );
}

.ring-cap {
  position: absolute;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  top: 50%;
  left: 50%;
  pointer-events: none;
}

.ring-cap-start {
  opacity: 0;
  background: var(--ring-start);
  transform: translate(-50%, -50%) rotate(0deg) translateY(-18px);
}

.ring-cap-end {
  background: var(--ring-end);
  transform: translate(-50%, -50%) rotate(calc(var(--p) * 1turn)) translateY(-18px);
}

.ring-inner {
  width: 31px;
  height: 31px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--surface) 94%, transparent);
  display: grid;
  place-items: center;
  font-size: 1.0rem;
  font-weight: 800;
  font-family: "TorusPro-Bold";
}

.center {
  display: grid;
  grid-template-columns: minmax(140px, 180px) 1fr;
  gap: 12px;
}

.center.profile-mode {
  grid-template-columns: 1fr;
}

.center.market-mode {
  grid-template-columns: 1fr;
}

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

.attr-list {
  display: grid;
  gap: 6px;
}

.attr-row {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 7px 9px;
  background: color-mix(in srgb, var(--surface) 90%, transparent);
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.attr-key {
  color: var(--muted);
  font-size: 0.82rem;
}

.attr-val {
  font-size: 0.82rem;
  font-weight: 700;
}

.scene-col,
.loop-col {
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.scene-btn {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
  color: var(--text);
  font-weight: 700;
  min-height: 40px;
  cursor: pointer;
}

.scene-btn.active {
  border-color: color-mix(in srgb, var(--brand) 44%, var(--border));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--brand-2) 30%, transparent);
}

.events-list {
  overflow: auto;
  min-height: 0;
  display: grid;
  gap: 8px;
}

.event-card {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px;
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
}

.event-card.blocked {
  border-color: color-mix(in srgb, var(--warning) 30%, var(--border));
}

.event-head {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
}

.event-card p {
  margin: 8px 0 0;
  color: var(--muted);
  line-height: 1.45;
  font-size: 0.9rem;
}

.tag {
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 0.72rem;
  border: 1px solid color-mix(in srgb, var(--success) 45%, var(--border));
  color: var(--success);
}

.tag.blocked {
  border-color: color-mix(in srgb, var(--warning) 45%, var(--border));
  color: var(--warning);
}

.event-meta,
.event-req {
  margin-top: 8px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.event-meta span,
.event-req span {
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 0.76rem;
  color: var(--muted);
  border: 1px solid var(--border);
}

.event-action {
  margin-top: 10px;
  min-height: 34px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  padding: 0 12px;
  color: rgba(255, 255, 255, 0.95);
  font-weight: 800;
  cursor: pointer;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--brand) 82%, #000),
    color-mix(in srgb, var(--brand-2) 78%, #000)
  );
}

.event-actions {
  margin-top: 10px;
  display: flex;
  gap: 8px;
}

.event-action.ghost {
  background: color-mix(in srgb, var(--surface-2) 86%, transparent);
  color: var(--text);
  border-color: var(--border);
}

.event-action:disabled {
  background: color-mix(in srgb, var(--surface-3) 86%, transparent);
  color: var(--muted);
  border-color: var(--border);
  cursor: not-allowed;
}

.event-actions-row {
  margin-top: 10px;
  display: flex;
  gap: 8px;
  align-items: center;
}

.enhance-panel {
  min-height: 0;
  overflow: auto;
  display: grid;
  gap: 8px;
}

.enhance-block {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px;
  background: color-mix(in srgb, var(--surface-2) 82%, transparent);
  display: grid;
  gap: 8px;
}

.enhance-desc {
  margin: 0;
  color: var(--muted);
  font-size: 0.84rem;
  line-height: 1.4;
}

.enhance-target-cell.active {
  border-color: color-mix(in srgb, var(--brand) 48%, var(--border));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--brand-2) 28%, transparent);
}

.enhance-cast-wrap {
  display: grid;
  gap: 5px;
}

.enhance-cast-text {
  font-size: 0.78rem;
  color: var(--muted);
}

.enhance-result {
  margin: 0;
  color: var(--success);
  font-size: 0.84rem;
  font-weight: 700;
}

.iterations-input {
  width: 52px;
  padding: 4px 6px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
  color: var(--text);
  font-size: 0.8rem;
  text-align: center;
}

.iterations-input::placeholder {
  color: var(--muted);
}

.queue-remaining {
  font-size: 0.7rem;
  color: var(--muted);
  padding: 1px 6px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--surface-3) 60%, transparent);
}

.event-action.secondary {
  background: color-mix(in srgb, var(--surface-2) 86%, transparent);
  color: var(--text);
  border-color: var(--border);
  font-weight: 600;
  font-size: 0.8rem;
  padding: 0 10px;
}

.queue-bar {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 12px;
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
  border: 1px solid var(--border);
  border-radius: 10px;
  margin: 0 10px 10px;
}

.queue-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  background: transparent;
  border: none;
  color: var(--text);
  font-weight: 700;
  cursor: pointer;
  padding: 0;
}

.queue-toggle-arrow {
  font-size: 0.7rem;
  color: var(--muted);
}

.queue-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.queue-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 0.82rem;
}

.queue-item.current {
  background: color-mix(in srgb, var(--brand) 18%, transparent);
  border: 1px solid color-mix(in srgb, var(--brand) 40%, transparent);
}

.queue-item.past {
  opacity: 0.5;
}

.queue-index {
  min-width: 20px;
  text-align: center;
  color: var(--muted);
  font-weight: 700;
}

.queue-name {
  flex: 1;
}

.queue-badge {
  font-size: 0.65rem;
  padding: 1px 6px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--brand) 30%, transparent);
  color: var(--text);
}

.queue-actions {
  display: flex;
  gap: 4px;
}

.queue-btn {
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-3) 80%, transparent);
  color: var(--text);
  font-size: 0.75rem;
  cursor: pointer;
  padding: 0;
}

.queue-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.queue-btn.danger {
  color: #ff6b6b;
}

.profile-menu-btn {
  margin-top: 6px;
  min-height: 40px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-2) 82%, transparent);
  color: var(--text);
  font-weight: 800;
  cursor: pointer;
}

.profile-menu-btn.active {
  border-color: color-mix(in srgb, var(--brand) 46%, var(--border));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--brand-2) 30%, transparent);
}

.battle-module-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.market-module-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.right {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.right-tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.right-tab-btn {
  min-height: 34px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: color-mix(in srgb, var(--surface-2) 82%, transparent);
  color: var(--text);
  font-weight: 700;
  cursor: pointer;
}

.right-tab-btn.active {
  border-color: color-mix(in srgb, var(--brand) 45%, var(--border));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--brand-2) 28%, transparent);
}

.item-panels {
  min-height: 0;
  overflow: auto;
  display: grid;
  gap: 8px;
}

.item-panel {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px;
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
}

.item-panel h3 {
  margin-top: 0;
}

.square-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.square-cell {
  aspect-ratio: 1 / 1;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: color-mix(in srgb, var(--surface-2) 84%, transparent);
  color: var(--text);
  display: grid;
  place-items: center;
  text-align: center;
  padding: 8px 6px;
  gap: 2px;
  cursor: pointer;
  position: relative;
}

.square-cell.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.item-cell {
  cursor: default;
}

.cell-icon-img {
  width: 32px;
  height: 32px;
  object-fit: contain;
  pointer-events: none;
}

.cell-name {
  font-size: 0.8rem;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.cell-qty {
  font-size: 0.72rem;
  color: var(--brand);
  font-weight: 700;
}

.cell-icon {
  font-size: 1rem;
}

.cell-plus-badge {
  position: absolute;
  right: 5px;
  bottom: 5px;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--brand) 38%, var(--border));
  background: color-mix(in srgb, var(--surface) 95%, transparent);
  color: var(--brand);
  font-weight: 800;
  font-size: 0.68rem;
  padding: 1px 5px;
  line-height: 1.1;
}

.equipment-pane {
  min-height: 0;
  overflow: auto;
  display: grid;
  gap: 10px;
}

.equip-slot-cell.active {
  border-color: color-mix(in srgb, var(--brand) 50%, var(--border));
}

.slot-picker {
  display: grid;
  gap: 8px;
}

.picker-action {
  font-weight: 800;
}

.skill-float-tip {
  position: fixed;
  width: min(320px, calc(100vw - 40px));
  border: 1px solid color-mix(in srgb, var(--brand-2) 42%, var(--border));
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface) 98%, transparent);
  box-shadow: var(--shadow-sm);
  padding: 8px 10px;
  display: grid;
  gap: 6px;
  z-index: 99999;
  pointer-events: none;
}

.equip-float-tip {
  position: fixed;
  width: min(360px, calc(100vw - 40px));
  border: 1px solid color-mix(in srgb, var(--brand) 40%, var(--border));
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface) 98%, transparent);
  box-shadow: var(--shadow-sm);
  padding: 8px 10px;
  display: grid;
  gap: 6px;
  z-index: 100000;
  pointer-events: none;
}

.item-float-tip {
  position: fixed;
  width: min(360px, calc(100vw - 40px));
  border: 1px solid color-mix(in srgb, var(--brand-2) 40%, var(--border));
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface) 98%, transparent);
  box-shadow: var(--shadow-sm);
  padding: 8px 10px;
  display: grid;
  gap: 6px;
  z-index: 100001;
  pointer-events: none;
}

.item-float-tip h4 {
  margin: 0;
  font-size: 0.9rem;
  overflow-wrap: anywhere;
}

.item-float-tip p {
  margin: 0;
  font-size: 0.78rem;
  color: var(--muted);
}

.equip-float-tip h4 {
  margin: 0;
  font-size: 0.9rem;
}

.equip-float-tip p {
  margin: 0;
  font-size: 0.78rem;
  color: var(--muted);
}

.equip-attrs {
  display: grid;
  gap: 4px;
  max-height: 260px;
  overflow: auto;
}

.equip-attr-row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 4px 6px;
  font-size: 0.76rem;
}

.equip-attr-row strong {
  color: #80ff80;
}

.error {
  color: var(--danger);
  font-weight: 800;
  margin: 0;
}

.empty {
  color: var(--muted);
  border: 1px dashed var(--border);
  border-radius: 10px;
  padding: 10px;
}

@media (max-width: 1280px) {
  .layout {
    grid-template-columns: 1fr;
    min-height: auto;
  }

  .center {
    grid-template-columns: 1fr;
  }

  .production-attr-grid {
    grid-template-columns: 1fr;
  }

  .battle-attr-grid {
    grid-template-columns: 1fr;
  }

  .scene-col {
    flex-direction: row;
    flex-wrap: wrap;
  }

  .scene-col h2 {
    width: 100%;
  }

  .top-bar {
    flex-wrap: wrap;
  }

  .loop-progress-wrap {
    order: 3;
    width: 100%;
    max-width: none;
  }
}

@media (max-width: 780px) {
  .main-page {
    height: auto;
    min-height: 100vh;
    overflow: visible;
    width: 100%;
    max-width: 100%;
    padding: 16px;
    display: block;
  }

  .top-bar {
    flex-direction: column;
    align-items: flex-start;
  }

  .actions {
    width: 100%;
  }

  .loop-progress-wrap {
    width: 100%;
  }
}

.skip-time-fab-wrap {
  position: fixed;
  right: 20px;
  bottom: 20px;
  z-index: 100;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.skip-time-fab {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  border: none;
  background: var(--primary, #3b82f6);
  color: #fff;
  font-size: 20px;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.2s, background 0.2s;
}

.skip-time-fab:hover {
  transform: scale(1.05);
  background: var(--primary-hover, #2563eb);
}

.skip-time-panel {
  width: 240px;
  background: var(--panel-bg, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  overflow: hidden;
}

.skip-time-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-color, #e5e7eb);
  font-size: 14px;
}

.skip-time-header .btn.icon {
  width: 28px;
  height: 28px;
  padding: 0;
  border-radius: 6px;
  background: transparent;
  border: none;
  color: var(--text-muted, #6b7280);
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
}

.skip-time-header .btn.icon:hover {
  background: var(--hover-bg, #f3f4f6);
  color: var(--text, #111827);
}

.skip-time-body {
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.skip-time-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text, #374151);
}

.skip-time-input {
  flex: 1;
  min-width: 0;
  padding: 6px 10px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg, #fff);
  color: var(--text, #111827);
}

.skip-time-unit {
  color: var(--text-muted, #6b7280);
  font-size: 13px;
  white-space: nowrap;
}

.skip-time-btn {
  width: 100%;
  padding: 8px 12px;
  font-size: 14px;
}

.dark .skip-time-panel {
  background: var(--panel-bg-dark, #1f2937);
  border-color: var(--border-color-dark, #374151);
}

.dark .skip-time-header {
  border-color: var(--border-color-dark, #374151);
}

.dark .skip-time-header .btn.icon {
  color: var(--text-muted-dark, #9ca3af);
}

.dark .skip-time-header .btn.icon:hover {
  background: var(--hover-bg-dark, #374151);
  color: var(--text-dark, #f9fafb);
}

.dark .skip-time-label {
  color: var(--text-dark, #d1d5db);
}

.dark .skip-time-input {
  background: #111827;
  color: #f9fafb;
  border-color: #374151;
}

.dark .skip-time-unit {
  color: var(--text-muted-dark, #9ca3af);
}
</style>
