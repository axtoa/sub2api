<template>
  <AppLayout v-if="embedded">
    <div class="best-shop"><div class="shop-main">
        <template v-if="!storefront">
        <div class="shop-heading"><h1>{{ t(admin ? 'shop.adminTitle' : 'shop.legacyOrdersTitle') }}</h1><p v-if="admin">{{ t('shop.adminIntro') }}</p></div>
        <nav class="shop-tabs" :aria-label="t('shop.entry')">
          <template v-if="admin">
            <router-link to="/admin/shop" :class="{ active: route.path === '/admin/shop' || route.path.includes('/orders/') }">{{ t('shop.allOrders') }}</router-link>
            <router-link to="/admin/shop/products" :class="{ active: route.path.includes('/products') }">{{ t('shop.products') }}</router-link>
            <router-link to="/admin/shop/cdk" :class="{ active: route.path.includes('/cdk') }">CDK 商品与库存</router-link>
            <router-link to="/admin/shop/settings" :class="{ active: route.path.includes('/settings') }">{{ t('shop.settingsTab') }}</router-link>
            <router-link to="/admin/shop/homepage" :class="{ active: route.path.includes('/homepage') }">{{ t('shop.homepageTab') }}</router-link>
          </template>
          <template v-else>
            <router-link to="/shop" target="_blank" rel="noopener noreferrer">{{ t('shop.storefrontTitle') }}</router-link>
            <router-link to="/shop/orders" :class="{ active: route.path.includes('/orders') }">{{ t('shop.legacyOrdersTitle') }}</router-link>
          </template>
        </nav>
        </template>
        <slot />
    </div></div>
  </AppLayout>
  <div v-else class="best-shop cdk-public-shop">
    <header class="cdk-public-header">
      <router-link class="cdk-public-brand" to="/shop" aria-label="Best AI 商城">
        <img class="cdk-public-logo" :src="resolvedSiteLogo" alt="Best AI" />
        <span class="cdk-public-site-name">Best AI</span>
      </router-link>
      <nav class="cdk-public-nav" :aria-label="t('shop.storefrontNavigation')">
        <router-link to="/shop">{{ t('shop.storefrontAllProducts') }}</router-link>
        <a href="/shop?lookup=1#shop-order-lookup">{{ t('shop.storefrontLookup') }}</a>
        <router-link v-if="!auth.isAuthenticated && route.name !== 'ShopLogin'" class="cdk-public-login" :to="loginLink"><span>{{ t('home.login') }}</span></router-link>
        <span v-else-if="auth.isAuthenticated" class="cdk-public-account">{{ t('shop.storefrontSignedIn') }}</span>
      </nav>
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
import { useAuthStore } from '@/stores/auth'
import { sanitizeUrl } from '@/utils/url'
import { storefrontLoginReturnTo } from './storefront'
import './shop.css'
const props = withDefaults(defineProps<{ admin?: boolean; embedded?: boolean; storefront?: boolean; siteLogo?: string; loginReturnTo?: string }>(), { embedded: true })
const { t } = useI18n()
const route = useRoute()
const app = useAppStore()
const auth = useAuthStore()
const loginLink = computed(() => ({ path: '/shop/login', query: { redirect: storefrontLoginReturnTo(props.loginReturnTo || route.fullPath) } }))
const resolvedSiteLogo = computed(() => sanitizeUrl(props.siteLogo || app.cachedPublicSettings?.site_logo || app.siteLogo || '', { allowRelative: true, allowDataUrl: true }) || '/logo.svg')
</script>
