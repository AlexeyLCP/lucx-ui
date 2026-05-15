<!--
Copyright (c) 2025 LucX-UI Project.
Licensed under the PolyForm Noncommercial License 1.0.0.
LucX-UI Component. Free for personal and educational use.
Commercial use (including VPN resale) requires explicit written permission from the author.
SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
-->

<template>
  <span>
    <a-tooltip title="Скопировать как аутбаунд на текущую ноду">
      <a-button
        v-if="canLink"
        size="small"
        type="text"
        :loading="loading"
        @click="generateOutbound"
      >
        <LinkOutlined />
      </a-button>
    </a-tooltip>

    <a-modal
      v-model:visible="modalVisible"
      :title="'Аутбаунд для ' + protocol + ' (' + inboundTag + ')'"
      :footer="null"
      width="640px"
    >
      <a-alert
        v-if="outboundData"
        type="info"
        message="Используется первый клиент инбаунда"
        style="margin-bottom: 12px"
      />

      <pre
        v-if="outboundData"
        style="background: #1e1e1e; color: #d4d4d4; padding: 16px; border-radius: 6px; overflow-x: auto; max-height: 400px; font-size: 12px"
      >{{ formattedJSON }}</pre>

      <div style="margin-top: 16px; display: flex; gap: 8px; justify-content: flex-end">
        <a-button @click="modalVisible = false">Закрыть</a-button>
        <a-button @click="copyToClipboard">
          <CopyOutlined /> Копировать
        </a-button>
      </div>

      <a-alert
        v-if="errorMsg"
        type="error"
        :message="errorMsg"
        style="margin-top: 12px"
        closable
        @close="errorMsg = ''"
      />
    </a-modal>
  </span>
</template>

<script setup>
import { ref, computed } from 'vue'
import { LinkOutlined, CopyOutlined } from '@ant-design/icons-vue'
import { postLucx } from '../../api/lucx-api'
import { message } from 'ant-design-vue'

const props = defineProps({
  inboundId: { type: Number, required: true },
  nodeId: { type: Number, required: true },
  protocol: { type: String, required: true },
  inboundTag: { type: String, default: '' },
  canLink: { type: Boolean, default: true }
})

const loading = ref(false)
const modalVisible = ref(false)
const outboundData = ref(null)
const errorMsg = ref('')

const formattedJSON = computed(() => {
  if (!outboundData.value) return ''
  try {
    const obj = typeof outboundData.value.outboundJson === 'string'
      ? JSON.parse(outboundData.value.outboundJson)
      : outboundData.value.outboundJson
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(outboundData.value.outboundJson)
  }
})

async function generateOutbound() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await postLucx('/inbound-to-outbound', {
      nodeId: props.nodeId,
      inboundId: props.inboundId
    })
    if (res.success) {
      outboundData.value = res.obj
      modalVisible.value = true
    } else {
      errorMsg.value = res.msg || 'Ошибка генерации аутбаунда'
    }
  } catch (e) {
    errorMsg.value = 'Ошибка сети или сервера'
  } finally {
    loading.value = false
  }
}

async function copyToClipboard() {
  try {
    await navigator.clipboard.writeText(formattedJSON.value)
    message.success('Скопировано в буфер обмена')
  } catch {
    message.error('Не удалось скопировать')
  }
}
</script>
