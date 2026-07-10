<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    id="top"
    class="min-h-screen bg-stone-50 text-stone-950 dark:bg-[#050505] dark:text-white"
  >
    <header
      class="sticky top-0 z-50 border-b border-stone-200/80 bg-white/90 backdrop-blur dark:border-[#1e1e1e] dark:bg-[#050505]/90"
    >
      <nav class="mx-auto flex h-16 max-w-6xl items-center justify-between px-4">
        <router-link to="/home" class="flex min-w-0 items-center gap-3">
          <div
            class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-emerald-500/30 bg-emerald-500/10"
          >
            <img
              v-if="siteLogo"
              :src="siteLogo"
              alt="Logo"
              class="h-full w-full object-contain"
            />
            <Icon v-else name="sparkles" size="md" class="text-emerald-500" />
          </div>
          <span class="truncate text-lg font-bold tracking-tight text-emerald-500">
            {{ siteName }}
          </span>
        </router-link>

        <div class="flex items-center gap-2 sm:gap-3">
          <LocaleSwitcher />

          <button
            type="button"
            class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-stone-200 text-stone-500 transition hover:border-emerald-500/40 hover:text-emerald-500 dark:border-[#1e1e1e] dark:text-stone-400"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>

          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex h-9 items-center gap-2 rounded-lg bg-emerald-500 px-3 text-sm font-semibold text-black transition hover:bg-emerald-400"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-full bg-black/15 text-[10px]"
            >
              {{ userInitial }}
            </span>
            <span class="hidden sm:inline">{{ t('home.dashboard') }}</span>
          </router-link>
          <template v-else>
            <router-link
              to="/login"
              class="hidden h-9 items-center rounded-lg border border-stone-200 px-3 text-sm font-medium text-stone-700 transition hover:border-emerald-500/40 hover:text-emerald-600 dark:border-[#1e1e1e] dark:text-stone-300 sm:inline-flex"
            >
              {{ t('home.login') }}
            </router-link>
            <router-link
              to="/register"
              class="inline-flex h-9 items-center rounded-lg bg-emerald-500 px-3 text-sm font-semibold text-black transition hover:bg-emerald-400"
            >
              {{ t('home.nav.register') }}
            </router-link>
          </template>
        </div>
      </nav>
    </header>

    <main>
      <div class="product-flow">
      <section
        class="relative overflow-hidden py-16 md:py-24"
      >
        <div
          class="pointer-events-none absolute inset-0 opacity-[0.22] dark:opacity-[0.18]"
          aria-hidden="true"
        >
          <div
            class="h-full w-full bg-[linear-gradient(rgba(34,197,94,0.18)_1px,transparent_1px),linear-gradient(90deg,rgba(34,197,94,0.14)_1px,transparent_1px)] bg-[size:56px_56px]"
          ></div>
        </div>
        <div class="relative mx-auto max-w-6xl px-4 text-center">
          <div
            class="mx-auto mb-6 inline-flex items-center gap-2 rounded-full border border-emerald-500/25 bg-white/70 px-4 py-2 text-xs font-semibold uppercase text-emerald-600 shadow-sm dark:bg-black/30 dark:text-emerald-400"
          >
            <span class="h-2 w-2 rounded-full bg-emerald-500"></span>
            {{ t('home.hero.badge') }}
          </div>

          <h1
            class="mx-auto max-w-5xl text-4xl font-black leading-tight tracking-tight text-stone-950 dark:text-white md:text-6xl"
          >
            {{ t('home.hero.titleLead') }}
            <span class="text-emerald-500">{{ t('home.hero.titleHighlight') }}</span>
          </h1>
          <p
            class="mx-auto mt-6 max-w-3xl text-base leading-8 text-stone-600 dark:text-stone-400 md:text-lg"
          >
            {{ t('home.hero.description') || siteSubtitle }}
          </p>

          <div class="mt-10 flex flex-col justify-center gap-3 sm:flex-row">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/register'"
              class="inline-flex items-center justify-center rounded-lg bg-emerald-500 px-8 py-4 text-base font-bold text-black transition hover:bg-emerald-400 hover:shadow-lg hover:shadow-emerald-500/20"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.hero.primaryCta') }}
              <Icon name="arrowRight" size="sm" class="ml-2" :stroke-width="2" />
            </router-link>
            <a
              :href="docUrl || '#quick-access'"
              :target="docUrl ? '_blank' : undefined"
              :rel="docUrl ? 'noopener noreferrer' : undefined"
              class="inline-flex items-center justify-center rounded-lg border border-stone-300 bg-white/70 px-8 py-4 text-base font-semibold text-stone-800 transition hover:border-emerald-500/50 hover:text-emerald-600 dark:border-[#1e1e1e] dark:bg-black/20 dark:text-white"
            >
              {{ t('home.hero.secondaryCta') }}
            </a>
          </div>

          <div
            class="mt-12 flex flex-wrap justify-center gap-x-8 gap-y-3 text-sm font-medium text-stone-500 dark:text-stone-500"
          >
            <span v-for="provider in heroProviders" :key="provider">{{ provider }}</span>
          </div>
        </div>
      </section>

      <section id="quick-access" class="product-flow-block pt-20 pb-14">
        <div class="mx-auto max-w-6xl px-4">
          <div class="mb-10 max-w-3xl">
            <p class="mb-3 text-xs font-bold uppercase tracking-[0.32em] text-emerald-500">
              Quickstart
            </p>
            <h2 class="text-3xl font-bold tracking-tight md:text-4xl">
              {{ t('home.quickAccess.title') }}
            </h2>
            <p class="mt-4 max-w-2xl leading-7 text-stone-500 dark:text-stone-400">
              {{ t('home.quickAccess.guideDesc') }}
            </p>
          </div>

          <div class="grid grid-cols-1 gap-6 lg:grid-cols-[0.9fr_1.35fr]">
            <div class="tech-panel p-6 md:p-8">
              <div class="relative space-y-6">
                <div
                  class="absolute bottom-8 left-5 top-5 w-px bg-gradient-to-b from-emerald-500/80 via-emerald-500/30 to-transparent"
                  aria-hidden="true"
                ></div>
                <div
                  v-for="(step, index) in quickSteps"
                  :key="step"
                  class="relative flex gap-4"
                >
                  <span
                    class="z-10 flex h-10 w-10 shrink-0 items-center justify-center rounded-full border border-emerald-500/40 bg-[#07130d] text-xs font-bold text-emerald-400 shadow-[0_0_24px_rgba(16,185,129,0.18)]"
                  >
                    0{{ index + 1 }}
                  </span>
                  <div class="pt-1">
                    <h3 class="text-base font-semibold text-stone-950 dark:text-white">
                      {{ step }}
                    </h3>
                    <p class="mt-2 text-sm leading-6 text-stone-500 dark:text-stone-400">
                      {{ quickStepDescriptions[index] }}
                    </p>
                  </div>
                </div>
              </div>

              <a
                :href="docUrl || '#faq'"
                :target="docUrl ? '_blank' : undefined"
                :rel="docUrl ? 'noopener noreferrer' : undefined"
                class="mt-8 inline-flex items-center rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-5 py-3 text-sm font-semibold text-emerald-600 transition hover:bg-emerald-500/15 dark:text-emerald-400"
              >
                {{ t('home.quickAccess.fullDoc') }}
                <Icon name="arrowRight" size="xs" class="ml-2" />
              </a>
            </div>

            <div class="tech-panel overflow-hidden">
              <div class="flex items-center justify-between border-b border-stone-200/80 px-4 py-3 dark:border-[#1e1e1e]">
                <div class="flex items-center gap-2">
                  <span class="h-3 w-3 rounded-full bg-red-500/80"></span>
                  <span class="h-3 w-3 rounded-full bg-amber-400/80"></span>
                  <span class="h-3 w-3 rounded-full bg-emerald-500/80"></span>
                </div>
                <span class="rounded-full border border-emerald-500/25 px-3 py-1 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                  OpenAI compatible
                </span>
              </div>
              <div class="border-b border-stone-200/80 px-4 py-3 dark:border-[#1e1e1e]">
                <div class="flex flex-wrap gap-2">
                  <button
                    v-for="example in codeExamples"
                    :key="example.id"
                    type="button"
                    class="rounded-md px-3 py-1.5 text-sm font-semibold transition"
                    :class="
                      activeCodeTab === example.id
                        ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                        : 'text-stone-500 hover:bg-stone-100 dark:text-stone-400 dark:hover:bg-white/5'
                    "
                    @click="activeCodeTab = example.id"
                  >
                    {{ example.label }}
                  </button>
                </div>
              </div>
              <pre
                class="min-h-[310px] overflow-x-auto bg-stone-950 p-5 text-sm leading-7 text-stone-300 dark:bg-[#050505]"
              ><code>{{ activeCodeExample.code }}</code></pre>
              <div class="flex flex-wrap items-center gap-3 border-t border-stone-200/80 px-4 py-3 text-xs text-stone-500 dark:border-[#1e1e1e]">
                <span class="inline-flex items-center gap-1.5">
                  <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                  200 OK
                </span>
                <span>Latency 128ms</span>
                <span>Unified gateway</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section id="advantages" class="product-flow-block py-14">
        <div class="mx-auto max-w-6xl px-4">
          <div class="mb-10 max-w-3xl">
            <p class="mb-3 text-xs font-bold uppercase tracking-[0.32em] text-emerald-500">
              Platform
            </p>
            <h2 class="text-3xl font-bold tracking-tight md:text-4xl">
              {{ t('home.advantages.title') }}
            </h2>
            <p class="mt-4 leading-7 text-stone-500 dark:text-stone-400">
              {{ t('home.advantages.description') }}
            </p>
          </div>

          <div class="grid grid-cols-1 border-y border-stone-200/70 bg-white/35 backdrop-blur-sm dark:border-[#1e1e1e] dark:bg-black/10 md:grid-cols-3">
            <article
              v-for="advantage in advantages"
              :key="advantage.title"
              class="group border-b border-stone-200/80 py-8 md:border-b-0 md:border-r md:px-8 md:last:border-r-0 dark:border-[#1e1e1e]"
            >
              <Icon :name="advantage.icon" size="lg" class="mb-5 text-emerald-500" />
              <h3 class="text-xl font-semibold">{{ advantage.title }}</h3>
              <p class="mt-3 leading-7 text-stone-500 dark:text-stone-400">
                {{ advantage.description }}
              </p>
            </article>
          </div>

          <div class="mt-8 grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div class="md:px-8">
              <p class="text-sm text-stone-500">OpenAI compatible</p>
              <p class="mt-1 text-lg font-semibold">{{ t('home.advantages.format') }}</p>
            </div>
            <div class="md:px-8">
              <p class="text-sm text-stone-500">Stable access</p>
              <p class="mt-1 text-lg font-semibold">{{ t('home.advantages.stable') }}</p>
            </div>
            <div class="md:px-8">
              <p class="text-sm text-stone-500">Usage visibility</p>
              <p class="mt-1 text-lg font-semibold">{{ t('home.advantages.usage') }}</p>
            </div>
          </div>
        </div>
      </section>

      <section id="models" class="product-flow-block py-16">
        <div class="mx-auto max-w-6xl px-4">
          <div class="mb-10 flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
            <div class="max-w-3xl">
              <p class="mb-3 text-xs font-bold uppercase tracking-[0.32em] text-emerald-500">
                Model Catalog
              </p>
              <h2 class="text-3xl font-bold tracking-tight md:text-4xl">{{ t('home.models.title') }}</h2>
              <p class="mt-4 leading-7 text-stone-500 dark:text-stone-400">
                {{ t('home.models.description') }}
              </p>
            </div>
            <router-link
              to="/available-channels"
              class="inline-flex w-fit items-center rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-5 py-3 text-sm font-semibold text-emerald-600 transition hover:bg-emerald-500/15 dark:text-emerald-400"
            >
              {{ t('home.models.more') }}
              <Icon name="arrowRight" size="xs" class="ml-2" />
            </router-link>
          </div>

          <div class="grid grid-cols-1 gap-4 lg:grid-cols-[0.88fr_1.12fr]">
            <div class="tech-panel p-6">
              <div class="flex items-center justify-between border-b border-stone-200/80 pb-5 dark:border-[#1e1e1e]">
                <div>
                  <p class="text-sm text-stone-500">Unified access</p>
                  <p class="mt-1 text-xl font-semibold">OpenAI compatible</p>
                </div>
                <span class="rounded-full bg-emerald-500/10 px-3 py-1 text-xs font-bold text-emerald-600 dark:text-emerald-400">
                  Live
                </span>
              </div>
              <div class="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-stone-200/80 bg-stone-200/80 dark:border-[#1e1e1e] dark:bg-[#1e1e1e]">
                <div
                  v-for="metric in modelMetrics"
                  :key="metric.label"
                  class="bg-white p-4 dark:bg-[#101010]"
                >
                  <p class="text-xs uppercase tracking-[0.18em] text-stone-500">{{ metric.label }}</p>
                  <p class="mt-3 text-2xl font-bold">{{ metric.value }}</p>
                  <p class="mt-1 text-sm text-stone-500">{{ metric.hint }}</p>
                </div>
              </div>
              <div class="mt-6 space-y-3 text-sm text-stone-500 dark:text-stone-400">
                <p class="flex items-center gap-3 leading-7">
                  <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500"></span>
                  <span>{{ t('home.models.compatibilityBullet') }}</span>
                </p>
                <p class="flex items-center gap-3 leading-7">
                  <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500"></span>
                  <span>{{ t('home.models.usageBullet') }}</span>
                </p>
              </div>
            </div>

            <div class="tech-panel overflow-hidden">
              <div class="grid grid-cols-[minmax(0,1fr)_auto] border-b border-stone-200/80 px-5 py-3 text-xs font-semibold uppercase tracking-[0.18em] text-stone-500 dark:border-[#1e1e1e] md:grid-cols-[minmax(16rem,1fr)_minmax(18rem,0.9fr)]">
                <span>Model</span>
                <span class="text-left">
                  <span>Price</span>
                  <span class="mt-1 block text-[11px] font-medium normal-case tracking-normal text-emerald-600 dark:text-emerald-400">
                    {{ t('home.models.pricingNote') }}
                  </span>
                </span>
              </div>
              <article
                v-for="model in models"
                :key="model.name"
                class="group grid gap-4 border-b border-stone-200/80 p-5 transition last:border-b-0 hover:bg-emerald-500/[0.04] dark:border-[#1e1e1e] md:grid-cols-[minmax(16rem,1fr)_minmax(18rem,0.9fr)]"
              >
                <div class="min-w-0">
                  <div class="flex items-start gap-3">
                    <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-stone-200 bg-white text-xs font-bold text-emerald-600 dark:border-[#1e1e1e] dark:bg-[#050505] dark:text-emerald-400">
                      {{ model.initial }}
                    </span>
                    <div class="min-w-0">
                      <div class="flex flex-wrap items-center gap-2">
                        <h3 class="font-semibold">{{ model.name }}</h3>
                        <span
                          v-if="model.badge"
                          class="rounded-full bg-emerald-500/10 px-2.5 py-1 text-xs font-semibold text-emerald-600 dark:text-emerald-400"
                        >
                          {{ model.badge }}
                        </span>
                      </div>
                      <p class="mt-1 text-sm text-stone-500">{{ model.vendor }} · {{ model.format }}</p>
                    </div>
                  </div>
                  <p class="mt-4 leading-7 text-stone-500 dark:text-stone-400">
                    {{ model.description }}
                  </p>
                </div>
                <div class="grid grid-cols-2 gap-8 text-left text-sm">
                  <div>
                    <p class="text-xs text-stone-500">{{ t('home.models.input') }}</p>
                    <p class="mt-1 whitespace-nowrap font-semibold">{{ model.input }}</p>
                  </div>
                  <div>
                    <p class="text-xs text-stone-500">{{ t('home.models.output') }}</p>
                    <p class="mt-1 whitespace-nowrap font-semibold">{{ model.output }}</p>
                  </div>
                </div>
              </article>
            </div>
          </div>
        </div>
      </section>
      <section class="product-flow-block overflow-hidden py-14">
        <div class="mx-auto mb-8 max-w-6xl px-4">
          <div class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
            <div class="max-w-2xl">
              <p class="mb-3 text-xs font-bold uppercase tracking-[0.32em] text-emerald-500">
                Customer Voices
              </p>
              <h2 class="text-3xl font-bold tracking-tight md:text-4xl">
                {{ t('home.testimonials.title') }}
              </h2>
              <p class="mt-4 leading-7 text-stone-500 dark:text-stone-400">
                {{ t('home.testimonials.description') }}
              </p>
            </div>
            <div class="grid grid-cols-2 gap-3 text-sm">
              <div class="rounded-lg border border-stone-200/70 bg-white/55 px-4 py-3 backdrop-blur-sm dark:border-[#1e1e1e] dark:bg-[#101010]/55">
                <p class="text-stone-500">Teams</p>
                <p class="mt-1 text-xl font-bold">10+</p>
              </div>
              <div class="rounded-lg border border-stone-200/70 bg-white/55 px-4 py-3 backdrop-blur-sm dark:border-[#1e1e1e] dark:bg-[#101010]/55">
                <p class="text-stone-500">Use cases</p>
                <p class="mt-1 text-xl font-bold">6</p>
              </div>
            </div>
          </div>
        </div>

        <div class="testimonial-wall relative space-y-4">
          <div
            class="pointer-events-none absolute inset-y-0 left-0 z-10 w-16 bg-gradient-to-r from-stone-50 to-transparent dark:from-[#050505] md:w-32"
            aria-hidden="true"
          ></div>
          <div
            class="pointer-events-none absolute inset-y-0 right-0 z-10 w-16 bg-gradient-to-l from-stone-50 to-transparent dark:from-[#050505] md:w-32"
            aria-hidden="true"
          ></div>

          <div
            v-for="row in testimonialRows"
            :key="row.id"
            class="testimonial-marquee"
            :class="{ 'testimonial-marquee-reverse': row.reverse }"
            :style="{ '--marquee-duration': `${row.duration}s` }"
          >
            <div class="testimonial-track">
              <article
                v-for="(item, index) in row.items"
                :key="`${row.id}-${item.name}-${index}`"
                class="testimonial-card"
              >
                <Icon name="chatBubble" size="md" class="shrink-0 text-emerald-500" />
                <p class="testimonial-quote">{{ item.quote }}</p>
                <div class="flex shrink-0 items-center gap-3">
                  <div
                    class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-stone-200 text-sm font-bold text-stone-600 dark:bg-stone-800 dark:text-stone-300"
                  >
                    {{ item.initial }}
                  </div>
                  <div class="min-w-0">
                    <p class="truncate text-sm font-semibold text-stone-950 dark:text-white">
                      {{ item.name }}
                    </p>
                    <p class="mt-0.5 truncate text-xs text-stone-500">{{ item.role }}</p>
                  </div>
                </div>
              </article>
            </div>
          </div>
        </div>
      </section>

      <section id="faq" class="product-flow-block pt-14 pb-20">
        <div class="mx-auto grid max-w-6xl grid-cols-1 gap-10 px-4 lg:grid-cols-[0.62fr_1fr]">
          <div>
            <p class="mb-3 text-xs font-bold uppercase tracking-[0.32em] text-emerald-500">
              FAQ
            </p>
            <h2 class="text-3xl font-bold tracking-tight md:text-4xl">
              {{ t('home.faq.title') }}
            </h2>
            <p class="mt-5 max-w-md leading-7 text-stone-500 dark:text-stone-400">
              {{ t('home.faq.description') }}
            </p>
            <a
              :href="docUrl || '#quick-access'"
              :target="docUrl ? '_blank' : undefined"
              :rel="docUrl ? 'noopener noreferrer' : undefined"
              class="mt-8 inline-flex items-center rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-5 py-3 text-sm font-semibold text-emerald-600 transition hover:bg-emerald-500/15 dark:text-emerald-400"
            >
              {{ t('home.quickAccess.fullDoc') }}
              <Icon name="arrowRight" size="xs" class="ml-2" />
            </a>
          </div>

          <div class="tech-panel divide-y divide-stone-200/80 overflow-hidden dark:divide-[#1e1e1e]">
            <button
              v-for="(item, index) in faqs"
              :key="item.question"
              type="button"
              class="group w-full p-6 text-left transition hover:bg-emerald-500/[0.04] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/50"
              :aria-expanded="openFaqIndex === index"
              :aria-controls="`home-faq-panel-${index}`"
              @click="toggleFaq(index)"
            >
              <div class="grid items-start gap-4 md:grid-cols-[4rem_1fr_auto]">
                <span class="text-sm font-bold text-emerald-500">0{{ index + 1 }}</span>
                <div>
                  <h3 class="text-lg font-semibold">{{ item.question }}</h3>
                  <div
                    :id="`home-faq-panel-${index}`"
                    class="grid transition-all duration-300"
                    :class="openFaqIndex === index ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0'"
                  >
                    <div class="overflow-hidden">
                      <p class="mt-4 leading-7 text-stone-500 dark:text-stone-400">
                        {{ item.answer }}
                      </p>
                    </div>
                  </div>
                </div>
                <span
                  class="mt-0.5 flex h-8 w-8 items-center justify-center rounded-full border border-stone-200 text-stone-500 transition group-hover:border-emerald-500/40 group-hover:text-emerald-500 dark:border-[#1e1e1e]"
                  :class="{ 'rotate-45 border-emerald-500/40 text-emerald-500': openFaqIndex === index }"
                >
                  <Icon name="plus" size="sm" />
                </span>
              </div>
            </button>
          </div>
        </div>
      </section>
      </div>

      <section class="mx-auto max-w-6xl px-4 py-16 text-center">
        <h2 class="mb-6 text-3xl font-bold tracking-tight">{{ t('home.cta.title') }}</h2>
        <p class="mx-auto mb-8 max-w-2xl leading-7 text-stone-500 dark:text-stone-400">
          {{ t('home.cta.description') }}
        </p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/register'"
          class="inline-flex items-center justify-center rounded-lg bg-emerald-500 px-8 py-4 text-base font-bold text-black transition hover:bg-emerald-400 hover:shadow-lg hover:shadow-emerald-500/20"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.cta.button') }}
        </router-link>
      </section>
    </main>

    <footer class="border-t border-stone-200 bg-white py-12 dark:border-[#1e1e1e] dark:bg-[#050505]">
      <div class="mx-auto grid max-w-6xl grid-cols-1 gap-10 px-4 md:grid-cols-4">
        <div>
          <div class="mb-4 text-2xl font-bold text-emerald-500">{{ siteName }}</div>
          <p class="mb-6 text-sm leading-7 text-stone-500 dark:text-stone-400">
            {{ t('home.footer.tagline') }}
          </p>
          <div class="flex gap-3">
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-stone-200 text-stone-500 transition hover:border-emerald-500/40 hover:text-emerald-500 dark:border-[#1e1e1e] dark:text-stone-400"
            >
              <Icon name="book" size="sm" />
            </a>
          </div>
        </div>

        <div v-for="column in footerColumns" :key="column.title">
          <h4 class="mb-5 text-base font-semibold">{{ column.title }}</h4>
          <ul class="space-y-3 text-sm">
            <li v-for="link in column.links" :key="link.label">
              <a
                :href="link.href"
                class="text-stone-500 transition-colors hover:text-emerald-500 dark:text-stone-400"
              >
                {{ link.label }}
              </a>
            </li>
          </ul>
        </div>
      </div>
      <div class="mx-auto mt-10 max-w-6xl border-t border-stone-200 px-4 pt-8 text-center dark:border-[#1e1e1e]">
        <p class="text-sm text-stone-500">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
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

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()
type IconName = InstanceType<typeof Icon>['$props']['name']

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const contactInfo = computed(() => appStore.cachedPublicSettings?.contact_info || appStore.contactInfo || '')
const contactHref = computed(() => normalizeContactHref(contactInfo.value))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const activeCodeTab = ref('curl')
const openFaqIndex = ref(0)
const heroProviders = ['OpenAI', 'Anthropic', 'Google', 'DeepSeek', '...more']

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

const currentYear = computed(() => new Date().getFullYear())

const quickSteps = computed(() => [
  t('home.quickAccess.step1'),
  t('home.quickAccess.step2'),
  t('home.quickAccess.step3')
])

const quickStepDescriptions = computed(() => [
  t('home.quickAccess.stepDesc1'),
  t('home.quickAccess.stepDesc2'),
  t('home.quickAccess.stepDesc3')
])

const apiEndpointExample = computed(() => {
  const origin = typeof window === 'undefined' ? 'https://your-domain.example.com' : window.location.origin
  return `${origin.replace(/\/$/, '')}/v1/chat/completions`
})

const codeExamples = computed(() => [
  {
    id: 'curl',
    label: 'curl',
    code: `curl ${apiEndpointExample.value} \\
  -H "Authorization: Bearer sk_xxxxxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-5.5",
    "messages": [{"role":"user","content":"${t('home.quickAccess.samplePrompt')}"}]
  }'`
  },
  {
    id: 'python',
    label: 'Python',
    code: `import requests

url = "${apiEndpointExample.value}"
headers = {
    "Authorization": "Bearer sk_xxxxxxxxx",
    "Content-Type": "application/json"
}
data = {
    "model": "gpt-5.5",
    "messages": [{"role":"user","content":"${t('home.quickAccess.samplePrompt')}"}]
}

response = requests.post(url, headers=headers, json=data)
print(response.json())`
  },
  {
    id: 'javascript',
    label: 'JavaScript',
    code: `fetch("${apiEndpointExample.value}", {
  method: "POST",
  headers: {
    "Authorization": "Bearer sk_xxxxxxxxx",
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    "model": "gpt-5.5",
    "messages": [{"role":"user","content":"${t('home.quickAccess.samplePrompt')}"}]
  })
})
  .then(response => response.json())
  .then(data => console.log(data))`
  }
])

const activeCodeExample = computed(() => {
  return codeExamples.value.find((example) => example.id === activeCodeTab.value) || codeExamples.value[0]
})

const advantages = computed<Array<{ icon: IconName; title: string; description: string }>>(() => [
  {
    icon: 'bolt',
    title: t('home.advantages.fast'),
    description: t('home.advantages.fastDesc')
  },
  {
    icon: 'link',
    title: t('home.advantages.unified'),
    description: t('home.advantages.unifiedDesc')
  },
  {
    icon: 'shield',
    title: t('home.advantages.safe'),
    description: t('home.advantages.safeDesc')
  }
])

const modelMetrics = computed(() => [
  {
    label: 'Formats',
    value: '3+',
    hint: 'OpenAI / Claude / Gemini'
  },
  {
    label: 'Access',
    value: 'One',
    hint: t('home.models.metricAccess')
  },
  {
    label: 'Usage',
    value: 'Clear',
    hint: t('home.models.metricUsage')
  },
  {
    label: 'Models',
    value: 'Global',
    hint: t('home.models.metricModels')
  }
])

const models = computed(() => [
  {
    initial: 'O',
    name: 'GPT-5.5',
    vendor: 'OpenAI',
    format: 'Responses / Chat',
    badge: t('home.models.hot'),
    description: t('home.models.gptDesc'),
    input: '$5/M input tokens',
    output: '$30/M output tokens'
  },
  {
    initial: 'A',
    name: 'Claude Opus 4.7',
    vendor: 'Anthropic',
    format: 'Messages',
    badge: '',
    description: t('home.models.claudeDesc'),
    input: '$5/M input tokens',
    output: '$25/M output tokens'
  },
  {
    initial: 'G',
    name: 'Gemini 3.1 Pro',
    vendor: 'Google',
    format: 'v1beta',
    badge: '',
    description: t('home.models.geminiDesc'),
    input: '$2/M input tokens',
    output: '$12/M output tokens'
  }
])

const testimonials = computed(() => [
  {
    initial: t('home.testimonials.first.initial'),
    quote: t('home.testimonials.first.quote'),
    name: t('home.testimonials.first.name'),
    role: t('home.testimonials.first.role')
  },
  {
    initial: t('home.testimonials.second.initial'),
    quote: t('home.testimonials.second.quote'),
    name: t('home.testimonials.second.name'),
    role: t('home.testimonials.second.role')
  },
  {
    initial: t('home.testimonials.third.initial'),
    quote: t('home.testimonials.third.quote'),
    name: t('home.testimonials.third.name'),
    role: t('home.testimonials.third.role')
  },
  {
    initial: t('home.testimonials.fourth.initial'),
    quote: t('home.testimonials.fourth.quote'),
    name: t('home.testimonials.fourth.name'),
    role: t('home.testimonials.fourth.role')
  },
  {
    initial: t('home.testimonials.fifth.initial'),
    quote: t('home.testimonials.fifth.quote'),
    name: t('home.testimonials.fifth.name'),
    role: t('home.testimonials.fifth.role')
  },
  {
    initial: t('home.testimonials.sixth.initial'),
    quote: t('home.testimonials.sixth.quote'),
    name: t('home.testimonials.sixth.name'),
    role: t('home.testimonials.sixth.role')
  },
  {
    initial: t('home.testimonials.seventh.initial'),
    quote: t('home.testimonials.seventh.quote'),
    name: t('home.testimonials.seventh.name'),
    role: t('home.testimonials.seventh.role')
  },
  {
    initial: t('home.testimonials.eighth.initial'),
    quote: t('home.testimonials.eighth.quote'),
    name: t('home.testimonials.eighth.name'),
    role: t('home.testimonials.eighth.role')
  },
  {
    initial: t('home.testimonials.ninth.initial'),
    quote: t('home.testimonials.ninth.quote'),
    name: t('home.testimonials.ninth.name'),
    role: t('home.testimonials.ninth.role')
  },
  {
    initial: t('home.testimonials.tenth.initial'),
    quote: t('home.testimonials.tenth.quote'),
    name: t('home.testimonials.tenth.name'),
    role: t('home.testimonials.tenth.role')
  }
])

const testimonialRows = computed(() => {
  const base = testimonials.value
  const rows = [
    [base[0], base[3], base[6], base[8]],
    [base[1], base[4], base[7], base[2]],
    [base[5], base[2], base[8], base[3]]
  ]

  return rows.map((items, index) => ({
    id: `testimonial-row-${index + 1}`,
    reverse: index % 2 === 1,
    duration: 34 + index * 6,
    items: [...items, ...items, ...items]
  }))
})

const faqs = computed(() => [
  {
    question: t('home.faq.q1'),
    answer: t('home.faq.a1')
  },
  {
    question: t('home.faq.q2'),
    answer: t('home.faq.a2')
  },
  {
    question: t('home.faq.q3'),
    answer: t('home.faq.a3')
  }
])

const footerColumns = computed(() => {
  const supportLinks = [
    { label: t('home.footer.quickAccess'), href: '#quick-access' },
    { label: t('home.footer.faq'), href: '#faq' },
    { label: t('home.footer.apiDocs'), href: docUrl.value || '#quick-access' }
  ]
  const businessLinks = [
    { label: t('home.footer.enterprise'), href: '#advantages' },
    { label: t('home.footer.partner'), href: contactHref.value || '#advantages' }
  ]

  if (contactHref.value) {
    supportLinks.push({ label: t('home.footer.contact'), href: contactHref.value })
  }

  return [
    {
      title: t('home.footer.navTitle'),
      links: [
        { label: t('home.nav.home'), href: '#top' },
        { label: t('home.nav.model'), href: '#models' },
        { label: t('home.nav.document'), href: docUrl.value || '#quick-access' }
      ]
    },
    {
      title: t('home.footer.supportTitle'),
      links: supportLinks
    },
    {
      title: t('home.footer.businessTitle'),
      links: businessLinks
    }
  ]
})

function normalizeContactHref(value: string) {
  const contact = value.trim()
  if (!contact) return ''
  if (/^(https?:|mailto:|tel:)/i.test(contact)) return contact
  if (/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(contact)) return `mailto:${contact}`

  const phone = contact.replace(/[\s()-]/g, '')
  if (/^\+?\d{6,}$/.test(phone)) return `tel:${phone}`

  return ''
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function toggleFaq(index: number) {
  openFaqIndex.value = openFaqIndex.value === index ? -1 : index
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  } else {
    isDark.value = false
    document.documentElement.classList.remove('dark')
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
.product-flow {
  position: relative;
  overflow: hidden;
  background:
    radial-gradient(circle at 18% 5%, rgba(16, 185, 129, 0.14), transparent 34rem),
    radial-gradient(circle at 84% 30%, rgba(16, 185, 129, 0.075), transparent 32rem),
    linear-gradient(180deg, #fafaf9 0%, #f5f5f4 22rem, rgba(245, 245, 244, 0.98) 58%, rgba(250, 250, 249, 0.98));
}

.dark .product-flow {
  background:
    radial-gradient(circle at 18% 5%, rgba(16, 185, 129, 0.13), transparent 34rem),
    radial-gradient(circle at 84% 30%, rgba(16, 185, 129, 0.08), transparent 32rem),
    linear-gradient(180deg, #050505 0%, #07130d 22rem, #050505 34rem, #070707 58%, #050505);
}

.product-flow::before {
  position: absolute;
  inset: 0;
  content: '';
  background-image:
    linear-gradient(rgba(16, 185, 129, 0.065) 1px, transparent 1px),
    linear-gradient(90deg, rgba(16, 185, 129, 0.055) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: linear-gradient(180deg, transparent 0%, black 6%, black 94%, transparent 100%);
  pointer-events: none;
}

.product-flow-block {
  position: relative;
}

.tech-panel {
  position: relative;
  border: 1px solid rgba(231, 229, 228, 0.9);
  border-radius: 0.75rem;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 20px 70px rgba(15, 23, 42, 0.08);
  backdrop-filter: blur(18px);
}

.dark .tech-panel {
  border-color: #1e1e1e;
  background: rgba(16, 16, 16, 0.78);
  box-shadow:
    0 18px 60px rgba(0, 0, 0, 0.34),
    inset 0 1px 0 rgba(255, 255, 255, 0.035);
}

.testimonial-wall {
  margin-inline: auto;
  max-width: min(100vw, 1280px);
}

.testimonial-marquee {
  display: flex;
  overflow: hidden;
}

.testimonial-track {
  display: flex;
  min-width: max-content;
  gap: 1rem;
  animation: testimonial-scroll var(--marquee-duration, 38s) linear infinite;
  will-change: transform;
}

.testimonial-marquee-reverse .testimonial-track {
  animation-direction: reverse;
}

.testimonial-marquee:hover .testimonial-track {
  animation-play-state: paused;
}

.testimonial-card {
  display: flex;
  width: clamp(20rem, 38vw, 31rem);
  min-height: 8.25rem;
  align-items: center;
  gap: 1rem;
  border: 1px solid rgb(231 229 228);
  border-radius: 0.5rem;
  background: rgba(255, 255, 255, 0.82);
  padding: 1.125rem 1.25rem;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.06);
}

.dark .testimonial-card {
  border-color: #1e1e1e;
  background: rgba(16, 16, 16, 0.9);
  box-shadow: 0 18px 44px rgba(0, 0, 0, 0.22);
}

.testimonial-quote {
  display: -webkit-box;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  color: rgb(87 83 78);
  font-size: 0.9375rem;
  line-height: 1.75;
}

.dark .testimonial-quote {
  color: rgb(214 211 209);
}

@keyframes testimonial-scroll {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(calc(-100% / 3));
  }
}

@media (prefers-reduced-motion: reduce) {
  .testimonial-marquee {
    overflow-x: auto;
  }

  .testimonial-track {
    animation: none;
  }
}
</style>
