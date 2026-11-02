<template>
  <ShopLayout admin>
    <CDKShopSettings />
    <details class="shop-legacy-settings">
      <summary>人工代购与外部渠道设置</summary>
      <p class="shop-caption">以下设置用于原有人工代购、外站入口和站点首页宣传，独立于兑换码商城的营业开关。</p>
      <ShopAvailabilityControl />
    </details>
  </ShopLayout>
</template>

<script setup lang="ts">
import ShopLayout from './ShopLayout.vue'
import CDKShopSettings from './CDKShopSettings.vue'
import ShopAvailabilityControl from './ShopAvailabilityControl.vue'
</script>
