# WB Demo Project (Go)

Демонстрационный сервис для обработки и отображения заказов.  
Проект реализует полный цикл: получение данных через **NATS Streaming**, сохранение в **PostgreSQL**, кеширование в памяти и отображение заказов в веб-интерфейсе.

---

# Основные возможности

- Подключение и подписка на канал `orders` в **NATS Streaming**
- Сохранение заказов в базу данных **PostgreSQL**
- Кеширование данных в памяти (in-memory cache) для быстрого доступа
- Автоматическое восстановление кеша при перезапуске сервиса
- HTTP API:
  - `GET /order/{id}` — получить заказ по ID
  - `GET /order/all` — получить список всех заказов
- Веб-интерфейс с поиском заказов и просмотром подробностей в popup-окне
- Автотесты

---

# Структура проекта

| Файл / Папка      | Описание                                           |
| ----------------- | -------------------------------------------------- |
| `main.go`         | Основной сервис (HTTP + NATS + PostgreSQL + Cache) |
| `publisher.go`    | Скрипт для публикации заказов в NATS Streaming     |
| `db/init.sql`     | SQL-схема для создания таблицы `orders`            |
| `web/index.html`  | Веб-интерфейс для просмотра заказов                |
| `web/style.css`   | Стили фронтенда                                    |
| `model.json`      | Пример JSON заказа                                 |
| `main_test.go`    | Модульные тесты сервиса                            |
| `service_test.go` |                                                    |

---

# Быстрый старт (локально)

1. **Разверните PostgreSQL и NATS Streaming (Stan)** локально.
   nats-streaming-server.exe -p 4222 -m 8222 -hbi 5s -hbt 5s -hbf 2 -SD -cid test-cluster

2. **Создайте базу данных и таблицы:**

   ```bash
   psql -h localhost -U postgres -c "CREATE DATABASE wb_demo;"
   psql -h localhost -U postgres -d wb_demo -f db/init.sql
   ```

3. **Настройте переменные окружения:**
   export DSN="postgres://postgres:2212@localhost:5432/wb_demo?sslmode=disable"
   export NATS_CLUSTER="test-cluster"

4. **Запустите сервис**
   go run main.go

5. **(Опционально) Отправьте пример заказа в NATS Streaming:**
   go run publisher.go

6. **Откройте веб-интерфейс:**
   http://localhost:8080
