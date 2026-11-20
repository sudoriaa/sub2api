<template>
  <div class="card overflow-hidden">
    <div class="flex items-center justify-between border-b border-[#e6edf5] px-5 py-3.5 dark:border-dark-700">
      <h2 class="text-sm font-bold text-[#252b3b] dark:text-white">{{ t('dashboard.recentUsage') }}</h2>
      <span class="badge badge-sky">{{ t('dashboard.last7Days') }}</span>
    </div>
    <div class="px-5 py-2">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner size="lg" />
      </div>
      <div v-else-if="data.length === 0" class="py-8">
        <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
      </div>
      <div v-else class="divide-y divide-[#e6edf5] dark:divide-dark-700">
        <div v-for="(log, idx) in data" :key="log.id" class="flex items-center justify-between gap-4 py-3">
          <div class="flex min-w-0 items-center gap-3">
            <div
              class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
              :class="idx % 2 === 0 ? 'bg-primary-50 text-primary-700 dark:bg-[#2e1d27] dark:text-[#ff9bbd]' : 'bg-accent-50 text-accent-700 dark:bg-[#1a293a] dark:text-[#8ad0ff]'"
            >
              <Icon name="beaker" size="sm" />
            </div>
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-[#252b3b] dark:text-white">{{ log.model }}</p>
              <p v-if="visibleResponseModel(log)" class="truncate text-[11px] text-[#929cb3] dark:text-dark-400">
                {{ t('usage.upstreamResponseModel') }}: {{ visibleResponseModel(log) }}
              </p>
              <p class="text-[11px] text-[#929cb3] dark:text-dark-400">{{ formatDateTime(log.created_at) }}</p>
            </div>
          </div>
          <div class="shrink-0 text-right tabular-nums">
            <p class="text-sm font-bold">
              <span class="text-primary-700 dark:text-[#ff9bbd]" :title="t('dashboard.actual')">${{ formatCost(log.actual_cost) }}</span>
              <span class="font-normal text-[#929cb3] dark:text-dark-400" :title="t('dashboard.standard')"> / ${{ formatCost(log.total_cost) }}</span>
            </p>
            <p class="text-[11px] text-[#929cb3] dark:text-dark-400">{{ (log.input_tokens + log.output_tokens).toLocaleString() }} tokens</p>
          </div>
        </div>

        <router-link to="/usage" class="flex items-center justify-center gap-1.5 py-3 text-xs font-semibold text-accent-700 transition-colors hover:text-accent-600 dark:text-[#8ad0ff] dark:hover:text-accent-300">
          {{ t('dashboard.viewAllUsage') }}
          <Icon name="arrowRight" size="sm" />
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { userVisibleUpstreamResponseModel } from '@/utils/upstreamResponseModel'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()
const { t } = useI18n()
const appStore = useAppStore()
const visibleResponseModel = (log: UsageLog) => userVisibleUpstreamResponseModel(appStore.cachedPublicSettings, log)
const formatCost = (c: number) => c.toFixed(4)
</script>
