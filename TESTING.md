# Тестирование GoCron

Этот документ описывает стратегию тестирования и все доступные тесты для проекта GoCron.

## Обзор тестирования

Проект включает в себя комплексное тестирование всех компонентов:

- **Unit тесты** - тестирование отдельных компонентов в изоляции
- **Integration тесты** - тестирование взаимодействия компонентов
- **End-to-End тесты** - полные сценарии использования
- **Mock тесты** - тестирование с использованием заглушек

## Структура тестов

```
.
├── cmd/gocron/main_test.go              # Тесты main функции
├── integration_test.go                  # Интеграционные тесты
├── internal/
│   ├── api/api_test.go                 # Тесты API сервера
│   ├── config/config_test.go           # Тесты конфигурации
│   ├── models/models_test.go           # Тесты моделей данных
│   ├── scheduler/
│   │   ├── scheduler_test.go           # Основные тесты scheduler
│   │   └── scheduler_extended_test.go  # Расширенные тесты scheduler
│   ├── storage/postgres/
│   │   ├── postgres_generated_test.go  # Сгенерированные тесты
│   │   └── store_test.go              # Тесты хранилища
│   └── worker/
│       ├── worker_generated_test.go    # Сгенерированные тесты
│       └── worker_test.go             # Тесты worker manager
└── Makefile                           # Команды для запуска тестов
```

## Запуск тестов

### Все тесты
```bash
make test
# или
go test -v ./...
```

### Unit тесты
```bash
make test-unit
# или
go test -v ./internal/...
```

### Интеграционные тесты
```bash
make test-integration
# или
go test -v ./integration_test.go
```

### Тесты с покрытием
```bash
make test-coverage
```

### Тесты с детектором гонок
```bash
make test-race
```

### Отдельные компоненты
```bash
make test-models      # Тесты моделей
make test-config      # Тесты конфигурации
make test-api         # Тесты API
make test-worker      # Тесты worker
make test-storage     # Тесты хранилища
make test-scheduler   # Тесты scheduler
make test-main        # Тесты main функции
```

## Описание тестов по компонентам

### 1. Models (`internal/models/models_test.go`)
Тестирует структуры данных и их валидацию:
- `Job`, `Webhook`, `JobLog`, `CreateJobRequest`
- Валидация полей
- Сериализация/десериализация
- Edge cases для всех типов данных

**Покрытие:**
- ✅ Создание структур с валидными данными
- ✅ Обработка nil значений
- ✅ Различные типы webhook payload (JSON, Data)
- ✅ Статусы задач
- ✅ Временные метки

### 2. Configuration (`internal/config/config_test.go`)
Тестирует загрузку и валидацию конфигурации:
- Загрузка из файла
- Переменные окружения
- Значения по умолчанию
- Валидация параметров

**Покрытие:**
- ✅ Загрузка с дефолтными значениями
- ✅ Переопределение через environment variables
- ✅ Загрузка из YAML файла
- ✅ Валидация всех конфигурационных структур

### 3. API Server (`internal/api/api_test.go`)
Тестирует HTTP API:
- Endpoints (GET /, POST /jobs)
- Middleware (logging, recovery, request ID)
- Обработка ошибок
- Сериализация JSON

**Покрытие:**
- ✅ Health check endpoint
- ✅ Создание задач через API
- ✅ Валидация входных данных
- ✅ Обработка различных типов ошибок
- ✅ Middleware функциональность
- ✅ HTTP методы и статус коды

### 4. Worker Manager (`internal/worker/worker_test.go`)
Тестирует управление воркерами:
- Публикация задач
- Mock реализация для тестов
- Concurrency обработка
- Обработка ошибок

**Покрытие:**
- ✅ Mock worker manager
- ✅ Публикация задач с задержкой
- ✅ Concurrent операции
- ✅ Edge cases (zero delay, negative job ID)
- ✅ Timeout обработка
- ✅ Error handling

### 5. Storage (`internal/storage/postgres/store_test.go`)
Тестирует операции с базой данных:
- CRUD операции для задач и логов
- Транзакции
- Запросы и фильтрация
- Обработка ошибок БД

