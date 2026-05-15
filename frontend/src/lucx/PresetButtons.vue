<!--
Copyright (c) 2025 LucX-UI Project.
Licensed under the PolyForm Noncommercial License 1.0.0.
LucX-UI Component. Free for personal and educational use.
Commercial use (including VPN resale) requires explicit written permission from the author.
SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
-->

<template>
  <div class="preset-buttons" style="margin-bottom: 16px">
    <div style="margin-bottom: 6px; font-weight: 500; color: var(--text-secondary)">
      Quick Presets
    </div>
    <div style="display: flex; gap: 6px; flex-wrap: wrap">
      <a-button
        v-for="preset in presets"
        :key="preset.id"
        size="small"
        :type="appliedPresetId === preset.id ? 'primary' : 'default'"
        @click="applyPreset(preset)"
      >
        {{ preset.label }}
      </a-button>
    </div>
    <a-alert
      v-if="currentPreset"
      type="success"
      :message="currentPreset.label"
      :description="currentPreset.description"
      style="margin-top: 8px"
    />
    <a-alert
      v-if="currentPreset?.notes"
      type="warning"
      :message="currentPreset.notes"
      style="margin-top: 4px"
    />
  </div>
</template>

<script setup>
import { ref } from 'vue'

const props = defineProps({
  presets: { type: Array, required: true },
})

const emit = defineEmits(['apply'])

const appliedPresetId = ref(null)
const currentPreset = ref(null)

function applyPreset(preset) {
  appliedPresetId.value = preset.id
  currentPreset.value = preset
  emit('apply', preset)
}
</script>
