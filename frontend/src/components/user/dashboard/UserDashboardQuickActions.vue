<template>
  <div class="card p-5">
    <div class="mb-3.5 flex items-center justify-between">
      <h2 class="text-sm font-bold text-[#252b3b] dark:text-white">{{ t('dashboard.quickActions') }}</h2>
    </div>
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <button
        @click="router.push('/keys')"
        class="group rounded-xl border border-primary-200 bg-primary-50 p-3.5 text-left transition-colors hover:border-primary-300 dark:border-[#4d2c3e] dark:bg-[#2e1d27]"
      >
        <div class="mb-2.5 flex h-8 w-8 items-center justify-center rounded-lg bg-primary-500 text-white">
          <Icon name="key" size="sm" />
        </div>
        <p class="text-sm font-bold text-[#252b3b] dark:text-white">{{ t('dashboard.createApiKey') }}</p>
        <p class="mt-0.5 text-[11px] text-[#5d677e] dark:text-dark-300">{{ t('dashboard.generateNewKey') }}</p>
      </button>

      <button
        @click="router.push('/usage')"
        class="group rounded-xl border border-accent-200 bg-accent-50 p-3.5 text-left transition-colors hover:border-accent-300 dark:border-[#2a435e] dark:bg-[#1a293a]"
      >
        <div class="mb-2.5 flex h-8 w-8 items-center justify-center rounded-lg bg-accent-500 text-white">
          <Icon name="chart" size="sm" />
        </div>
        <p class="text-sm font-bold text-[#252b3b] dark:text-white">{{ t('dashboard.viewUsage') }}</p>
        <p class="mt-0.5 text-[11px] text-[#5d677e] dark:text-dark-300">{{ t('dashboard.checkDetailedLogs') }}</p>
      </button>

      <button
        v-if="canUseBatchImage"
        @click="router.push('/batch-image')"
        class="group rounded-xl border border-[#e6edf5] bg-white p-3.5 text-left transition-colors hover:border-accent-200 dark:border-dark-700 dark:bg-dark-800 dark:hover:border-accent-800"
      >
        <div class="mb-2.5 flex h-8 w-8 items-center justify-center rounded-lg bg-accent-50 text-accent-700 dark:bg-[#1a293a] dark:text-[#8ad0ff]">
          <Icon name="sparkles" size="sm" />
        </div>
        <p class="text-sm font-bold text-[#252b3b] dark:text-white">{{ t('dashboard.batchImageAgent') }}</p>
        <p class="mt-0.5 text-[11px] text-[#929cb3] dark:text-dark-400">{{ t('dashboard.batchImageAgentDesc') }}</p>
      </button>

      <button
        @click="router.push('/redeem')"
        class="group rounded-xl border border-[#e6edf5] bg-white p-3.5 text-left transition-colors hover:border-primary-200 dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-800"
      >
        <div class="mb-2.5 flex h-8 w-8 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-[#2e1d27] dark:text-[#ff9bbd]">
          <Icon name="gift" size="sm" />
        </div>
        <p class="text-sm font-bold text-[#252b3b] dark:text-white">{{ t('dashboard.redeemCode') }}</p>
        <p class="mt-0.5 text-[11px] text-[#929cb3] dark:text-dark-400">{{ t('dashboard.addBalanceWithCode') }}</p>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
const router = useRouter()
const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>
