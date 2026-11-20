<template>
  <div class="relative flex min-h-screen items-center justify-center bg-[#f6fafe] p-4 dark:bg-dark-950">
    <!-- Content Container -->
    <div class="relative z-10 w-full max-w-sm">
      <!-- Logo/Brand -->
      <div class="mb-6 text-center">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded">
          <div
            class="mb-3 inline-flex h-12 w-12 items-center justify-center overflow-hidden rounded-xl border border-primary-200 bg-primary-50 dark:border-[#4d2c3e] dark:bg-[#2e1d27]"
          >
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="mb-1 text-2xl font-bold text-[#252b3b] dark:text-white">
            {{ siteName }}
          </h1>
          <p class="text-xs text-[#929cb3] dark:text-dark-400">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="card p-6 sm:p-7">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-5 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-6 text-center text-xs text-[#929cb3] dark:text-dark-400">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.text-gradient {
  @apply text-primary-500 dark:text-[#ff9bbd];
}
</style>
