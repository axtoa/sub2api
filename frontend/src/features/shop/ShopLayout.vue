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
      <router-link class="cdk-public-brand" to="/shop" :aria-label="resolvedSiteName">
        <span class="cdk-public-logo"><img v-if="siteLogo" :src="resolvedSiteLogo" alt="" /><Icon v-else name="sparkles" size="md" aria-hidden="true" /></span>
        <span class="cdk-public-site-name">{{ resolvedSiteName }}</span>
      </router-link>
      <nav class="cdk-public-nav" :aria-label="t('shop.storefrontNavigation')">
        <router-link to="/shop">{{ t('shop.storefrontAllProducts') }}</router-link>
        <a href="/shop?lookup=1#shop-order-lookup">{{ t('shop.storefrontLookup') }}</a>
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
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { sanitizeUrl } from '@/utils/url'
import './shop.css'
const props = withDefaults(defineProps<{ admin?: boolean; embedded?: boolean; storefront?: boolean; siteName?: string; siteLogo?: string }>(), { embedded: true })
const { t } = useI18n()
const route = useRoute()
const app = useAppStore()
const resolvedSiteName = computed(() => props.siteName || app.cachedPublicSettings?.official_shop_name?.trim() || t('shop.storefrontTitle'))
const resolvedSiteLogo = computed(() => sanitizeUrl(props.siteLogo || app.cachedPublicSettings?.site_logo || app.siteLogo || '', { allowRelative: true, allowDataUrl: true }) || '/logo.svg')
</script>
