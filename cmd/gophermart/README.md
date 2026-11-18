# Gophermart: запускаемый каркас (этап 2)

Целевая версия модуля — Go 1.26.0. По указанию пользователя локальные команды
и проверки выполняются доступным Go 1.27.1; совместимость с Go 1.26 ещё не проверена.

Приложение читает конфигурацию, пишет production JSON-логи уровня info в stderr
и запускает HTTP-сервер. Все пути пока возвращают 404: бизнес-API, PostgreSQL,
миграции и worker будут добавлены в следующих этапах.

## Конфигурация

| Флаг | Окружение | Значение по умолчанию |
| --- | --- | --- |
| `-a` | `RUN_ADDRESS` | `:8080` |
| `-d` | `DATABASE_URI` | обязательно |
| `-r` | `ACCRUAL_SYSTEM_ADDRESS` | обязательно |

Приоритет: явно переданный флаг → окружение → default. Явно пустые значения
считаются ошибкой; проверяется только окончательный результат слияния.
Используется стандартный глобальный `flag.Parse()`: `-h`/`-help` выводят справку
и завершают процесс с кодом 0, ошибочные флаги — с кодом 2. В этих случаях
выход выполняет сам парсер, без defer (включая logger.Sync).
Ошибки валидации конфигурации и запуска возвращают код 1, штатное завершение — 0.
LoadConfig вызывается один раз при старте. Стандартная справка показывает defaults
из окружения, включая DATABASE_URI, который может содержать пароль.

HTTP-адрес — host:port с портом 1–65535; допускаются `:8080`, `localhost:8080`,
`[::1]:8080`. PostgreSQL DSN принимает `postgres://`, `postgresql://` и
keyword/value, например `host=localhost port=5432 dbname=gophermart user=postgres
password=postgres sslmode=disable` (одной строкой).
Accrual — HTTP(S) origin без credentials, префикса пути, query и fragment;
завершающий `/` допустим и удаляется после проверки.

Адреса БД и accrual обязательны уже сейчас, но каркас не устанавливает соединения
с этими сервисами. Разбор DSN через pgx может учитывать стандартные PG*-переменные
и служебные файлы драйвера. Приложение не записывает DSN в логи.

## Локальный запуск

Из корня репозитория:

```sh
make build
make run
make run RUN_ADDRESS=:8090
make test
```

Makefile передаёт локальные defaults: HTTP `:8080`, accrual `http://localhost:8081`,
DSN из примера выше. Все параметры переопределяются переменными make.
DSN передаётся одним shell-аргументом, включая пробелы и одиночные кавычки;
команда с DSN не печатается. Для буквального `$` в значении make используйте `$$`
и учитывайте quoting вашей оболочки.

Прямой запуск с URL DSN:

```sh
./cmd/gophermart/gophermart -a :8080 \
  -d 'postgresql://postgres:postgres@localhost:5432/gophermart?sslmode=disable' \
  -r http://localhost:8081
curl -i http://localhost:8080/unknown
```

Ожидается 404. Ctrl+C (SIGINT) или SIGTERM завершает приложение.
ReadHeaderTimeout=5s, ReadTimeout=10s, WriteTimeout=30s, IdleTimeout=60s.
Shutdown получает отдельный контекст на 10s; при истечении соединения закрываются
принудительно и приложение возвращает ошибку. Будущий общий бюджет бизнес-операции
HTTP — 25s; RetryStrategy добавляется вместе с репозиториями, а не при старте каркаса.

## Готовый accrual

Для следующих этапов заранее подготовьте PostgreSQL и БД `gophermart`. Приложения
не создают саму базу. Запускайте в отдельных терминалах:

```sh
make run-accrual
# Во втором терминале:
make run
```

`ACCRUAL_RUN_ADDRESS` по умолчанию `:8081`, `ACCRUAL_DATABASE_URI` наследует
`DATABASE_URI`. Пример: `make run-accrual ACCRUAL_RUN_ADDRESS=:8082` и
`make run ACCRUAL_SYSTEM_ADDRESS=http://localhost:8082`.

Цель выбирает существующий бинарник по `go env GOHOSTOS GOHOSTARCH`:
darwin/arm64, darwin/amd64, linux/amd64, windows/amd64. Windows требует GNU Make
и POSIX-оболочку. Неподдерживаемая платформа или отсутствующий файл дают ошибку;
скачивание замены не выполняется. Перед запуском очищаются конфликтующие
`RUN_ADDRESS`, `DATABASE_URI`, `ACCRUAL_SYSTEM_ADDRESS`, чтобы accrual использовал
переданные `-a` и `-d`. Выбранный файл при необходимости получает исполняемый бит.

Проверены выбор локального darwin/arm64, передача DSN в аргументах и запуск HTTP
на заданном адресе при конфликтующем окружении;
успешная работа с настоящей БД пока не проверена.

## Проверки

```sh
go test ./...
go test -race ./...
go vet ./...
(cd cmd/gophermart && go build -buildvcs=false -o gophermart)
```

Для тестов сервера нужны временные loopback TCP-порты, PostgreSQL не нужен.
Обычный `go vet` не заменяет внешний `statictest` из CI. Полные `gophermarttest`
относятся к завершённому бизнес-API. Фактический прогресс и ограничения проверок —
в `docs/IMPLEMENTATION-STATUS.md` (каталог docs сейчас исключён существующим .gitignore).

## Структура после ревью каркаса

`main` создаёт zap, вызывает `config.LoadConfig()`, затем
`build.BuildApp(cfg, logger)` из `internal/build` и `App.Run(ctx)`.
Он логирует ошибки и штатную остановку, выполняет Sync до выхода.

`App` зависит от интерфейса `ServerBuilder` с `Build() (*http.Server, error)`.
Конкретный билдер в `internal/server` хранит chi и адрес, принимает готовые
handler-структуры через `AddHandler(method, path, handler) error` и создаёт
`*http.Server`. Пока маршрутов нет. AddHandler проверяет метод, непустой путь
с `/` и обычный nil; повторная регистрация заменяет обработчик. Полный синтаксис
шаблонов оставлен chi, типизированный nil не проверяется.

Общий `internal/interfaces.Logger` предоставляет Debug/Info/Warn/Error с zap.Field.
`LoadConfig()` самостоятельно читает окружение и аргументы процесса, после чего
вызывает отдельный `Validator`. Ошибки различимы через errors.Is и оборачиваются
названиями действий. Проверки выполняются последовательно до первой ошибки.
