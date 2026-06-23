# Code Style & Conventions

## Go
- **Copyright header:** Каждый файл LucX начинается с блока лицензии PolyForm Noncommercial 1.0.0
- **Package naming:** Стандартный Go style — lowercase, без подчёркиваний
- **LUCX-HOOK маркеры:** Все интеграции с оригинальным кодом 3x-ui обёрнуты в `// LUCX-HOOK:` / `// END LUCX-HOOK` (152 маркера)
- **Структура пакетов:** `internal/lucx/<component>/` — плоская, без вложенности
- **Экспорты:** Имена экспортируемых типов — PascalCase, приватные — camelCase
- **JSON теги:** snake_case в JSON-тегах (без префикса `json:`)
- **Обработка ошибок:** Стандартный Go паттерн с возвратом error
- **Логирование:** Не используется стандартный log — преимущественно через общий logger проекта
- **Тесты:** Файлы `*_test.go` рядом с кодом; integration-тесты в `internal/lucx/integration/`

## Vue 3 / JavaScript
- **Composition API:** `<script setup>` синтаксис
- **Именование файлов:** PascalCase для компонентов (.vue), kebab-case для утилит (.js)
- **Структура:** Каждый LucX компонент в `frontend/src/lucx/`
- **i18n:** Все строки через vue-i18n, переводы в locale-файлах
- **Стили:** Используется Ant Design Vue 4 компоненты

## Общие правила
- **Изоляция:** Код LucX не смешивается с оригинальным кодом 3x-ui
- **Минимальное вторжение:** Оригинальные файлы 3x-ui получают только LUCX-HOOK блоки
- **Без комментариев:** Код самодокументируемый через имена (за исключением лицензионного заголовка)
- **Маркеры в Go:** `// LUCX-HOOK: reason` и `// END LUCX-HOOK`
- **Маркеры в JS/Vue:** `/* LUCX-HOOK: reason */` и `/* END LUCX-HOOK */`