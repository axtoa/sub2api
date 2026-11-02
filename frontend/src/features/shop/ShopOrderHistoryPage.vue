<template>
  <ShopLayout>
    <p class="shop-caption">{{ t('shop.legacyOrdersHint') }}</p>
    <ShopOrders />
  </ShopLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import ShopLayout from './ShopLayout.vue'
import ShopOrders from './ShopOrders.vue'
const { t } = useI18n()
</script>
