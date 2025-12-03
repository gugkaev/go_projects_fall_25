### API Gateway

**Назначение**: центральная точка входа в систему, принимает запросы клиентов и оркестрирует вызовы File Storage и File Analysis сервисов. Оборачивает ошибки внутренних сервисов в понятные HTTP 5xx JSON-ответы.

### Основные сценарии

- Клиент загружает работу (`student_id`, `assignment_id`, файл) на `/works`.
- Gateway отправляет файл в File Storage Service и получает `file_id`.
- Gateway вызывает File Analysis Service, создаёт запись `Work` и возвращает её клиенту.
- Клиент может получить результат анализа и URL облака слов через `/works/{id}` и `/works/{id}/wordcloud`.

### Эндпоинты

- **GET** `/healthz`  
  **Описание**: health-check API Gateway.  
  **Ответ 200**:
  ```json
  { "status": "ok", "service": "api-gateway" }
  ```

- **POST** `/works`  
  **Описание**: загрузка работы и запуск анализа.  
  **Тело запроса**: `multipart/form-data` с полями:
  - `student_id` — обязательное.
  - `assignment_id` — обязательное.
  - `file` — обязательное, бинарный файл.

  **Ответ 201**: JSON-объект `Work` (проксируется из File Analysis Service).

  **Коды ошибок**:
  - `400` — валидация или некорректная форма.
  - `413` — файл слишком большой.
  - `502` — File Storage или File Analysis недоступны / вернули 5xx.

- **GET** `/works/{id}`  
  **Описание**: получение информации о проверке работы (проксируется в File Analysis Service).  
  **Ответ 200**: объект `Work`.  
  **Ошибки**:
  - `400`, `404`, `500` — как в File Analysis Service.
  - `502` — если сервис анализа недоступен.

- **GET** `/works/{id}/wordcloud`  
  **Описание**: получение URL облака слов для работы.  
  **Ответ 200**:
  ```json
  { "url": "https://quickchart.io/wordcloud?text=..." }
  ```
  **Ошибки**:
  - `400`, `404`, `500` — как в File Analysis Service.
  - `502` — если сервис анализа недоступен.

### Примеры curl

- **Health-check**:

```bash
curl -i http://localhost:8080/healthz
```

- **Загрузка работы**:

```bash
curl -i -X POST http://localhost:8080/works \
  -F "student_id=s1" \
  -F "assignment_id=hw1" \
  -F "file=@/path/to/work.txt"
```

- **Получение отчёта по работе**:

```bash
curl -i http://localhost:8080/works/1
```

- **Получение URL облака слов**:

```bash
curl -i http://localhost:8080/works/1/wordcloud
```


