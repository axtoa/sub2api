<template>
  <div class="ledger-form-grid">
    <label>{{ t('ledger.resourceName') }}<input v-model="draft.name" required maxlength="100" :placeholder="t('ledger.resourceExample')" /></label>
    <label>{{ t('ledger.resourceKind') }}<select v-model="draft.kind"><option v-for="kind in ['domain','server','other']" :key="kind" :value="kind">{{ t(`ledger.${kind}`) }}</option></select></label>
    <label>{{ t('ledger.provider') }}<input v-model="draft.provider" maxlength="100" /></label>
    <label>{{ t('ledger.expiresAt') }}<input v-model="draft.expiry" type="datetime-local" min="1900-01-01T00:00" max="2200-12-31T23:59" required /></label>
    <label>{{ t('ledger.estimate') }}<input v-model="draft.renewal_amount" inputmode="decimal" pattern="(0|[1-9][0-9]{0,8})(\.[0-9]{1,6})?" :placeholder="t('ledger.optional')" /></label>
    <label>{{ t('ledger.currency') }}<select v-model="draft.currency"><option v-for="currency in currencies" :key="currency">{{ currency }}</option></select></label>
    <label class="ledger-span">{{ t('ledger.managementURL') }}<input v-model="draft.management_url" type="url" pattern="https://.*" maxlength="2000" placeholder="https://" /></label>
    <label class="ledger-span">{{ t('ledger.note') }}<textarea v-model="draft.note" rows="2" maxlength="2000" /></label>
  </div>
  <p class="ledger-muted">{{ t('ledger.localTimeHint', { timezone }) }}</p>
  <label class="ledger-check"><input v-model="draft.active" type="checkbox" />{{ t('ledger.keepRenewing') }}</label>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { currencies } from './display'
export interface ResourceDraft { name: string; kind: 'domain' | 'server' | 'other'; provider: string; expiry: string; originalExpiry?: string; renewal_amount: string; currency: string; management_url: string; note: string; active: boolean; version: number }
defineProps<{ draft: ResourceDraft }>()
const { t } = useI18n()
const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
</script>
