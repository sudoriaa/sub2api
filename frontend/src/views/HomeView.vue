<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="relative flex min-h-screen flex-col bg-[#f6fafe] text-[#252b3b] dark:bg-dark-950 dark:text-[#eef2fa]"
  >
    <!-- Header -->
    <header class="relative z-20 border-b border-[#e6edf5] bg-white px-6 py-3 dark:border-dark-700 dark:bg-dark-800">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <!-- Logo -->
        <div class="flex items-center gap-2.5">
          <div class="h-9 w-9 overflow-hidden rounded-lg border border-primary-200 bg-primary-50 dark:border-[#4d2c3e] dark:bg-[#2e1d27]">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="text-base font-bold text-[#252b3b] dark:text-white">{{ siteName }}</span>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-2">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-lg p-2 text-[#5d677e] transition-colors hover:bg-[#f3f7fc] hover:text-[#252b3b] dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Model Plaza Link -->
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-[#5d677e] transition-colors hover:bg-[#f3f7fc] hover:text-[#252b3b] dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-[#5d677e] transition-colors hover:bg-[#f3f7fc] hover:text-[#252b3b] dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Login / Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-lg bg-primary-500 py-1.5 pl-1.5 pr-3 text-xs font-semibold text-white transition-colors hover:bg-primary-600"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-md bg-white/20 text-[10px] font-bold text-white"
            >
              {{ userInitial }}
            </span>
            <span>{{ t('home.dashboard') }}</span>
            <svg
              class="h-3 w-3 text-white/90"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25"
              />
            </svg>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-lg bg-primary-500 px-3.5 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-primary-600"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-6 py-14">
      <div class="mx-auto max-w-6xl">
        <!-- Hero Section - Left/Right Layout -->
        <div class="mb-12 flex flex-col items-center justify-between gap-10 lg:flex-row lg:gap-14">
          <!-- Left: Text Content -->
          <div class="flex-1 text-center lg:text-left">
            <h1
              class="mb-4 text-4xl font-extrabold tracking-tight text-[#252b3b] dark:text-white md:text-5xl"
            >
              {{ siteName }}
            </h1>
            <p class="mb-7 text-base leading-relaxed text-[#5d677e] dark:text-dark-300 md:text-lg">
              {{ siteSubtitle }}
            </p>

            <!-- CTA Button -->
            <div>
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="btn btn-primary px-7 py-2.5 text-sm"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="md" class="ml-1" :stroke-width="2" />
              </router-link>
            </div>
          </div>

          <!-- Right: Terminal Animation -->
          <div class="flex flex-1 justify-center lg:justify-end">
            <div class="terminal-container">
              <div class="terminal-window">
                <!-- Window header -->
                <div class="terminal-header">
                  <div class="terminal-buttons">
                    <span class="btn-close"></span>
                    <span class="btn-minimize"></span>
                    <span class="btn-maximize"></span>
                  </div>
                  <span class="terminal-title">terminal</span>
                </div>
                <!-- Terminal content -->
                <div class="terminal-body">
                  <div class="code-line line-1">
                    <span class="code-prompt">$</span>
                    <span class="code-cmd">curl</span>
                    <span class="code-flag">-X POST</span>
                    <span class="code-url">/v1/messages</span>
                  </div>
                  <div class="code-line line-2">
                    <span class="code-comment"># Routing to upstream...</span>
                  </div>
                  <div class="code-line line-3">
                    <span class="code-success">200 OK</span>
                    <span class="code-response">{ "content": "Hello!" }</span>
                  </div>
                  <div class="code-line line-4">
                    <span class="code-prompt">$</span>
                    <span class="cursor"></span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Feature Tags - Centered -->
        <div class="mb-10 flex flex-wrap items-center justify-center gap-3 md:gap-4">
          <div
            class="inline-flex items-center gap-2 rounded-lg border border-primary-200 bg-primary-50 px-4 py-2 dark:border-[#4d2c3e] dark:bg-[#2e1d27]"
          >
            <Icon name="swap" size="sm" class="text-primary-700 dark:text-[#ff9bbd]" />
            <span class="text-xs font-semibold text-primary-700 dark:text-[#ff9bbd]">{{
              t('home.tags.subscriptionToApi')
            }}</span>
          </div>
          <div
            class="inline-flex items-center gap-2 rounded-lg border border-accent-200 bg-accent-50 px-4 py-2 dark:border-[#2a435e] dark:bg-[#1a293a]"
          >
            <Icon name="shield" size="sm" class="text-accent-700 dark:text-[#8ad0ff]" />
            <span class="text-xs font-semibold text-accent-700 dark:text-[#8ad0ff]">{{
              t('home.tags.stickySession')
            }}</span>
          </div>
          <div
            class="inline-flex items-center gap-2 rounded-lg border border-[#e6edf5] bg-white px-4 py-2 dark:border-dark-700 dark:bg-dark-800"
          >
            <Icon name="chart" size="sm" class="text-primary-500" />
            <span class="text-xs font-semibold text-[#252b3b] dark:text-dark-100">{{
              t('home.tags.realtimeBilling')
            }}</span>
          </div>
        </div>

        <!-- Features Grid -->
        <div class="mb-12 grid gap-5 md:grid-cols-3">
          <!-- Feature 1: Unified Gateway -->
          <div class="card card-hover p-5">
            <div class="mb-3.5 h-1.5 w-8 rounded-full bg-primary-500"></div>
            <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-[#2e1d27] dark:text-[#ff9bbd]">
              <Icon name="server" size="md" />
            </div>
            <h3 class="mb-1.5 text-base font-bold text-[#252b3b] dark:text-white">
              {{ t('home.features.unifiedGateway') }}
            </h3>
            <p class="text-xs leading-relaxed text-[#5d677e] dark:text-dark-300">
              {{ t('home.features.unifiedGatewayDesc') }}
            </p>
          </div>

          <!-- Feature 2: Account Pool -->
          <div class="card card-hover p-5">
            <div class="mb-3.5 h-1.5 w-8 rounded-full bg-accent-500"></div>
            <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-accent-50 text-accent-700 dark:bg-[#1a293a] dark:text-[#8ad0ff]">
              <svg
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.75"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z"
                />
              </svg>
            </div>
            <h3 class="mb-1.5 text-base font-bold text-[#252b3b] dark:text-white">
              {{ t('home.features.multiAccount') }}
            </h3>
            <p class="text-xs leading-relaxed text-[#5d677e] dark:text-dark-300">
              {{ t('home.features.multiAccountDesc') }}
            </p>
          </div>

          <!-- Feature 3: Billing & Quota -->
          <div class="card card-hover p-5">
            <div class="mb-3.5 h-1.5 w-8 rounded-full bg-primary-500"></div>
            <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-[#2e1d27] dark:text-[#ff9bbd]">
              <svg
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.75"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z"
                />
              </svg>
            </div>
            <h3 class="mb-1.5 text-base font-bold text-[#252b3b] dark:text-white">
              {{ t('home.features.balanceQuota') }}
            </h3>
            <p class="text-xs leading-relaxed text-[#5d677e] dark:text-dark-300">
              {{ t('home.features.balanceQuotaDesc') }}
            </p>
          </div>
        </div>

        <!-- Supported Providers -->
        <div class="mb-6 text-center">
          <h2 class="mb-2 text-xl font-bold text-[#252b3b] dark:text-white">
            {{ t('home.providers.title') }}
          </h2>
          <p class="text-xs text-[#929cb3] dark:text-dark-400">
            {{ t('home.providers.description') }}
          </p>
        </div>

        <div class="mb-14 flex flex-wrap items-center justify-center gap-3">
          <!-- Claude - Supported -->
          <div class="flex items-center gap-2.5 rounded-xl border border-primary-200 bg-white px-4 py-2.5 dark:border-[#4d2c3e] dark:bg-dark-800">
            <div class="flex h-7 w-7 items-center justify-center rounded-lg bg-primary-500 text-xs font-bold text-white">C</div>
            <span class="text-sm font-semibold text-[#252b3b] dark:text-dark-100">{{ t('home.providers.claude') }}</span>
            <span class="badge badge-primary !py-0 text-[10px]">{{ t('home.providers.supported') }}</span>
          </div>
          <!-- GPT - Supported -->
          <div class="flex items-center gap-2.5 rounded-xl border border-accent-200 bg-white px-4 py-2.5 dark:border-[#2a435e] dark:bg-dark-800">
            <div class="flex h-7 w-7 items-center justify-center rounded-lg bg-accent-500 text-xs font-bold text-white">G</div>
            <span class="text-sm font-semibold text-[#252b3b] dark:text-dark-100">GPT</span>
            <span class="badge badge-sky !py-0 text-[10px]">{{ t('home.providers.supported') }}</span>
          </div>
          <!-- Gemini - Supported -->
          <div class="flex items-center gap-2.5 rounded-xl border border-accent-200 bg-white px-4 py-2.5 dark:border-[#2a435e] dark:bg-dark-800">
            <div class="flex h-7 w-7 items-center justify-center rounded-lg bg-accent-500 text-xs font-bold text-white">G</div>
            <span class="text-sm font-semibold text-[#252b3b] dark:text-dark-100">{{ t('home.providers.gemini') }}</span>
            <span class="badge badge-sky !py-0 text-[10px]">{{ t('home.providers.supported') }}</span>
          </div>
          <!-- Antigravity - Supported -->
          <div class="flex items-center gap-2.5 rounded-xl border border-primary-200 bg-white px-4 py-2.5 dark:border-[#4d2c3e] dark:bg-dark-800">
            <div class="flex h-7 w-7 items-center justify-center rounded-lg bg-primary-500 text-xs font-bold text-white">A</div>
            <span class="text-sm font-semibold text-[#252b3b] dark:text-dark-100">{{ t('home.providers.antigravity') }}</span>
            <span class="badge badge-primary !py-0 text-[10px]">{{ t('home.providers.supported') }}</span>
          </div>
          <!-- More - Coming Soon -->
          <div class="flex items-center gap-2.5 rounded-xl border border-[#e6edf5] bg-white px-4 py-2.5 opacity-75 dark:border-dark-700 dark:bg-dark-800">
            <div class="flex h-7 w-7 items-center justify-center rounded-lg bg-[#e6edf5] text-xs font-bold text-[#5d677e] dark:bg-dark-700 dark:text-dark-300">+</div>
            <span class="text-sm font-semibold text-[#5d677e] dark:text-dark-300">{{ t('home.providers.more') }}</span>
            <span class="badge badge-gray !py-0 text-[10px]">{{ t('home.providers.soon') }}</span>
          </div>
        </div>
      </div>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-[#e6edf5] bg-white px-6 py-6 dark:border-dark-700 dark:bg-dark-800">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-xs text-[#929cb3] dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-xs text-[#5d677e] transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-xs text-[#5d677e] transition-colors hover:text-accent-600 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
/* Terminal Container */
.terminal-container {
  position: relative;
  display: inline-block;
}

/* Terminal Window - 简约纯白卡片 + 浅蓝顶栏 */
.terminal-window {
  width: 420px;
  max-width: 100%;
  background: #ffffff;
  border: 1px solid #e6edf5;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(37, 43, 59, 0.05);
  overflow: hidden;
}

/* Terminal Header */
.terminal-header {
  display: flex;
  align-items: center;
  padding: 10px 14px;
  background: #eef8ff;
  border-bottom: 1px solid #e6edf5;
}

.terminal-buttons {
  display: flex;
  gap: 6px;
}

.terminal-buttons span {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.btn-close {
  background: #f4729b;
}
.btn-minimize {
  background: #5eb5f7;
}
.btn-maximize {
  background: #d5e0ec;
}

.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 12px;
  font-family: ui-monospace, monospace;
  color: #5d677e;
  margin-right: 42px;
}

/* Terminal Body */
.terminal-body {
  padding: 18px 20px;
  font-family: ui-monospace, 'Fira Code', monospace;
  font-size: 13px;
  line-height: 1.9;
  color: #252b3b;
}

.code-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  opacity: 0;
  animation: line-appear 0.5s ease forwards;
}

.line-1 {
  animation-delay: 0.3s;
}
.line-2 {
  animation-delay: 1s;
}
.line-3 {
  animation-delay: 1.8s;
}
.line-4 {
  animation-delay: 2.5s;
}

@keyframes line-appear {
  from {
    opacity: 0;
    transform: translateY(5px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.code-prompt {
  color: #d64576;
  font-weight: bold;
}
.code-cmd {
  color: #2084d4;
}
.code-flag {
  color: #5d677e;
}
.code-url {
  color: #d64576;
}
.code-comment {
  color: #929cb3;
  font-style: italic;
}
.code-success {
  color: #059669;
  background: #ecfdf5;
  border: 1px solid #bbf7d0;
  padding: 1px 7px;
  border-radius: 6px;
  font-weight: 600;
}
.code-response {
  color: #252b3b;
}

/* Blinking Cursor */
.cursor {
  display: inline-block;
  width: 7px;
  height: 15px;
  background: #f4729b;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%,
  50% {
    opacity: 1;
  }
  51%,
  100% {
    opacity: 0;
  }
}

/* Dark mode adjustments */
:global(.dark) .terminal-window {
  background: #1b1f2e;
  border-color: #2a3147;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.28);
}
:global(.dark) .terminal-header {
  background: #1a293a;
  border-bottom-color: #2a3147;
}
:global(.dark) .terminal-title {
  color: #a5b0c7;
}
:global(.dark) .terminal-body,
:global(.dark) .code-response {
  color: #eef2fa;
}
:global(.dark) .code-prompt,
:global(.dark) .code-url {
  color: #ff9bbd;
}
:global(.dark) .code-cmd {
  color: #8ad0ff;
}
:global(.dark) .code-success {
  color: #6ee7b7;
  background: #142c24;
  border-color: #1f4d3d;
}
</style>
