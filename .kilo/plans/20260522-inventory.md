# План: Инвентаризация функций crocson с ранжировкой по категориям

## Цель
Собрать все описания функций crocson (включая унаследованные от croc) и ранжировать их по категориям из сводного реестра (F-Droid, FreeDesktop, MS Store, Google Play).

---

## 1. Функции croc используются в CLI режиме - если хоть один параметр не файл

### Передача файлов (File Transfer / Network / FileTransfer)
- Передача файлов и папок между любыми двумя компьютерами через ретранслятор (relay)
- Передача нескольких файлов и папок
- Возобновление прерванных передач (resuming)
- Кросс-платформенность: Windows, Linux, macOS
- Не требует локального сервера или проброса портов
- IPv6-first с автоматическим fallback на IPv4
- Самохостинг ретранслятора (`crocson relay`)
- Кастомные порты ретранслятора

### Безопасность (Security)
- Сквозное шифрование (end-to-end encryption) на базе PAKE (Password-Authenticated Key Agreement)
- Кастомная фраза-код (code phrase, мин. 6 символов)
- Выбор эллиптической кривой шифрования (`--curve p521` и др.)
- Пароль ретранслятора
- Переменная среды CROC_SECRET для безопасности процесса

### Сетевые технологии (Connectivity / Network)
- Поддержка прокси: SOCKS5 (в т.ч. Tor)
- HTTP прокси
- Multicast-адрес для локального обнаружения
- Множественные порты ретранслятора (9009-9013 по умолчанию)

### Утилиты (Utility / Productivity)
- Pipes (stdin/stdout): `cat file | crocson send`
- Отправка текста: `crocson send --text "hello"`
- QR-код для мобильных устройств
- Тихий режим (quiet mode) для скриптов
- Копирование кода в буфер обмена / расширенный буфер
- Хеширование: imohash, md5, xxhash, highway
- Исключение папок (`--exclude`)
- Перезапись файлов (`--overwrite`)
- .gitignore поддержка
- Ограничение скорости загрузки (throttle upload)

---

## 2. Функции crocson (GUI надстройка над croc)

### Передача файлов — GUI (File Transfer / Connectivity)
- Диалог выбора файлов и каталогов
- Drag-and-drop (Windows, Linux, macOS)
- Командная строка: аргументы `os.Args` и stdin pipe (`cat file|crocson`)
- Android: меню "Поделиться" (Share) из файловых менеджеров
- Android: "Открыть с помощью" (Open with)
- Отправка текста/URL через буфер обмена
- Android: отправка без вложенных каталогов, приём с вложенными → сохранение в .zip
- Прогресс-бар общий и пофайловый
- Кнопка отмены передачи
- Переключатель Send ↔ Receive (swap)

### WebDAV сервер (Connectivity / Cloud Storage & File Sync / Network)
- Встроенный WebDAV-сервер (HTTP/HTTPS)
- Самоподписанный TLS-сертификат (детерминированный, по локальным IP)
- Просмотр файлов через браузер (directory listing HTML)
- Дерево файлов WebDAV в GUI (WebDAVFileTree)
- Выбор хоста/порта для WebDAV
- Потоковое воспроизведение аудио/видео через WebDAV (Accept-Ranges, Content-Type)
- Символические ссылки и псевдоссылки (ResolvingFileSystem)
- TCP-портфорвардинг через croc-туннель (TCPForwarder)
  - Проброс WebDAV через ретранслятор croc (без прямого IP)
  - Зашифрованный туннель через ретранслятор (WebDAV доступен удалённо без прямого IP)

### Видеозвонки (Voice & Video Chat / Multimedia)
- Видеозвонки P2P через WebRTC (встроенная HTML-страница)
- Комнаты видеозвонков (create/join/wait/end)
- WebSocket для доставки чанков видео+аудио в реальном времени
- Loopback-превью (если удалённый пир не подключён)
- Серверная запись видеозвонков (WebM/MP4)
- Ремукс записи (fixRecordingFile): дописание индекса WebM / moov MP4
- Настройки пиров (кодек, разрешение)
- Ожидание второго участника (WaitingChan)

### Чат (Messaging / Social)
- Встроенный чат с текстовыми сообщениями
- WebSocket-based чат (handleChatWS)
- История сообщений (ChatStorage, in-memory)
- Автоматическое открытие чата при получении сообщения
- Отправка скриншотов и записей в чат (chatMessage)

### Запись видео/аудио (Multimedia / Photo + video)
- Запись видео+аудио через веб-камеру/микрофон браузера
- Серверная запись в WebM/MP4
- Выбор кодека (WebM / MP4)
- Запись с временной меткой (YYYYMMDD_HHMMSS_mmm)
- Публикация записей в чат (📹 /filename)

