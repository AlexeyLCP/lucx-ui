<!--
Copyright (c) 2025 LucX-UI Project.
Licensed under the PolyForm Noncommercial License 1.0.0.
LucX-UI Component. Free for personal and educational use.
Commercial use (including VPN resale) requires explicit written permission from the author.
SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
-->

<template>
  <span v-if="nodeType === 'lucx'" class="lucx-badge-wrapper">
    <a-tag color="blue">LucX-UI</a-tag>
    <a-tag v-if="hasFeature('awg')" color="orange">AWG {{ features.awgVersion || '' }}</a-tag>
    <a-tag v-if="hasFeature('telemt')" color="purple">MT {{ features.telemtVersion || '' }}</a-tag>
    <a-tag v-if="hasFeature('presets')" color="green">Pr</a-tag>
  </span>
  <a-tag v-else-if="nodeType === 'vanilla'" color="default">Vanilla 3x-ui</a-tag>
  <span v-else><!-- unchecked — no badge --></span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  nodeType: { type: String, default: '' },
  nodeFeatures: { type: String, default: '{}' }
})

const features = computed(() => {
  try {
    return JSON.parse(props.nodeFeatures)
  } catch {
    return {}
  }
})

function hasFeature(name) {
  return features.value.features?.includes(name)
}
</script>
