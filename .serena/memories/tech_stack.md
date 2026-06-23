# Tech Stack

## Backend (Go)
- **Go** 1.26.3
- **Web Framework:** Gin (gin-gonic/gin v1.12.0)
- **ORM:** GORM + SQLite (gorm.io/gorm v1.31.1)
- **Xray-core:** xtls/xray-core v1.260327.0 (MPL-2.0)
- **Telegram Bot:** mymmrac/telego v1.8.0
- **gRPC:** google.golang.org/grpc v1.81.0
- **Сессии:** gin-contrib/sessions v1.1.0
- **HTTP Client:** valyala/fasthttp v1.71.0
- **Логирование:** op/go-logging
- **WireGuard:** golang.zx2c4.com/wireguard
- **Cron:** robfig/cron/v3

## Frontend (Vue 3 + Vite)
- **Vue** 3.5.13 (Composition API)
- **UI Framework:** Ant Design Vue 4.2.6
- **Build Tool:** Vite 8.0.11
- **HTTP:** Axios 1.7.9
- **i18n:** vue-i18n 11.1.4
- **Code Editor:** CodeMirror 6
- **Lint:** ESLint 10.3.0 + eslint-plugin-vue

## Инфраструктура
- **Деплой:** bash-скрипт install-lucx.sh (curl|bash, как официальный 3x-ui)
- **Поддерживаемые ОС:** Debian 12/13, Ubuntu 22.04/24.04, CentOS, Arch, Alpine
- **Бинарник:** x-ui (для CLI-совместимости)
- **CI:** GitHub Actions (.github/)