<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="relative flex min-h-screen flex-col overflow-hidden bg-gray-50 dark:bg-dark-950"
  >
    <!-- Background: subtle grid + radial glows -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <!-- grid pattern -->
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(0,0,0,0.04)_1px,transparent_1px),linear-gradient(90deg,rgba(0,0,0,0.04)_1px,transparent_1px)] bg-[size:48px_48px] dark:bg-[linear-gradient(rgba(255,255,255,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.03)_1px,transparent_1px)]"
      ></div>
      <!-- glow orbs -->
      <div
        class="absolute left-1/4 top-0 h-[500px] w-[500px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-500/10 blur-[120px]"
      ></div>
      <div
        class="absolute right-1/4 top-1/3 h-[400px] w-[400px] translate-x-1/2 rounded-full bg-primary-400/8 blur-[100px]"
      ></div>
      <div
        class="absolute bottom-0 left-1/2 h-[300px] w-[600px] -translate-x-1/2 rounded-full bg-primary-500/6 blur-[80px]"
      ></div>
    </div>

    <!-- ═══════════════ HEADER ═══════════════ -->
    <header class="relative z-20 border-b border-gray-200/60 bg-white/80 backdrop-blur-md dark:border-dark-800/60 dark:bg-dark-950/80">
      <nav class="mx-auto flex h-14 max-w-6xl items-center justify-between px-6">
        <!-- Logo + Name -->
        <div class="flex items-center gap-2.5">
          <div class="h-7 w-7 overflow-hidden rounded-lg shadow-sm ring-1 ring-gray-200/50 dark:ring-dark-700/50">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ siteName }}</span>
        </div>

        <!-- Right Actions -->
        <div class="flex items-center gap-1">
          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="hidden items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white sm:flex"
          >
            <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
            </svg>
            {{ t('home.docs') }}
          </a>

          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Dark Mode Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-md p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <svg v-if="isDark" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z" />
            </svg>
            <svg v-else class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z" />
            </svg>
          </button>

          <!-- Divider -->
          <div class="mx-1 h-4 w-px bg-gray-200 dark:bg-dark-700"></div>

          <!-- Login / Dashboard -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-full bg-gray-900 py-1 pl-1 pr-2.5 text-xs font-medium text-white transition-colors hover:bg-gray-700 dark:bg-dark-700 dark:hover:bg-dark-600"
          >
            <span class="flex h-5 w-5 items-center justify-center rounded-full bg-gradient-to-br from-primary-400 to-primary-600 text-[10px] font-bold">
              {{ userInitial }}
            </span>
            {{ t('home.dashboard') }}
            <svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25" />
            </svg>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="rounded-full bg-gray-900 px-3.5 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-gray-700 dark:bg-dark-700 dark:hover:bg-dark-600"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- ═══════════════ HERO ═══════════════ -->
    <main class="relative z-10 flex-1">
      <section class="mx-auto max-w-6xl px-6 pb-16 pt-20 text-center">
        <!-- Badge -->
        <div class="mb-6 inline-flex items-center gap-2 rounded-full border border-primary-200/80 bg-primary-50/80 px-3 py-1 dark:border-primary-800/50 dark:bg-primary-950/50">
          <span class="h-1.5 w-1.5 rounded-full bg-primary-500"></span>
          <span class="text-xs font-medium text-primary-700 dark:text-primary-300">AI API Gateway Platform</span>
        </div>

        <!-- Headline -->
        <h1 class="mb-5 text-4xl font-bold tracking-tight text-gray-900 dark:text-white md:text-5xl lg:text-6xl">
          <span class="block">{{ siteName }}</span>
          <span class="mt-1 block bg-gradient-to-r from-primary-500 via-primary-400 to-primary-600 bg-clip-text text-transparent">
            {{ t('home.heroSubtitle') }}
          </span>
        </h1>

        <!-- Subtitle -->
        <p class="mx-auto mb-8 max-w-2xl text-base text-gray-600 dark:text-dark-300 md:text-lg">
          {{ siteSubtitle }}
        </p>

        <!-- CTA Buttons -->
        <div class="mb-12 flex flex-wrap items-center justify-center gap-3">
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex items-center gap-2 rounded-full bg-primary-500 px-6 py-2.5 text-sm font-semibold text-white shadow-lg shadow-primary-500/30 transition-all hover:-translate-y-0.5 hover:bg-primary-600 hover:shadow-primary-500/40"
          >
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25" />
            </svg>
          </router-link>
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-2 rounded-full border border-gray-300 bg-white px-6 py-2.5 text-sm font-semibold text-gray-700 transition-all hover:-translate-y-0.5 hover:border-gray-400 hover:shadow-sm dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200 dark:hover:border-dark-500"
          >
            {{ t('home.viewDocs') }}
          </a>
        </div>

        <!-- Stats Row -->
        <div class="mx-auto mb-16 flex max-w-lg flex-wrap items-center justify-center gap-x-8 gap-y-3">
          <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
            <svg class="h-4 w-4 text-primary-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
            </svg>
            {{ t('home.tags.stickySession') }}
          </div>
          <div class="h-1 w-1 rounded-full bg-gray-300 dark:bg-dark-600"></div>
          <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
            <svg class="h-4 w-4 text-primary-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
            </svg>
            {{ t('home.tags.subscriptionToApi') }}
          </div>
          <div class="h-1 w-1 rounded-full bg-gray-300 dark:bg-dark-600"></div>
          <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
            <svg class="h-4 w-4 text-primary-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
            </svg>
            {{ t('home.tags.realtimeBilling') }}
          </div>
        </div>

        <!-- API Code Demo -->
        <div class="mx-auto max-w-2xl">
          <div class="overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-xl shadow-gray-200/60 dark:border-dark-700/60 dark:bg-dark-900 dark:shadow-dark-950/60">
            <!-- Window bar -->
            <div class="flex items-center justify-between border-b border-gray-100/80 bg-gray-50/80 px-4 py-3 dark:border-dark-800/60 dark:bg-dark-800/60">
              <div class="flex items-center gap-1.5">
                <span class="h-3 w-3 rounded-full bg-red-400"></span>
                <span class="h-3 w-3 rounded-full bg-yellow-400"></span>
                <span class="h-3 w-3 rounded-full bg-green-400"></span>
              </div>
              <span class="font-mono text-xs text-gray-400 dark:text-dark-500">terminal</span>
              <div class="w-16"></div>
            </div>
            <!-- Code lines -->
            <div class="px-5 py-5 font-mono text-sm leading-loose">
              <div class="code-line line-1 flex flex-wrap items-center gap-2">
                <span class="text-green-500 dark:text-green-400">$</span>
                <span class="text-sky-500 dark:text-sky-400">curl</span>
                <span class="text-violet-400">-X POST</span>
                <span class="text-primary-500">/v1/messages</span>
              </div>
              <div class="code-line line-2 flex items-center gap-2">
                <span class="text-gray-400 dark:text-dark-500 italic"># Routing to upstream AI...</span>
              </div>
              <div class="code-line line-3 flex flex-wrap items-center gap-2">
                <span class="rounded bg-green-100 px-2 py-0.5 font-semibold text-green-700 dark:bg-green-900/30 dark:text-green-400">200 OK</span>
                <span class="text-amber-500 dark:text-amber-400">{"content": "Hello from Claude!"}</span>
              </div>
              <div class="code-line line-4 flex items-center gap-2">
                <span class="text-green-500 dark:text-green-400">$</span>
                <span class="cursor"></span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- ═══════════════ PROVIDERS ═══════════════ -->
      <section class="border-t border-gray-200/60 bg-white/50 py-16 backdrop-blur-sm dark:border-dark-800/60 dark:bg-dark-900/30">
        <div class="mx-auto max-w-6xl px-6">
          <div class="mb-10 text-center">
            <h2 class="mb-2 text-2xl font-bold text-gray-900 dark:text-white">
              {{ t('home.providers.title') }}
            </h2>
            <p class="text-sm text-gray-500 dark:text-dark-400">
              {{ t('home.providers.description') }}
            </p>
          </div>

          <!-- 大语言模型 -->
          <div class="mb-4 flex items-center gap-3">
            <span class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">{{ t('home.providers.llmLabel') }}</span>
            <div class="h-px flex-1 bg-gray-200/80 dark:bg-dark-700/60"></div>
          </div>
          <div class="mb-8 grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
            <!-- DeepSeek V4 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg hover:shadow-primary-500/8 dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-slate-600 to-slate-800 shadow-sm">
                  <span class="text-sm font-bold text-white">DS</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.deepseek') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">深度求索 · 1.6T MoE</p>
              </div>
            </div>
            <!-- Kimi K2.6 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg hover:shadow-primary-500/8 dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-violet-500 to-purple-700 shadow-sm">
                  <span class="text-sm font-bold text-white">Ki</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.kimi') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">月之暗面 · 1T MoE</p>
              </div>
            </div>
            <!-- 通义千问 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg hover:shadow-primary-500/8 dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-orange-500 to-amber-600 shadow-sm">
                  <span class="text-sm font-bold text-white">Qw</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.qwen') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">阿里巴巴 · Qwen 3.6</p>
              </div>
            </div>
            <!-- 豆包 Doubao 2.0 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg hover:shadow-primary-500/8 dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-cyan-600 shadow-sm">
                  <span class="text-sm font-bold text-white">DB</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.doubao') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">字节跳动 · Seed 2.0</p>
              </div>
            </div>
            <!-- 混元 Hunyuan -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg hover:shadow-primary-500/8 dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-teal-500 to-green-600 shadow-sm">
                  <span class="text-sm font-bold text-white">HY</span>
                </div>
                <span class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-semibold text-gray-500 dark:bg-dark-700 dark:text-dark-400">{{ t('home.providers.soon') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.hunyuan') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">腾讯 · 295B MoE</p>
              </div>
            </div>
          </div>

          <!-- 多模态模型 -->
          <div class="mb-4 flex items-center gap-3">
            <span class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">{{ t('home.providers.multimodalLabel') }}</span>
            <div class="h-px flex-1 bg-gray-200/80 dark:bg-dark-700/60"></div>
          </div>
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
            <!-- Seedance 2.0 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-pink-500 to-rose-600 shadow-sm">
                  <span class="text-xs font-bold text-white">Sc</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.seedance') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">字节跳动 · 文生视频</p>
              </div>
            </div>
            <!-- 可灵系列 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-orange-500 to-red-600 shadow-sm">
                  <span class="text-xs font-bold text-white">可灵</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.kling') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">快手 · 视频生成</p>
              </div>
            </div>
            <!-- 海螺系列 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-violet-500 to-purple-700 shadow-sm">
                  <span class="text-xs font-bold text-white">海螺</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.hailuo') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">MiniMax · 视频生成</p>
              </div>
            </div>
            <!-- vidu系列 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500 to-teal-600 shadow-sm">
                  <span class="text-xs font-bold text-white">vidu</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.vidu') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">生数科技 · 视频生成</p>
              </div>
            </div>
            <!-- 拍我系列 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-yellow-500 to-amber-600 shadow-sm">
                  <span class="text-xs font-bold text-white">拍我</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.paiwo') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">视频生成</p>
              </div>
            </div>
            <!-- 豆包系列 -->
            <div class="group flex cursor-default flex-col gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-4 backdrop-blur-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-lg dark:border-dark-700/60 dark:bg-dark-800/80 dark:hover:border-primary-700/40">
              <div class="flex items-center justify-between">
                <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-cyan-600 shadow-sm">
                  <span class="text-xs font-bold text-white">豆包</span>
                </div>
                <span class="rounded-md bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ t('home.providers.supported') }}</span>
              </div>
              <div>
                <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ t('home.providers.doubaovideo') }}</p>
                <p class="text-[11px] text-gray-400 dark:text-dark-500">字节跳动 · 视频生成</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- ═══════════════ FEATURES ═══════════════ -->
      <section class="py-16">
        <div class="mx-auto max-w-6xl px-6">
          <div class="grid gap-5 md:grid-cols-3">
            <!-- Feature 1 -->
            <div class="group rounded-2xl border border-gray-200/60 bg-white/70 p-6 backdrop-blur-sm transition-all duration-200 hover:-translate-y-1 hover:border-primary-200/60 hover:shadow-xl dark:border-dark-700/50 dark:bg-dark-800/60 dark:hover:border-primary-800/40">
              <div class="mb-4 flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-blue-600 shadow-lg shadow-blue-500/30 transition-transform duration-200 group-hover:scale-110">
                <svg class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z" />
                </svg>
              </div>
              <h3 class="mb-1.5 text-base font-semibold text-gray-900 dark:text-white">
                {{ t('home.features.unifiedGateway') }}
              </h3>
              <p class="text-sm leading-relaxed text-gray-500 dark:text-dark-400">
                {{ t('home.features.unifiedGatewayDesc') }}
              </p>
            </div>

            <!-- Feature 2 -->
            <div class="group rounded-2xl border border-gray-200/60 bg-white/70 p-6 backdrop-blur-sm transition-all duration-200 hover:-translate-y-1 hover:border-primary-200/60 hover:shadow-xl dark:border-dark-700/50 dark:bg-dark-800/60 dark:hover:border-primary-800/40">
              <div class="mb-4 flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-primary-500 to-primary-600 shadow-lg shadow-primary-500/30 transition-transform duration-200 group-hover:scale-110">
                <svg class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z" />
                </svg>
              </div>
              <h3 class="mb-1.5 text-base font-semibold text-gray-900 dark:text-white">
                {{ t('home.features.multiAccount') }}
              </h3>
              <p class="text-sm leading-relaxed text-gray-500 dark:text-dark-400">
                {{ t('home.features.multiAccountDesc') }}
              </p>
            </div>

            <!-- Feature 3 -->
            <div class="group rounded-2xl border border-gray-200/60 bg-white/70 p-6 backdrop-blur-sm transition-all duration-200 hover:-translate-y-1 hover:border-primary-200/60 hover:shadow-xl dark:border-dark-700/50 dark:bg-dark-800/60 dark:hover:border-primary-800/40">
              <div class="mb-4 flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-purple-500 to-purple-600 shadow-lg shadow-purple-500/30 transition-transform duration-200 group-hover:scale-110">
                <svg class="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" />
                </svg>
              </div>
              <h3 class="mb-1.5 text-base font-semibold text-gray-900 dark:text-white">
                {{ t('home.features.balanceQuota') }}
              </h3>
              <p class="text-sm leading-relaxed text-gray-500 dark:text-dark-400">
                {{ t('home.features.balanceQuotaDesc') }}
              </p>
            </div>
          </div>
        </div>
      </section>

      <!-- ═══════════════ CTA ═══════════════ -->
      <section class="py-16">
        <div class="mx-auto max-w-6xl px-6">
          <div class="relative overflow-hidden rounded-3xl bg-gradient-to-br from-primary-500 to-primary-600 p-10 text-center shadow-2xl shadow-primary-500/20">
            <!-- subtle inner pattern -->
            <div class="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.05)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.05)_1px,transparent_1px)] bg-[size:32px_32px]"></div>
            <div class="relative">
              <h2 class="mb-3 text-2xl font-bold text-white md:text-3xl">
                {{ t('home.cta.title') }}
              </h2>
              <p class="mb-7 text-primary-100/90">
                {{ t('home.cta.description') }}
              </p>
              <router-link
                to="/login"
                class="inline-flex items-center gap-2 rounded-full bg-white px-7 py-3 text-sm font-semibold text-primary-600 shadow-lg transition-all hover:-translate-y-0.5 hover:shadow-xl"
              >
                {{ t('home.cta.button') }}
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25" />
                </svg>
              </router-link>
            </div>
          </div>
        </div>
      </section>
    </main>

    <!-- ═══════════════ FOOTER ═══════════════ -->
    <footer class="relative z-10 border-t border-gray-200/60 bg-white/50 px-6 py-8 backdrop-blur-sm dark:border-dark-800/60 dark:bg-dark-900/30">
      <div class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 sm:flex-row">
        <p class="text-xs text-gray-400 dark:text-dark-500">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-5">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-xs text-gray-400 transition-colors hover:text-gray-600 dark:text-dark-500 dark:hover:text-dark-300"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-xs text-gray-400 transition-colors hover:text-gray-600 dark:text-dark-500 dark:hover:text-dark-300"
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

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'TokenPanel')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Auth
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const userInitial = computed(() => {
  const user = authStore.user
  return user?.email ? user.email.charAt(0).toUpperCase() : ''
})

const currentYear = computed(() => new Date().getFullYear())
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Dark mode
const isDark = ref(document.documentElement.classList.contains('dark'))
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedDark = localStorage.getItem('theme')
  if (savedDark === 'dark' || (!savedDark && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.code-line {
  opacity: 0;
  animation: line-appear 0.4s ease forwards;
}
.line-1 { animation-delay: 0.3s; }
.line-2 { animation-delay: 1.0s; }
.line-3 { animation-delay: 1.8s; }
.line-4 { animation-delay: 2.5s; }

@keyframes line-appear {
  from { opacity: 0; transform: translateY(4px); }
  to   { opacity: 1; transform: translateY(0); }
}

.cursor {
  display: inline-block;
  width: 8px;
  height: 15px;
  background: #22c55e;
  border-radius: 1px;
  animation: blink 1s step-end infinite;
}
@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
</style>
