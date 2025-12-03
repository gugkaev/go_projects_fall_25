### Система антиплагиата (Go, микросервисы)

**Назначение**: Система принимает студенческие работы, хранит файлы, проверяет их на полное совпадение с уже загруженными работами и генерирует облако слов.

### Архитектура

- **API Gateway (`api-gateway`)**
  - Единственная точка входа для клиентов.
  - Принимает multipart-загрузку работы, валидирует данные.
  - Отправляет файл в File Storage Service.
  - Передаёт `file_id`, `student_id`, `assignment_id` в File Analysis Service.
  - Проксирует запросы на получение отчёта и облака слов.

- **File Storage Service (`file-storage`)**
  - Принимает файлы и сохраняет их в локальный каталог/volume.
  - Возвращает `file_id` для дальнейшего использования.
  - Отдаёт файл по `file_id`.

- **File Analysis Service (`file-analysis`)**
  - Хранит метаданные работ в PostgreSQL (через GORM).
  - Выполняет базовую проверку на плагиат (совпадение содержимого/файла между разными студентами по одному заданию).
  - Генерирует URL облака слов через QuickChart Word Cloud API.

PostgreSQL используется как единая БД для File Analysis Service (таблица `works`).

#### Текстовая архитектурная диаграмма

```text
Client
  |
  v
API Gateway (8080)
  |  \___________________________
  |                              \
  v                               v
File Storage Service (8081)   File Analysis Service (8082)
       |                             |
       v                             v
   Files volume                 PostgreSQL (antiplag)

File Analysis -> QuickChart Word Cloud API (HTTP, внешняя служба)
```

### User Flow

1. **Загрузка работы**
   - Клиент отправляет `POST /works` на API Gateway с `multipart/form-data` (`student_id`, `assignment_id`, `file`).
   - Gateway загружает файл в File Storage (`POST /files`) → получает `file_id`.
   - Gateway вызывает File Analysis (`POST /works`) c `student_id`, `assignment_id`, `file_id`.
   - Analysis сохраняет запись в БД, проверяет на плагиат и возвращает объект `Work`.

2. **Получение отчёта**
   - Клиент делает `GET /works/{id}` на API Gateway.
   - Gateway проксирует запрос в File Analysis Service и возвращает JSON с работой и флагом `plagiarism`.

3. **Получение облака слов**
   - Клиент вызывает `GET /works/{id}/wordcloud` на API Gateway.
   - Analysis загружает текст файла из File Storage (`GET /files/{file_id}`).
   - Analysis строит URL для QuickChart Word Cloud API и возвращает его.

### Запуск через Docker Compose

```bash
docker compose up --build
```

После запуска:

- API Gateway: `http://localhost:8080`
- File Storage: `http://localhost:8081`
- File Analysis: `http://localhost:8082`
- PostgreSQL: `localhost:5432` (user: `postgres`, password: `postgres`, db: `antiplag`)

### Ключевые эндпоинты (через API Gateway)

- **POST** `/works` — загрузка работы и запуск проверки на плагиат.
- **GET** `/works/{id}` — получение отчёта по работе.
- **GET** `/works/{id}/wordcloud` — получение URL облака слов.
- **GET** `/healthz` — health-check Gateway; также доступны `/healthz` у каждого сервиса.

Подробное описание эндпоинтов и примеры `curl` есть в `README.md` каждого сервиса.

### Тестирование

В репозитории есть Postman-коллекция `postman_collection.json`, покрывающая сценарии:

- загрузка работы через API Gateway;
- получение отчёта о работе;
- получение URL облака слов.

Импортируйте коллекцию в Postman и используйте переменную `baseUrl` = `http://localhost:8080`.


