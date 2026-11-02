<template>
  <ShopLayout :embedded="false" storefront>
    <main class="cdk-shop-login">
      <router-link class="shop-text-link" :to="returnTo"><Icon name="arrowLeft" size="sm" />{{ t('shop.storefrontBackToShopping') }}</router-link>
      <section class="cdk-shop-login-card"><slot /></section>
    </main>
  </ShopLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ShopLayout from './ShopLayout.vue'
defineProps<{ returnTo: string }>()
const { t } = useI18n()
</script>
