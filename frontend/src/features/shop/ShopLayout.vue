<template>
  <AppLayout>
    <div class="best-shop">
      <div class="shop-main">
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
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import './shop.css'
defineProps<{ admin?: boolean }>()
const { t } = useI18n()
const route = useRoute()
</script>
