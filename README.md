# Микросервисы интернет-магазина «Гоzон»

## Описание проекта

Реализация микросервисной архитектуры для интернет-магазина с двумя основными сервисами:
- **Orders Service** - управление заказами
- **Payments Service** - управление счетами и платежами

### Гарантии доставки

- **At-least-once доставка**: Обеспечивается через RabbitMQ с persistent messages и manual acknowledgment
- **Effectively exactly-once списание**: Реализовано через:
  - Transactional Inbox с уникальным `message_id` (защита от дубликатов сообщений)
  - Таблица `deductions` с уникальным constraint на `order_id` (защита от повторного списания)
  - Версионирование счетов (CAS) для атомарных операций

### Защита от коллизий

- **Версионирование счетов**: Каждый счет имеет поле `version`
- **CAS операции**: Все операции изменения баланса используют `SELECT FOR UPDATE` и проверку версии
- **Транзакции**: Все критические операции выполняются в транзакциях

## Структура проекта

```
gozon_shop/
├── docker-compose.yml          # Конфигурация всех сервисов
├── orders-service/              # Сервис заказов
│   ├── Dockerfile
│   ├── main.go
│   ├── init/
│   │   └── db-schema.sql       # Схема БД
│   └── internal/
│       ├── amqp/               # RabbitMQ consumer для обновлений заказов
│       ├── db/                 # Работа с БД
│       ├── handlers/           # HTTP handlers
│       ├── models/             # Модели данных
│       └── outbox/             # Outbox worker
├── payments-service/            # Сервис платежей
│   ├── Dockerfile
│   ├── main.go
│   ├── init/
│   │   └── 01-schema.sql       # Схема БД
│   └── internal/
│       ├── amqp/               # RabbitMQ consumer для платежей
│       ├── db/                 # Работа с БД
│       ├── handlers/           # HTTP handlers
│       └── outbox/             # Outbox worker
├── frontend/                    # Веб-интерфейс
│   ├── Dockerfile
│   └── index.html              # Frontend приложение
├── integration_test.go          # Integration тесты
├── openapi.yaml                # OpenAPI/Swagger документация
├── postman_collection.json     # Postman коллекция
├── Makefile                    # Команды для тестирования
└── README.md
```

### Запуск

```bash
# Запуск всех сервисов
docker compose up --build

# Просмотр логов
docker compose logs -f
```

После запуска сервисы будут доступны:
- **Orders Service**: http://localhost:8080
- **Payments Service**: http://localhost:8081
- **Frontend**: http://localhost:3000
- **RabbitMQ Management**: http://localhost:15672 (guest/guest)
- **OpenAPI/Swagger**: см. файл `openapi.yaml`

## API Endpoints

### Payments Service

#### POST /accounts
Создание счета пользователя

**Request:**
```json
{
  "user_id": "123"
}
```

**Response:** `201 Created`
```json
{
  "user_id": "123",
  "balance": 0
}
```

#### POST /accounts/{user_id}/topup
Пополнение счета

**Request:**
```json
{
  "amount": 1000,
  "idempotency_key": "optional-key"
}
```

**Response:** `200 OK`
```json
{
  "user_id": "123",
  "balance": 1000
}
```

#### GET /accounts/{user_id}/balance
Получение баланса

**Response:** `200 OK`
```json
{
  "user_id": "123",
  "balance": 1000
}
```

### Orders Service

#### POST /orders
Создание заказа (асинхронно запускает процесс оплаты)

**Request:**
```json
{
  "user_id": "123",
  "amount": 500,
  "items": []
}
```

**Response:** `202 Accepted`
```json
{
  "order_id": 1,
  "status": "PENDING"
}
```

#### GET /orders?user_id=123
Получение списка заказов пользователя

**Response:** `200 OK`
```json
[
  {
    "id": 1,
    "user_id": "123",
    "amount": 500,
    "status": "PAID",
    "created": "2024-01-01T12:00:00Z"
  }
]
```

#### GET /orders/{order_id}
Получение информации о заказе

**Response:** `200 OK`
```json
{
  "order_id": 1,
  "status": "PAID",
  "amount": 500
}
```

## Примеры использования

### Создание счета и пополнение

```bash
# Создание счета
curl -X POST http://localhost:8081/accounts \
  -H "Content-Type: application/json" \
  -d '{"user_id": "123"}'

# Пополнение счета
curl -X POST http://localhost:8081/accounts/123/topup \
  -H "Content-Type: application/json" \
  -d '{"amount": 1000}'

# Проверка баланса
curl http://localhost:8081/accounts/123/balance
```

### Создание заказа

```bash
# Создание заказа
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id": "123", "amount": 500, "items": []}'

# Проверка статуса заказа
curl http://localhost:8080/orders/1

# Список заказов пользователя
curl "http://localhost:8080/orders?user_id=123"
```