**Покрытие:**
- ✅ Создание и получение задач
- ✅ Обновление статусов задач
- ✅ Работа с логами выполнения
- ✅ Транзакционные операции
- ✅ Поиск по custom_id
- ✅ Пагинация логов
- ✅ Обработка ошибок БД

### 6. Scheduler (`internal/scheduler/scheduler_test.go`, `scheduler_extended_test.go`)
Тестирует основную логику планировщика:
- Создание и обработка задач
- Выполнение webhook'ов
- Переpланирование
- Обработка ошибок

**Основные тесты:**
- ✅ Создание задач
- ✅ Обработка задач (однократная и повторная)
- ✅ HTTP webhook выполнение

**Расширенные тесты:**
- ✅ Edge cases создания задач
- ✅ Обработка timeout'ов webhook'ов
- ✅ Невалидные URL'ы
- ✅ Различные типы payload
- ✅ Mock store для изоляции тестов
- ✅ Конвертация данных БД в модели

### 7. Main Function (`cmd/gocron/main_test.go`)
Тестирует инициализацию и жизненный цикл приложения:
- Graceful shutdown
- Signal handling
- Error handling
- Configuration loading

**Покрытие:**
- ✅ Функция run() с различными сценариями
- ✅ Обработка сигналов завершения
- ✅ Timeout'ы при shutdown
- ✅ Инициализация компонентов
- ✅ Обработка ошибок конфигурации
- ✅ Логирование

### 8. Integration Tests (`integration_test.go`)
Тестирует взаимодействие всех компонентов:
- End-to-end сценарии
- HTTP API интеграция
- Concurrent запросы
- Middleware интеграция

**Покрытие:**
- ✅ Полный workflow создания задач
- ✅ API endpoints интеграция
- ✅ Concurrent создание задач
- ✅ Различные типы webhook'ов
- ✅ Error handling на уровне API
- ✅ Middleware функциональность

## Настройка тестовой среды

### Переменные окружения для тестов

```bash
# Для тестов с реальной БД
export TEST_DATABASE_URL="postgres://user:password@localhost:5432/cron_test?sslmode=disable"

# Для интеграционных тестов
export POSTGRES_URL="postgres://user:password@localhost:5432/cron_test?sslmode=disable"
export RABBITMQ_URL="amqp://guest:guest@localhost:5672/"
```

### Подготовка тестовой БД

```bash
# Создание тестовой базы данных
createdb cron_test

# Запуск тестов с реальной БД
make test-with-db
```

## Mock объекты

Проект использует различные mock объекты для изоляции тестов:

1. **MockWorkerManager** - заглушка для worker manager
2. **MockScheduler** - заглушка для scheduler
3. **MockStore** - заглушка для базы данных
4. **MockSchedulerForIntegration** - заглушка для интеграционных тестов

## Покрытие кода

Для генерации отчета о покрытии:

```bash
make test-coverage
```

Это создаст файлы `coverage.out` и `coverage.html` с детальным отчетом.

## Continuous Integration

Для CI/CD используйте:

```bash
make ci-test    # Тесты для CI
make ci-lint    # Линтинг для CI
make ci-build   # Сборка для CI
```

## Бенчмарки

Для запуска бенчмарков:

```bash
make bench      # Все бенчмарки
make bench-cpu  # CPU профилирование
make bench-mem  # Memory профилирование
```

## Лучшие практики

1. **Изоляция тестов** - каждый тест должен быть независимым
2. **Cleanup** - всегда очищайте состояние после тестов
3. **Mock объекты** - используйте mock'и для внешних зависимостей
4. **Edge cases** - тестируйте граничные случаи
5. **Error handling** - проверяйте обработку ошибок
6. **Concurrency** - тестируйте concurrent операции
7. **Timeouts** - используйте timeout'ы в тестах

## Отладка тестов

Для отладки используйте:

```bash
# Подробный вывод
go test -v ./...

# Запуск конкретного теста
go test -v -run TestSpecificTest ./internal/api/

# С race detector
go test -v -race ./...

# Только короткие тесты
go test -v -short ./...
```

## Метрики качества

Проект стремится к следующим метрикам:
- **Покрытие кода**: > 80%
- **Все тесты**: должны проходить
- **Race conditions**: не должно быть
- **Lint issues**: не должно быть

Для проверки всех метрик качества:

```bash
make check-all
```