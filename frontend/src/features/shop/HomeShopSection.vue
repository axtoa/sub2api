<template>
  <section class="home-shop best-shop" aria-labelledby="home-shop-title">
    <div class="shop-section-heading"><div><h2 id="home-shop-title">{{ t('shop.homepageTitle') }}</h2><p>{{ t('home.subscriptionTeaser') }}</p></div><router-link :to="entryTo()" target="_blank" rel="noopener noreferrer" class="shop-text-link" @click="trackEntry" @auxclick="trackEntry">{{ t('shop.viewAll') }}<Icon name="arrowRight" size="sm" /></router-link></div>
    <div v-if="products.length" class="home-shop-grid">
      <router-link v-for="product in products.slice(0, 5)" :key="product.id" :to="entryTo(product.id)" target="_blank" rel="noopener noreferrer" class="home-shop-link" @click="trackEntry" @auxclick="trackEntry">
        <ProductCard :product="product" compact />
      </router-link>
    </div>
    <div v-else class="shop-empty"><span>{{ loading ? t('common.loading') : error ? t('shop.loadError') : t('shop.empty') }}</span><button v-if="error" type="button" class="shop-text-link" @click="load">{{ t('shop.retry') }}</button></div>
  </section>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ProductCard from './ProductCard.vue'
import { shopAPI, type ShopProduct } from './api'
import { isShopDisabled, useShopAvailability } from './availability'
import { useHomeShopEntry } from './useHomeShopEntry'
import './shop.css'
const { t } = useI18n()
const { apply, enabled } = useShopAvailability()
const { entryTo, trackEntry } = useHomeShopEntry(enabled)
const products = ref<ShopProduct[]>([])
const loading = ref(true)
const error = ref(false)
async function load() {
  loading.value = true; error.value = false
  try { products.value = await shopAPI.products() } catch (e) {
    products.value = []
    if (isShopDisabled(e)) apply(false)
    else error.value = true
  } finally { loading.value = false }
}
onMounted(load)
</script>
