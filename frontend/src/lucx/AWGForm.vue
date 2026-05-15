<!--
Copyright (c) 2025 LucX-UI Project.
Licensed under the PolyForm Noncommercial License 1.0.0.
LucX-UI Component. Free for personal and educational use.
Commercial use (including VPN resale) requires explicit written permission from the author.
SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
-->

<template>
  <div class="awg-form">
    <a-form-item label="Port" :rules="[{ required: true, message: 'Port is required' }]">
      <a-input-number v-model:value="form.port" :min="1024" :max="65535" style="width: 100%" placeholder="Random port" />
    </a-form-item>

    <a-form-item label="Obfuscation Level">
      <a-radio-group v-model:value="form.obfLevel">
        <a-radio :value="1">Basic (compatibility)</a-radio>
        <a-radio :value="2">I1 (1 CPS packet)</a-radio>
        <a-radio :value="3">I1-I5 (full CPS chain)</a-radio>
      </a-radio-group>
    </a-form-item>

    <a-form-item label="Mimicry Profile">
      <a-select v-model:value="form.mimicryProfile">
        <a-select-option value="quic">QUIC (recommended)</a-select-option>
        <a-select-option value="sip">SIP</a-select-option>
        <a-select-option value="dns">DNS</a-select-option>
      </a-select>
    </a-form-item>

    <a-form-item label="Region">
      <a-select v-model:value="form.region">
        <a-select-option value="ru">RU</a-select-option>
        <a-select-option value="world">WORLD</a-select-option>
      </a-select>
    </a-form-item>

    <a-form-item label="DNS">
      <a-select v-model:value="form.dns">
        <a-select-option value="1.1.1.1">Cloudflare (1.1.1.1)</a-select-option>
        <a-select-option value="8.8.8.8">Google (8.8.8.8)</a-select-option>
        <a-select-option value="94.140.14.14">AdGuard (94.140.14.14)</a-select-option>
      </a-select>
    </a-form-item>

    <a-form-item label="MTU">
      <a-input-number v-model:value="form.mtu" :min="1000" :max="1500" style="width: 100%" />
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
  port: props.modelValue.port || 0,
  obfLevel: props.modelValue.obfLevel || 1,
  mimicryProfile: props.modelValue.mimicryProfile || 'quic',
  region: props.modelValue.region || 'ru',
  dns: props.modelValue.dns || '1.1.1.1',
  mtu: props.modelValue.mtu || 1320,
})

watch(form, (val) => emit('update:modelValue', val), { deep: true })
</script>
