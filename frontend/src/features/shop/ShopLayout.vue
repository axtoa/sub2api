<template>
  <AppLayout v-if="embedded">
    <div class="best-shop"><div class="shop-main">
        <template v-if="!storefront">
        <div class="shop-heading"><h1>{{ t(admin ? 'shop.adminTitle' : 'shop.title') }}</h1><p>{{ t(admin ? 'shop.adminIntro' : 'shop.intro') }}</p></div>
        <nav class="shop-tabs" :aria-label="t('shop.entry')">
          <template v-if="admin">
            <router-link to="/admin/shop" :class="{ active: route.path === '/admin/shop' || route.path.includes('/orders/') }">{{ t('shop.allOrders') }}</router-link>
            <router-link to="/admin/shop/products" :class="{ active: route.path.includes('/products') }">{{ t('shop.products') }}</router-link>
            <router-link to="/admin/shop/cdk" :class="{ active: route.path.includes('/cdk') }">CDK 商品与库存</router-link>
            <router-link to="/admin/shop/settings" :class="{ active: route.path.includes('/settings') }">{{ t('shop.settingsTab') }}</router-link>
            <router-link to="/admin/shop/homepage" :class="{ active: route.path.includes('/homepage') }">{{ t('shop.homepageTab') }}</router-link>
          </template>
          <template v-else>
            <router-link to="/shop" :class="{ active: !route.path.includes('/orders') }">{{ t('shop.catalog') }}</router-link>
            <router-link to="/shop/orders" :class="{ active: route.path.includes('/orders') }">{{ t('shop.myOrders') }}</router-link>
          </template>
        </nav>
        </template>
        <slot />
    </div></div>
  </AppLayout>
  <div v-else class="best-shop cdk-public-shop">
    <header class="cdk-public-header">
      <a class="cdk-public-brand" href="/" :aria-label="resolvedSiteName">
        <span class="cdk-public-logo shadow-glow"><img :src="resolvedSiteLogo" alt="" /></span>
        <span class="cdk-public-site-name text-gray-900 transition-colors hover:text-primary-600 dark:text-white dark:hover:text-primary-400">{{ resolvedSiteName }}</span>
      </a>
      <router-link class="cdk-public-login" :to="{ path: '/login', query: { redirect: '/shop' } }">登录</router-link>
    </header>
    <div class="shop-main"><slot /></div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { sanitizeUrl } from '@/utils/url'
import './shop.css'
const props = withDefaults(defineProps<{ admin?: boolean; embedded?: boolean; storefront?: boolean; siteName?: string; siteLogo?: string }>(), { embedded: true })
const { t } = useI18n()
const route = useRoute()
const app = useAppStore()
const resolvedSiteName = computed(() => props.siteName || app.cachedPublicSettings?.site_name || app.siteName || 'Best AI')
const resolvedSiteLogo = computed(() => sanitizeUrl(props.siteLogo || app.cachedPublicSettings?.site_logo || app.siteLogo || '', { allowRelative: true, allowDataUrl: true }) || '/logo.svg')
</script>
