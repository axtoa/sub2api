<template>
  <div class="best-shop">
    <header class="shop-header">
      <router-link to="/home" class="shop-brand"><img :src="logo || '/logo.svg'" alt="Best AI" /><strong>Best AI</strong></router-link>
      <router-link :to="auth.isAuthenticated ? (auth.isAdmin ? '/admin/dashboard' : '/dashboard') : '/home'" class="shop-back"><Icon name="arrowLeft" size="sm" />{{ t('shop.back') }}</router-link>
    </header>
    <main class="shop-main">
      <div class="shop-heading"><h1>{{ t(admin ? 'shop.adminTitle' : 'shop.title') }}</h1><p>{{ t(admin ? 'shop.adminIntro' : 'shop.intro') }}</p></div>
      <nav class="shop-tabs" :aria-label="t('shop.entry')">
        <template v-if="admin">
          <router-link to="/admin/shop" :class="{ active: !route.path.includes('/products') }">{{ t('shop.allOrders') }}</router-link>
          <router-link to="/admin/shop/products" :class="{ active: route.path.includes('/products') }">{{ t('shop.products') }}</router-link>
        </template>
        <template v-else>
          <router-link to="/shop" :class="{ active: !route.path.includes('/orders') }">{{ t('shop.catalog') }}</router-link>
          <router-link to="/shop/orders" :class="{ active: route.path.includes('/orders') }">{{ t('shop.myOrders') }}</router-link>
        </template>
      </nav>
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import './shop.css'
defineProps<{ admin?: boolean }>()
const { t } = useI18n()
const route = useRoute()
const app = useAppStore()
const auth = useAuthStore()
const logo = computed(() => sanitizeUrl(app.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
</script>
