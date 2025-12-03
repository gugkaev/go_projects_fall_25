### File Analysis Service

**Назначение**: хранение метаданных о работах в PostgreSQL, базовая проверка на плагиат и генерация URL облака слов.

### Модель `Work`

- **id**: целочисленный идентификатор.
- **student_id**: идентификатор студента (обязателен).
- **assignment_id**: идентификатор задания (обязателен).
- **file_id**: идентификатор файла в File Storage Service (обязателен).
- **plagiarism**: `true`, если работа полностью совпадает с ранее загруженной работой другого студента.
- **status**: статус обработки (`pending`, `completed`, `failed`).

### Эндпоинты

- **GET** `/healthz`  
  **Описание**: health-check сервиса.  
  **Ответ 200**:
  ```json
  { "status": "ok", "service": "file-analysis" }
  ```

- **POST** `/works`  
  **Описание**: создание записи о работе и анализ на плагиат.  
  **Тело запроса**:
  ```json
  {
    "student_id": "s1",
    "assignment_id": "hw1",
    "file_id": "abc123"
  }
  ```
  **Ответ 201**:
  ```json
  {
    "id": 1,
    "student_id": "s1",
    "assignment_id": "hw1",
    "file_id": "abc123",
    "plagiarism": false,
    "status": "completed",
    "created_at": "...",
    "updated_at": "..."
  }
  ```
  **Ошибки**:
  - `400` — валидация (отсутствуют обязательные поля или неверный JSON).
  - `500` — ошибки анализа/БД/доступа к File Storage Service.

- **GET** `/works/{id}`  
  **Описание**: получение информации о загруженной работе и результате проверки.  
  **Ответ 200**: объект `Work`.  
  **Ошибки**:
  - `400` — некорректный `id`.
  - `404` — работа не найдена.
  - `500` — ошибки БД.

- **GET** `/works/{id}/wordcloud`  
  **Описание**: генерация URL облака слов по тексту из файла.  
  **Ответ 200**:
  ```json
  {
    "url": "https://quickchart.io/wordcloud?text=..."
  }
  ```
  **Ошибки**:
  - `400` — некорректный `id`.
  - `500` — не удалось получить файл или сформировать URL.

### Примеры curl

- **Health-check**:

```bash
curl -i http://localhost:8082/healthz
```

- **Создание работы**:

```bash
curl -i -X POST http://localhost:8082/works \
  -H "Content-Type: application/json" \
  -d '{
    "student_id": "s1",
    "assignment_id": "hw1",
    "file_id": "abc123"
  }'
```

- **Получение работы**:

```bash
curl -i http://localhost:8082/works/1
```

- **Получение wordcloud URL**:

```bash
curl -i http://localhost:8082/works/1/wordcloud
```


