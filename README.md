# Система Бронирования Билетов

## Архитектура

Система состоит из 4 сервисов:

- API Gateway, который принимает HTTP-запросы, валидирует JWT-токены и проксирует вызовы к внутренним сервисам
- Auth Service, который отвечает за регистрацию, вход и валидацию токенов
- Event Service, который управляет каталогом событий
- Booking Service, который обрабатывает логику создания бронирований

### .env файл

Добавил .envexample, его нужно переименовать в .env и, если надо, поменять переменные:
```bash
mv .envexample .env
```

### Запуск

1.  Запустить все сервисы:
    ```bash
    make up
    ```

2.  Остановить систему:
    ```bash
    make down
    ```

3. Вывести логи:
	```bash
	make logs
	```

### Подготовка

Чтобы протестировать систему, необходимо добавить события:

1.  Зайдём в psql:
```bash
docker exec -it ticket-booking-system-postgres-1 psql -U user -d booking_db
```

3.  Добавим события:
```sql
INSERT INTO events (id, title, description) VALUES (1, 'Shrek', 'A mean lord exiles fairytale creatures to the swamp of a grumpy ogre, who must go on a quest and rescue a princess for the lord in order to get his land back.');
INSERT INTO events (id, title, description) VALUES (2, 'Agil', 'An angelically patient person welcomes a group of dysfunctional friends into their life, who then embark on a quest to test every last one of his boundaries for their own amusement and personal gain.');
\q
```

## Примеры

### 1. Регистрация нового пользователя

```bash
curl -v -X POST -H "Content-Type: application/json" \
     -d '{"email": "my_user@example.com", "password": "password123"}' \
     http://localhost:8080/api/v1/register
```
Ожидаемый ответ:
```json
{"user_id":1}
```

### 2. Вход в систему и получение JWT-токена

```bash
curl -X POST -H "Content-Type: application/json" \
     -d '{"email": "my_user@example.com", "password": "password123"}' \
     http://localhost:8080/api/v1/login
```
Ожидаемый ответ:
```json
{"token":"your-token"}
```

### 3. Просмотр списка событий

```bash
curl http://localhost:8080/api/v1/events
```
Ожидаемый ответ:
```json
[{"id":2,"title":"Agil","description":"An angelically patient person welcomes a group of dysfunctional friends into their life, who then embark on a quest to test every last one of his boundaries for their own amusement and personal gain."},{"id":1,"title":"Shrek","description":"A mean lord exiles fairytale creatures to the swamp of a grumpy ogre, who must go on a quest and rescue a princess for the lord in order to get his land back."}]
```

### 4. Создание бронирования

Нужно взять токен (your-token) из 2 шага

```bash
curl -v -X POST -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-token" \
     -d '{"event_id": 1, "seat_ids": [1, 2, 3, 4]}' \
     http://localhost:8080/api/v1/bookings
```
Ожидаемый ответ:
```json
{"booking_id":1}
```

