<template>
  <section class="cdk-settings-module">
    <div class="shop-section-heading"><div><h2>兑换码商城设置</h2><p>控制商城营业与控制台入口，可分别保存。</p></div><router-link class="shop-secondary" to="/shop">预览商城</router-link></div>
      <div class="cdk-admin-switches">
        <section class="cdk-admin-switch"><div><h3>商城营业</h3><p>关闭后暂停新下单和重新发起支付，已有订单查询及支付回调继续处理。</p></div><Toggle :model-value="storeEnabled" :disabled="settingsBusy || !storeSettingsLoaded" aria-label="商城营业" @update:model-value="saveStoreEnabled" /></section>
        <section class="cdk-admin-switch"><div><h3>控制台入口仅管理员可见</h3><p>开启后普通用户隐藏入口，管理员保留入口便于调试。独立商城仍可通过 /shop 访问，能否购买由营业开关控制。</p></div><Toggle :model-value="!menuVisible" :disabled="settingsBusy || !menuSettingsLoaded" aria-label="控制台入口仅管理员可见" @update:model-value="enabled => saveMenuVisible(!enabled)" /></section>
      </div>
    <p v-if="message" class="shop-caption" role="status">{{ message }}</p>
    <p v-if="error" class="shop-error" role="alert">{{ error }} <button class="shop-plain" type="button" :disabled="settingsBusy" @click="load">重新加载</button></p>
  </section>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/api/client'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
const appStore=useAppStore()
const storeEnabled=ref(false),menuVisible=ref(false),settingsBusy=ref(false),storeSettingsLoaded=ref(false),menuSettingsLoaded=ref(false),message=ref(''),error=ref('')
async function call(path:string,body?:unknown,method=body===undefined?'GET':'POST'){
  const token=(await apiClient.post<{token:string}>('/auth/cdkshop-token')).data.token
  const res=await fetch(`/api/v1/cdkshop${path}`,{method,headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)})
  const data=await res.json()
  if(!res.ok)throw new Error(data.error||'请求失败')
  return data
}
async function load(){
  if(settingsBusy.value)return
  settingsBusy.value=true;error.value=''
  const results=await Promise.allSettled([
    call('/admin/settings').then(data=>{storeEnabled.value=data.enabled;storeSettingsLoaded.value=true}),
    apiClient.get<{enabled:boolean}>('/admin/shop/settings/cdk-menu').then(response=>{menuVisible.value=response.data.enabled;menuSettingsLoaded.value=true}),
  ])
  const failure=results.find(result=>result.status==='rejected')
  if(failure?.status==='rejected')error.value=(failure.reason as Error).message
  settingsBusy.value=false
}
async function saveStoreEnabled(enabled:boolean){if(settingsBusy.value||!storeSettingsLoaded.value)return;settingsBusy.value=true;error.value='';message.value='';try{const data=await call('/admin/settings',{enabled},'PUT');storeEnabled.value=data.enabled;message.value=enabled?'商城已开放':'商城已暂停'}catch(e){error.value=(e as Error).message}finally{settingsBusy.value=false}}
async function saveMenuVisible(enabled:boolean){if(settingsBusy.value||!menuSettingsLoaded.value)return;settingsBusy.value=true;error.value='';message.value='';try{const response=await apiClient.put<{enabled:boolean}>('/admin/shop/settings/cdk-menu',{enabled});menuVisible.value=response.data.enabled;await appStore.fetchPublicSettings(true);message.value=enabled?'普通用户菜单已显示':'控制台入口已设为仅管理员可见'}catch(e){error.value=(e as Error).message}finally{settingsBusy.value=false}}
onMounted(load)
</script>
