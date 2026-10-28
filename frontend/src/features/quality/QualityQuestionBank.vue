<template>
  <details class="quality-question-bank">
    <summary>{{ t('quality.questionBank') }} <span class="quality-caption">{{ modelValue.length ? t('quality.customQuestions', {count:modelValue.length}) : t('quality.builtinQuestions') }}</span></summary>
    <p class="quality-caption">{{ t('quality.questionBankHint') }}</p>
    <div v-for="(question,index) in modelValue" :key="question.id" class="quality-template">
      <div class="quality-card-heading"><strong>{{ t('quality.questionNumber',{count:index+1}) }}</strong><button type="button" class="quality-text-button" @click="emit('update:modelValue',modelValue.filter((_,i)=>i!==index))">{{ t('quality.removeQuestion') }}</button></div>
      <div class="quality-form-grid"><label>{{ t('quality.questionName') }}<input :value="question.name" maxlength="40" required @input="update(index,'name',($event.target as HTMLInputElement).value)" /></label><label>{{ t('quality.questionModel') }}<input :value="question.model" maxlength="100" :placeholder="t('quality.allTextModels')" @input="update(index,'model',($event.target as HTMLInputElement).value)" /></label></div>
      <label>{{ t('quality.question') }}<textarea :value="question.prompt" rows="4" maxlength="2000" required @input="update(index,'prompt',($event.target as HTMLTextAreaElement).value)" /></label>
      <label>{{ t('quality.integerAnswer') }}<input :value="question.expected" type="text" inputmode="numeric" pattern="[+-]?[0-9]+" maxlength="19" required @input="update(index,'expected',($event.target as HTMLInputElement).value)" /></label>
    </div>
    <button type="button" class="quality-button" :disabled="modelValue.length>=12" @click="add">{{ t('quality.addQuestion') }}</button>
  </details>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { QuestionTemplate } from './api'
const props=defineProps<{modelValue:QuestionTemplate[]}>()
const emit=defineEmits<{'update:modelValue':[value:QuestionTemplate[]]}>()
const {t}=useI18n()
function update(index:number,key:keyof QuestionTemplate,value:string){emit('update:modelValue',props.modelValue.map((q,i)=>i===index?{...q,[key]:value}:q))}
function add(){emit('update:modelValue',[...props.modelValue,{id:crypto.randomUUID(),name:'',model:'',prompt:'',expected:''}])}
</script>
