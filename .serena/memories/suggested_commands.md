# Suggested Commands

## Сборка
```bash
# Frontend build (обязательно перед Go build)
cd frontend && npm install && npm run build && cd ..

# Go build (бинарник x-ui)
go build -o x-ui -ldflags="-s -w" .

# Полный цикл сборки
cd frontend && npm run build && cd .. && go build -o x-ui -ldflags="-s -w" .
```

## Тестирование
```bash
# Unit + Integration тесты
go test ./internal/lucx/... ./internal/lucx/integration/... -v -count=1

# Chaos/stress тесты (требуют флаг -run "Vector")
go test ./internal/lucx/ -v -run "Vector" -count=1

# Все тесты с coverage
go test ./internal/lucx/... ./internal/lucx/integration/... -cover -count=1
```

## Линтинг
```bash
# Frontend
cd frontend && npm run lint

# Go — стандартный (go vet)
go vet ./...
```

## Запуск
```bash
# Запуск панели (после сборки)
./x-ui

# Или как сервис
systemctl restart x-ui
```

## Поиск по кодовой базе
```bash
# Найти все точки интеграции LucX
grep -rn "LUCX-HOOK" --include="*.go" --include="*.vue" --include="*.js"

# Найти все файлы LucX
find internal/lucx frontend/src/lucx -type f
```

## Git
```bash
git log --oneline -20
git diff main...lucx-ui-phase1
```
