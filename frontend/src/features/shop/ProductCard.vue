<template>
  <article class="shop-product" :class="{ selected, 'shop-product-compact': compact }">
    <div class="shop-product-top"><span class="shop-brand-label">{{ product.brand }}</span><Icon v-if="selected" name="check" size="sm" /></div>
    <h3>{{ product.name }}</h3>
    <strong class="shop-price">{{ product.price_cents === null || !checkoutLink(product.payment_url) ? t('shop.upcoming') : shopMoney(product.price_cents) }}</strong>
    <p class="shop-muted">{{ product.spec }}</p>
    <p v-if="product.reference_amount !== null" class="shop-reference">{{ t('shop.reference') }} {{ product.reference_currency }} {{ product.reference_amount }}</p>
    <p v-else-if="compact && product.description" class="shop-reference shop-product-summary">{{ product.description }}</p>
    <slot />
  </article>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { checkoutLink, shopMoney, type ShopProduct } from './api'
defineProps<{ product: ShopProduct; selected?: boolean; compact?: boolean }>()
const { t } = useI18n()
</script>