### Вебкамера / Фото (Multimedia / Photography)
- Захват с веб-камеры через браузер (getUserMedia)
- Скриншоты с видеозвонка (chatMessage с текстом скриншота)

### Безопасность — GUI (Security)
- Одноразовые пароли (TOTP) с таймером
- Секрет из поля ввода или переменной CROC_SECRET
- Сквозное шифрование (наследуется от croc)
- Настройки PAKE-кривой
- Настройки хеширования (imohash, md5, xxhash, highway)
- Зашифрованный туннель через ретранслятор (WebDAV доступен удалённо без прямого IP)

### QR-код и Deep Links (Connectivity / Utility)
- Генерация QR-кода с кодом-фразой
- Сканирование QR-кодов через камеру (html5-QRcode)
- Поддержка множества сканеров Android (Xiaomi, Samsung, OPlus, BinaryEye, Lens, ZXing, Chrome, Via, Samsung Browser, Opera mini, Microsoft, Firefox)
- Deep Links: `https://abakum.github.io/croc#...` (base64-encoded настройки)
- Deep Links: `davX:` / `webdavX:` для открытия WebDAV

### Профили ретрансляторов (Connectivity / Settings)
- Список профилей ретрансляторов (сохранение/загрузка)
- Переключение между ретрансляторами
- Локальный ретранслятор с управлением из GUI
- Кастомный адрес, IPv6, порты, пароль

### Интерфейс / Персонализация (Personalization / Theming)
- Темы: system, light, grey, dark, black
- Выбор шрифта (встроенные шрифты + системные)
- Многоязычность (en-US, tr-TR, ja-JP, zh-CN, ru-RU)
- Скрытие логотипа
- Цветной протокол передачи
- Android: протокол через `adb logcat -s croc`

### Настройки передачи (Utility / Settings)
- Отключение локальной передачи (только через relay)
- Подключение только к локальным отправителям
- .gitignore поддержка
- Перезапись файлов
- Сжатие вкл/выкл
- Мультиплексирование вкл/выкл
- ZIP-папок при отправке (Android)
- Ограничение скорости upload

### Кросс-платформенность
- Windows, Linux, macOS (Fyne GUI + desktop)
- Android (APK, 4 ABI: arm, arm64, 386, amd64)
- Командная строка (headless/pipe режим)

---

## 3. Маппинг на категории магазинов приложений

### F-Droid (наиболее релевантные)
| Категория | Функции |
|---|---|
| **File Transfer** | Передача файлов P2P через croc relay, WebDAV |
| **Connectivity** | WebDAV сервер, TCP-портфорвардинг, ретрансляторы, proxy |
| **Voice & Video Chat** | Видеозвонки WebRTC, запись видео/аудио |
| **Messaging** | Встроенный чат, история сообщений |
| **Security** | PAKE шифрование, TOTP, хеширование, TLS |
| **Multimedia** | Запись видео/аудио, вебкамера |
| **App Manager** | Профили ретрансляторов, настройки |

### FreeDesktop (Desktop Entry / AppStream)
| Категория | Функции |
|---|---|
| **Network** | Всё сетевое |
| **FileTransfer** | Передача файлов |
| **Chat** | Чат |
| **VideoConference** | Видеозвонки |
| **Telephony** | Аудио/видео звонки |
| **Security** | Шифрование |
| **AudioVideo** | Запись, воспроизведение |

### Microsoft Store
| Категория | Функции |
|---|---|
| **Productivity** | Передача файлов, утилиты |
| **Photo + video** | Вебкамера, запись видео |
| **Social** | Чат, видеозвонки |
| **Security** | Шифрование |

### Google Play
| Категория | Функции |
|---|---|
| **Communication** | Чат, видеозвонки, передача файлов |
| **Productivity** | WebDAV, утилиты |
| **Photography** | Вебкамера |
| **Tools** | Передача файлов, настройки |

---

## 4. Рекомендуемые категории для метаданных

### F-Droid (`com.github.abakum.crocson.yml`)
```yaml
Categories:
  - File Transfer
  - Connectivity
  - Voice & Video Chat
  - Security
```

### FreeDesktop (`FyneApp.toml` → `.desktop`)
```
Categories=Network;FileTransfer;Telephony;Security;
```

### Microsoft Store
- Primary: **Productivity**
- Secondary: **Social**

### Google Play
- Category: **Communication**
- Tags: file transfer, secure, P2P, video call, chat

---

## 5. Что нужно обновить

4. **`metadata/en-US/full_description.txt`** — обновить описание, добавив чат, видеозвонки, WebDAV, запись видео

5. **`metadata/ru-RU/full_description.txt`** — аналогично на русском

6. Остальные локализации (`ja-JP`, `zh-CN`, `tr-TR`) — аналогично
