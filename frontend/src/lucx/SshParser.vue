<!--
Copyright (c) 2025 LucX-UI Project.
Licensed under the PolyForm Noncommercial License 1.0.0.
LucX-UI Component. Free for personal and educational use.
Commercial use (including VPN resale) requires explicit written permission from the author.
SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
-->

<template>
  <div class="ssh-parser">
    <a-alert
      type="info"
      message="Быстрый импорт из SSH"
      description="Вставьте весь вывод установочного скрипта 3x-ui — поля формы заполнятся автоматически."
      style="margin-bottom: 12px"
    />
    <a-textarea
      v-model:value="sshText"
      :rows="6"
      placeholder="Вставьте сюда вывод консоли после установки панели..."
      :disabled="loading"
    />
    <div style="margin-top: 8px; display: flex; gap: 8px; align-items: center">
      <a-button
        type="primary"
        :loading="loading"
        :disabled="!sshText.trim()"
        @click="parseText"
      >
        Распарсить
      </a-button>
      <a-button v-if="sshText.trim()" @click="sshText = ''">
        Очистить
      </a-button>
    </div>

    <a-alert
      v-if="parseError"
      type="error"
      :message="parseError"
      style="margin-top: 12px"
      closable
      @close="parseError = ''"
    />

    <a-alert
      v-if="parsedData"
      type="success"
      style="margin-top: 12px"
    >
      <template #message>
        Данные успешно распознаны
      </template>
      <template #description>
        <div>
          <p><strong>URL:</strong> {{ parsedData.scheme }}://{{ parsedData.host }}:{{ parsedData.port }}{{ parsedData.webBasePath }}</p>
          <p><strong>Логин:</strong> {{ parsedData.username || '(не найден)' }}</p>
          <p><strong>Пароль:</strong> {{ parsedData.password ? '***' : '(не найден)' }}</p>
          <p style="color: #52c41a; margin-top: 4px">Поля формы заполнены автоматически.</p>
        </div>
      </template>
    </a-alert>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { postLucx } from '../api/lucx-api'

const emit = defineEmits(['parsed'])

const sshText = ref('')
const loading = ref(false)
const parseError = ref('')
const parsedData = ref(null)

async function parseText() {
  loading.value = true
  parseError.value = ''
  parsedData.value = null

  try {
    const res = await postLucx('/parse-ssh', { text: sshText.value })
    if (res.success) {
      parsedData.value = res.obj
      emit('parsed', res.obj)
    } else {
      parseError.value = res.msg || 'Не удалось распознать данные. Проверьте ввод.'
    }
  } catch (e) {
    parseError.value = 'Ошибка сети или сервера. Попробуйте ещё раз.'
  } finally {
    loading.value = false
  }
}
</script>
