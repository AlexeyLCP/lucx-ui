<!--
Copyright (c) 2025 LucX-UI Project.
Licensed under the PolyForm Noncommercial License 1.0.0.
LucX-UI Component. Free for personal and educational use.
Commercial use (including VPN resale) requires explicit written permission from the author.
SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
-->

<template>
  <div class="telemt-form">
    <a-form-item label="Port">
      <a-input-number v-model:value="form.port" :min="1" :max="65535" style="width: 100%" />
    </a-form-item>

    <a-form-item label="TLS Domain (SNI front)">
      <a-input v-model:value="form.tlsDomain" placeholder="gosuslugi.ru" />
    </a-form-item>

    <a-form-item label="Log Level">
      <a-select v-model:value="form.logLevel">
        <a-select-option value="normal">Normal</a-select-option>
        <a-select-option value="debug">Debug</a-select-option>
        <a-select-option value="silent">Silent</a-select-option>
      </a-select>
    </a-form-item>

    <a-divider>Client</a-divider>

    <a-form-item label="Client Name">
      <a-input v-model:value="form.clientName" placeholder="myphone" />
    </a-form-item>

    <a-form-item label="Secret (FakeTLS)">
      <a-input-group>
        <a-input v-model:value="form.clientSecret" :readonly="true" placeholder="ee..." />
        <a-button type="primary" @click="generateSecret">Generate</a-button>
      </a-input-group>
    </a-form-item>
  </div>
</template>

<script setup>
import { reactive, watch } from 'vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({}) }
})
const emit = defineEmits(['update:modelValue'])

const form = reactive({
  port: props.modelValue.port || 443,
  tlsDomain: props.modelValue.tlsDomain || 'gosuslugi.ru',
  logLevel: props.modelValue.logLevel || 'normal',
  clientName: props.modelValue.clientName || '',
  clientSecret: props.modelValue.clientSecret || '',
})

// Sync external changes (presets) into form
watch(() => props.modelValue, (val) => {
  if (!val) return
  if (val.port !== undefined) form.port = val.port
  if (val.tlsDomain !== undefined) form.tlsDomain = val.tlsDomain
  if (val.logLevel !== undefined) form.logLevel = val.logLevel
}, { deep: true })

watch(form, (val) => emit('update:modelValue', { ...val }), { deep: true })

function generateSecret() {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('')
  form.clientSecret = 'ee' + hex
}
</script>
