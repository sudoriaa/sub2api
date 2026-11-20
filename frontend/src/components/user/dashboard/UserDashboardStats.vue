<template>
  <!-- Row 1: Core Stats -->
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
    <!-- Balance -->
    <div v-if="!isSimple" class="card card-hover p-4">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-medium text-[#929cb3] dark:text-dark-400">{{ t('dashboard.balance') }}</p>
          <p class="tabular-nums mt-1.5 truncate text-2xl font-bold tracking-tight text-primary-700 dark:text-[#ff9bbd]">${{ formatBalance(balance) }}</p>
        </div>
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-[#2e1d27] dark:text-[#ff9bbd]">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" />
          </svg>
        </div>
      </div>
      <div class="mt-3 flex items-center justify-between border-t border-[#e6edf5] pt-2.5 text-xs dark:border-dark-700">
        <span class="badge badge-primary">{{ t('common.available') }}</span>
      </div>
    </div>

    <!-- API Keys -->
    <div class="card card-hover p-4">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-medium text-[#929cb3] dark:text-dark-400">{{ t('dashboard.apiKeys') }}</p>
          <p class="tabular-nums mt-1.5 text-2xl font-bold tracking-tight text-[#252b3b] dark:text-white">{{ stats?.total_api_keys || 0 }}</p>
        </div>
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-accent-50 text-accent-700 dark:bg-[#1a293a] dark:text-[#8ad0ff]">
          <Icon name="key" size="md" :stroke-width="2" />
        </div>
      </div>
      <div class="mt-3 flex items-center justify-between border-t border-[#e6edf5] pt-2.5 text-xs dark:border-dark-700">
        <span class="badge badge-success">{{ stats?.active_api_keys || 0 }} {{ t('common.active') }}</span>
      </div>
    </div>

    <!-- Today Requests -->
    <div class="card card-hover p-4">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-medium text-[#929cb3] dark:text-dark-400">{{ t('dashboard.todayRequests') }}</p>
          <p class="tabular-nums mt-1.5 text-2xl font-bold tracking-tight text-[#252b3b] dark:text-white">{{ stats?.today_requests || 0 }}</p>
        </div>
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-accent-50 text-accent-700 dark:bg-[#1a293a] dark:text-[#8ad0ff]">
          <Icon name="chart" size="md" :stroke-width="2" />
        </div>
      </div>
      <div class="mt-3 flex items-center justify-between border-t border-[#e6edf5] pt-2.5 text-xs dark:border-dark-700">
        <span class="text-[#929cb3] dark:text-dark-400">{{ t('common.total') }}</span>
        <span class="tabular-nums font-semibold text-[#5d677e] dark:text-dark-200">{{ formatNumber(stats?.total_requests || 0) }}</span>
      </div>
    </div>

    <!-- Today Cost -->
    <div class="card card-hover p-4">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-medium text-[#929cb3] dark:text-dark-400">{{ t('dashboard.todayCost') }}</p>
          <p class="tabular-nums mt-1.5 truncate text-2xl font-bold tracking-tight">
            <span class="text-accent-700 dark:text-[#8ad0ff]" :title="t('dashboard.actual')">${{ formatCost(stats?.today_actual_cost || 0) }}</span>
            <span class="ml-1 text-xs font-normal text-[#929cb3] dark:text-dark-400" :title="t('dashboard.standard')">/ ${{ formatCost(stats?.today_cost || 0) }}</span>
          </p>
        </div>
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-[#2e1d27] dark:text-[#ff9bbd]">
          <Icon name="dollar" size="md" :stroke-width="2" />
        </div>
      </div>
      <div class="mt-3 flex items-center justify-between gap-2 border-t border-[#e6edf5] pt-2.5 text-xs dark:border-dark-700">
        <span class="shrink-0 text-[#929cb3] dark:text-dark-400">{{ t('common.total') }}</span>
        <span class="tabular-nums truncate">
          <span class="font-semibold text-primary-700 dark:text-[#ff9bbd]" :title="t('dashboard.actual')">${{ formatCost(stats?.total_actual_cost || 0) }}</span>
          <span class="text-[#929cb3] dark:text-dark-400" :title="t('dashboard.standard')"> / ${{ formatCost(stats?.total_cost || 0) }}</span>
        </span>
      </div>
    </div>
  </div>

  <!-- Row 2: Token Stats & Performance -->
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
    <!-- Today Tokens -->
    <div class="card p-4">
      <div class="flex items-center justify-between gap-2">
        <span class="text-xs font-medium text-[#929cb3] dark:text-dark-400">{{ t('dashboard.todayTokens') }}</span>
        <span class="tabular-nums text-base font-bold text-[#252b3b] dark:text-white">{{ formatTokens(stats?.today_tokens || 0) }}</span>
      </div>
      <div class="mt-3 flex h-2 w-full gap-0.5 overflow-hidden rounded-full bg-[#f8fafd] dark:bg-dark-900">
        <div
          class="h-full bg-primary-500 transition-all"
          :style="{ width: tokenShare(stats?.today_input_tokens || 0, todayTokenTotal) }"
          :title="t('dashboard.input')"
        />
        <div
          class="h-full bg-accent-500 transition-all"
          :style="{ width: tokenShare(stats?.today_output_tokens || 0, todayTokenTotal) }"
          :title="t('dashboard.output')"
        />
        <div
          class="h-full bg-[#b7c6db] transition-all dark:bg-dark-500"
          :style="{ width: tokenShare((stats?.today_cache_creation_tokens || 0) + (stats?.today_cache_read_tokens || 0), todayTokenTotal) }"
          :title="t('dashboard.cache')"
        />
      </div>
      <div class="mt-2.5 flex flex-wrap items-center justify-between gap-x-2 gap-y-1 text-[11px] tabular-nums text-[#5d677e] dark:text-dark-300">
        <span class="inline-flex items-center gap-1"><i class="inline-block h-2 w-2 rounded-sm bg-primary-500" />{{ t('dashboard.input') }} {{ formatTokens(stats?.today_input_tokens || 0) }}</span>
        <span class="inline-flex items-center gap-1"><i class="inline-block h-2 w-2 rounded-sm bg-accent-500" />{{ t('dashboard.output') }} {{ formatTokens(stats?.today_output_tokens || 0) }}</span>
        <span class="inline-flex items-center gap-1"><i class="inline-block h-2 w-2 rounded-sm bg-[#b7c6db] dark:bg-dark-500" />{{ t('dashboard.cache') }} {{ formatTokens((stats?.today_cache_creation_tokens || 0) + (stats?.today_cache_read_tokens || 0)) }}</span>
      </div>
    </div>

    <!-- Total Tokens -->
    <div class="card p-4">
      <div class="flex items-center justify-between gap-2">
        <span class="text-xs font-medium text-[#929cb3] dark:text-dark-400">{{ t('dashboard.totalTokens') }}</span>
        <span class="tabular-nums text-base font-bold text-[#252b3b] dark:text-white">{{ formatTokens(stats?.total_tokens || 0) }}</span>
      </div>
      <div class="mt-3 flex h-2 w-full gap-0.5 overflow-hidden rounded-full bg-[#f8fafd] dark:bg-dark-900">
        <div
          class="h-full bg-accent-500 transition-all"
          :style="{ width: tokenShare(stats?.total_input_tokens || 0, allTokenTotal) }"
          :title="t('dashboard.input')"
        />
        <div
          class="h-full bg-primary-500 transition-all"
          :style="{ width: tokenShare(stats?.total_output_tokens || 0, allTokenTotal) }"
          :title="t('dashboard.output')"
        />
        <div
          class="h-full bg-[#b7c6db] transition-all dark:bg-dark-500"
          :style="{ width: tokenShare((stats?.total_cache_creation_tokens || 0) + (stats?.total_cache_read_tokens || 0), allTokenTotal) }"
          :title="t('dashboard.cache')"
        />
      </div>
      <div class="mt-2.5 flex flex-wrap items-center justify-between gap-x-2 gap-y-1 text-[11px] tabular-nums text-[#5d677e] dark:text-dark-300">
        <span class="inline-flex items-center gap-1"><i class="inline-block h-2 w-2 rounded-sm bg-accent-500" />{{ t('dashboard.input') }} {{ formatTokens(stats?.total_input_tokens || 0) }}</span>
        <span class="inline-flex items-center gap-1"><i class="inline-block h-2 w-2 rounded-sm bg-primary-500" />{{ t('dashboard.output') }} {{ formatTokens(stats?.total_output_tokens || 0) }}</span>
        <span class="inline-flex items-center gap-1"><i class="inline-block h-2 w-2 rounded-sm bg-[#b7c6db] dark:bg-dark-500" />{{ t('dashboard.cache') }} {{ formatTokens((stats?.total_cache_creation_tokens || 0) + (stats?.total_cache_read_tokens || 0)) }}</span>
      </div>
    </div>

    <!-- Performance (RPM/TPM) -->
    <div class="card p-4">
      <div class="mb-2 flex items-center justify-between">
        <span class="text-xs font-medium text-[#929cb3] dark:text-dark-400">{{ t('dashboard.performance') }}</span>
        <Icon name="bolt" size="sm" class="text-accent-500" :stroke-width="2" />
      </div>
      <div class="grid grid-cols-2 gap-2">
        <div class="rounded-lg border border-primary-200 bg-primary-50 px-2.5 py-1.5 dark:border-[#4d2c3e] dark:bg-[#2e1d27]">
          <div class="text-[10px] font-semibold text-primary-700 dark:text-[#ff9bbd]">RPM</div>
          <div class="tabular-nums text-sm font-bold text-[#252b3b] dark:text-white">{{ formatTokens(stats?.rpm || 0) }}</div>
        </div>
        <div class="rounded-lg border border-accent-200 bg-accent-50 px-2.5 py-1.5 dark:border-[#2a435e] dark:bg-[#1a293a]">
          <div class="text-[10px] font-semibold text-accent-700 dark:text-[#8ad0ff]">TPM</div>
          <div class="tabular-nums text-sm font-bold text-[#252b3b] dark:text-white">{{ formatTokens(stats?.tpm || 0) }}</div>
        </div>
      </div>
    </div>

    <!-- Avg Response Time -->
    <div class="card p-4">
      <div class="flex items-start justify-between gap-3">
        <div>
          <p class="text-xs font-medium text-[#929cb3] dark:text-dark-400">{{ t('dashboard.avgResponse') }}</p>
          <p class="tabular-nums mt-1.5 text-2xl font-bold tracking-tight text-[#252b3b] dark:text-white">{{ formatDuration(stats?.average_duration_ms || 0) }}</p>
          <p class="mt-1 text-xs text-[#929cb3] dark:text-dark-400">{{ t('dashboard.averageTime') }}</p>
        </div>
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-[#2e1d27] dark:text-[#ff9bbd]">
          <Icon name="clock" size="md" :stroke-width="2" />
        </div>
      </div>
    </div>
  </div>

  <!-- Row 3: Per-platform breakdown -->
  <div v-if="!isSimple && platformCards.length > 0" class="card p-5">
    <div class="mb-4 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <span class="h-4 w-1 rounded-full bg-primary-500" aria-hidden="true"></span>
        <h3 class="text-sm font-bold text-[#252b3b] dark:text-white">{{ t('dashboard.platformBreakdown') }}</h3>
        <span class="badge badge-sky">
          {{ t('dashboard.platformCount', { count: platformCount }) }}
        </span>
      </div>
    </div>
    <div class="grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-4">
      <div
        v-for="(item, idx) in platformCards"
        :key="item.platform"
        data-testid="platform-card"
        :data-platform="item.platform"
        :class="[
          'rounded-xl border p-3.5 transition-colors',
          item.isOther
            ? 'border-dashed border-[#d5e0ec] bg-[#f8fafd] dark:border-dark-600 dark:bg-dark-900/40'
            : idx % 2 === 0
              ? 'border-primary-200 bg-white dark:border-[#4d2c3e] dark:bg-dark-800'
              : 'border-accent-200 bg-white dark:border-[#2a435e] dark:bg-dark-800'
        ]"
      >
        <div class="flex items-center justify-between">
          <span class="flex items-center gap-2 text-sm font-bold text-[#252b3b] dark:text-white">
            <span
              class="h-2 w-2 rounded-full"
              :class="item.isOther ? 'bg-[#929cb3]' : idx % 2 === 0 ? 'bg-primary-500' : 'bg-accent-500'"
            />
            {{ item.isOther ? t('dashboard.platformOther') : platformLabel(item.platform) }}
          </span>
          <span
            class="tabular-nums font-mono text-sm font-bold"
            :class="idx % 2 === 0 ? 'text-primary-700 dark:text-[#ff9bbd]' : 'text-accent-700 dark:text-[#8ad0ff]'"
            :title="t('dashboard.actual')"
          >
            ${{ formatCost(item.total_actual_cost) }}
          </span>
        </div>
        <div class="mt-2.5 space-y-1 text-xs">
          <div class="flex items-center justify-between">
            <span class="text-[#929cb3] dark:text-dark-400">{{ t('dashboard.todayCost') }}</span>
            <span class="tabular-nums font-mono font-medium text-[#252b3b] dark:text-white">${{ formatCost(item.today_actual_cost) }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-[#929cb3] dark:text-dark-400">{{ t('dashboard.requests') }}</span>
            <span class="tabular-nums font-mono text-[#5d677e] dark:text-dark-300">
              {{ item.total_requests > 0 ? formatNumber(item.total_requests) : '-' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-[#929cb3] dark:text-dark-400">{{ t('dashboard.tokens') }}</span>
            <span class="tabular-nums font-mono text-[#5d677e] dark:text-dark-300">
              {{ item.total_tokens > 0 ? formatTokens(item.total_tokens) : '-' }}
            </span>
          </div>
        </div>

        <!-- Quota 区：仅当 quota 配置存在、非 __other__ 且至少有一个窗口配了 limit 时显示 -->
        <div v-if="hasAnyLimit(item.quota) && !item.isOther" class="mt-3 space-y-1.5 border-t border-[#e6edf5] pt-2.5 dark:border-dark-700">
          <p class="text-[10px] font-semibold uppercase tracking-wide text-[#929cb3]">
            {{ t('dashboard.platformQuota.title') }}
          </p>
          <template v-for="w in (['daily', 'weekly', 'monthly'] as const)" :key="w">
            <div v-if="quotaVal(item.quota, `${w}_limit_usd`) != null" class="space-y-1">
              <!-- limit=0：完全禁用 -->
              <template v-if="(quotaVal(item.quota, `${w}_limit_usd`) as number) === 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-[#5d677e] dark:text-dark-300">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                  <span class="font-mono text-rose-500">{{ t('dashboard.platformQuota.disabled') }}</span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-rose-100 dark:bg-dark-700">
                  <div class="h-full w-full rounded-full bg-rose-500" />
                </div>
              </template>
              <!-- limit>0：正常用量进度条 -->
              <template v-else>
                <div class="flex items-center justify-between text-xs">
                  <span class="text-[#5d677e] dark:text-dark-300">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                  <span class="tabular-nums font-mono text-[#252b3b] dark:text-dark-200">
                    ${{ formatUsd((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0) }} / ${{ formatUsd(quotaVal(item.quota, `${w}_limit_usd`) as number) }}
                  </span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-primary-50 dark:bg-dark-700">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="quotaBarClass(calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number))"
                    :style="{ width: calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number) + '%' }"
                  />
                </div>
                <p v-if="quotaVal(item.quota, `${w}_window_resets_at`)" class="text-[10px] text-[#929cb3]">
                  {{ t('dashboard.platformQuota.resetsAt', { time: formatResetTime(quotaVal(item.quota, `${w}_window_resets_at`) as string) }) }}
                </p>
              </template>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { PlatformDashboardStats, UserDashboardStats as UserStatsType } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'

interface FusedPlatformCard {
  platform: string
  total_actual_cost: number
  today_actual_cost: number
  total_requests: number
  total_tokens: number
  isOther?: boolean
  quota?: PlatformQuotaItem
}

const props = defineProps<{
  stats: UserStatsType
  balance: number
  isSimple: boolean
  platformQuotas?: PlatformQuotaItem[] | null
}>()
const { t } = useI18n()

const PLATFORM_LABELS: Record<string, string> = {
  anthropic: 'Claude',
  openai: 'OpenAI',
  gemini: 'Gemini',
  antigravity: 'Antigravity',
  grok: 'Grok',
  kimi: 'Kimi',
  zhipu: 'Zhipu GLM',
  deepseek: 'DeepSeek',
  minimax: 'MiniMax',
  typesafe: 'TypeSafe / Jev',
}

const platformLabel = (p: string) => PLATFORM_LABELS[p] ?? p

// 处理"各平台之和 < 总值"的差值：后端按平台聚合时过滤了无法归属平台的行
// （group 与 account 都缺 platform）。这里把差值作为"其他"卡片显式展示，
// 避免 Row 1 总值与 Row 3 平台拆分加总对不上、用户困惑。
const OTHER_THRESHOLD = 0.0001
const platformCards = computed<FusedPlatformCard[]>(() => {
  // 建立 by_platform Map
  const byPlat = new Map<string, PlatformDashboardStats>()
  for (const item of props.stats?.by_platform ?? []) byPlat.set(item.platform, item)

  // 建立 quota Map。三档全空的记录不产生卡片，挂到卡片上也不渲染配额区。
  const byQuota = new Map<string, PlatformQuotaItem>()
  for (const q of props.platformQuotas ?? []) byQuota.set(q.platform, q)

  // 卡片集合 = 有用量的平台 ∪ 至少配置了一档限额的平台。
  // 三档全空的限额记录等价于不限额，不单独产生卡片。
  // 后端 by_platform / quota 接口均不会返回 platform='__other__'，
  // 无需显式排除；__other__ 由下方差值补差逻辑单独追加。
  const platforms = new Set<string>(byPlat.keys())
  for (const [platform, q] of byQuota) {
    if (hasAnyLimit(q)) platforms.add(platform)
  }

  const PLATFORM_ORDER = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'typesafe']
  const cards: FusedPlatformCard[] = []

  for (const p of platforms) {
    const stat = byPlat.get(p)
    cards.push({
      platform: p,
      total_actual_cost: stat?.total_actual_cost ?? 0,
      today_actual_cost: stat?.today_actual_cost ?? 0,
      total_requests: stat?.total_requests ?? 0,
      total_tokens: stat?.total_tokens ?? 0,
      quota: byQuota.get(p),
    })
  }

  // 排序：按 PLATFORM_ORDER，未知平台按名称排序
  cards.sort((a, b) => {
    const ai = PLATFORM_ORDER.indexOf(a.platform)
    const bi = PLATFORM_ORDER.indexOf(b.platform)
    if (ai === -1 && bi === -1) return a.platform.localeCompare(b.platform)
    if (ai === -1) return 1
    if (bi === -1) return -1
    return ai - bi
  })

  // __other__ 补差逻辑：只对 by_platform 有 usage 数据的总和计算
  const total = props.stats?.total_actual_cost ?? 0
  const today = props.stats?.today_actual_cost ?? 0
  const sumTotal = cards.reduce((s, c) => s + c.total_actual_cost, 0)
  const sumToday = cards.reduce((s, c) => s + c.today_actual_cost, 0)
  const diffTotal = Math.max(0, total - sumTotal)
  const diffToday = Math.max(0, today - sumToday)

  if (diffTotal > OTHER_THRESHOLD || diffToday > OTHER_THRESHOLD) {
    cards.push({
      platform: '__other__',
      total_actual_cost: diffTotal,
      today_actual_cost: diffToday,
      total_requests: 0,
      total_tokens: 0,
      isOther: true,
    })
  }

  return cards
})

// 标题右侧的平台计数 = 实际渲染的平台卡片数，不含"其他"差额卡。
const platformCount = computed(() => platformCards.value.filter((c) => !c.isOther).length)

// Quota helpers

type QuotaWindow = 'daily' | 'weekly' | 'monthly'
type QuotaField = `${QuotaWindow}_limit_usd` | `${QuotaWindow}_usage_usd` | `${QuotaWindow}_window_resets_at`

function quotaVal(q: PlatformQuotaItem | undefined, key: QuotaField): PlatformQuotaItem[QuotaField] {
  return q?.[key]
}

function hasAnyLimit(q: PlatformQuotaItem | undefined): boolean {
  if (!q) return false
  return q.daily_limit_usd != null || q.weekly_limit_usd != null || q.monthly_limit_usd != null
}

function calcPercent(usage: number, limit: number): number {
  if (!limit || limit <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((usage / limit) * 100)))
}

function quotaBarClass(p: number): string {
  if (p >= 95) return 'bg-rose-500'
  if (p >= 75) return 'bg-amber-500'
  return 'bg-primary-500'
}

const todayTokenTotal = computed(() =>
  (props.stats?.today_input_tokens || 0) +
  (props.stats?.today_output_tokens || 0) +
  (props.stats?.today_cache_creation_tokens || 0) +
  (props.stats?.today_cache_read_tokens || 0)
)

const allTokenTotal = computed(() =>
  (props.stats?.total_input_tokens || 0) +
  (props.stats?.total_output_tokens || 0) +
  (props.stats?.total_cache_creation_tokens || 0) +
  (props.stats?.total_cache_read_tokens || 0)
)

function tokenShare(part: number, total: number): string {
  if (!total || total <= 0 || !part || part <= 0) return '0%'
  return `${Math.min(100, Math.max(0, (part / total) * 100)).toFixed(1)}%`
}

// 与 formatBalance 一致使用 Intl.NumberFormat 做半偶舍入，避免 toFixed 在不同 JS 引擎
// 下偶发截断而非四舍五入（与后端展示精度不一致）。
const usdFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})
function formatUsd(n: number): string {
  if (!Number.isFinite(n)) return '0.00'
  return usdFormatter.format(n)
}

function formatResetTime(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString(undefined, {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

const formatBalance = (b: number) =>
  new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(b)

const formatNumber = (n: number) => n.toLocaleString()
const formatCost = (c: number) => c.toFixed(4)
const formatTokens = (t: number) => {
  if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(1)}M`
  if (t >= 1000) return `${(t / 1000).toFixed(1)}K`
  return t.toString()
}
const formatDuration = (ms: number) => ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms.toFixed(0)}ms`
</script>
