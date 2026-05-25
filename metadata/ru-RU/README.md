# crocson

Форк [howeyc/crocgui](https://github.com/howeyc/crocgui/releases/tag/v1.11.5) — и [schollz/croc](https://github.com/schollz/croc) ориентированный на Android, Windows, Linux и macOS.

<p align="center">
  <img src="images/phoneScreenshots/1.png?raw=true" width="200">
  <img src="images/phoneScreenshots/2.png?raw=true" width="200">
  <img src="images/phoneScreenshots/4.png?raw=true" width="200">
  <br>
</p>

<p align="center">
  <img src="../../Icon.png?raw=true" width="100"><br>
  Автор <i>croc</i> — <a href="https://github.com/schollz">Zack Scholl</a> выбрал название
  <i>croc</i> как метафору безопасной перевозки зверей крокодилом-таксистом.<BR>
  <i>crocson</i> — наследник крокодила-таксиста позволяющего безопасно общаться онлайн.
</p>

## Передача файлов
`F-Droid: File Transfer` `FreeDesktop: Network;FileTransfer` `MS Store: Productivity` `Google Play: Communication;Tools`

- Передача файлов и папок между любыми двумя компьютерами через ретранслятор (relay)
- Передача нескольких файлов и папок (последовательно)
- Кросс-платформенность: Windows, Linux, macOS, Android
- Не требует настройки локального сервера или проброса портов
- IPv6-first с автоматическим переходом на IPv4
- Самохостинг ретранслятора (`crocson relay`)
- Кастомные порты ретранслятора
- Диалог выбора файлов и каталогов
- Drag-and-drop (Windows, Linux, macOS)
- Командная строка: аргументы и stdin pipe (`cat file | crocson`)
- Android: меню «Поделиться» из файловых менеджеров для одного или нескольких файлов
- Android: «Открыть с помощью»
- Отправка текста/URL через буфер обмена
- Android: отправка без вложенных каталогов, приём с вложенными — сохранение в .zip
- Прогресс-бар общий и пофайловый
- Кнопка отмены передачи
- Переключатель Send ↔ Receive

## WebDAV
`F-Droid: Connectivity;Cloud Storage & File Sync` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Productivity`

- Встроенный WebDAV-сервер (HTTP/HTTPS)
- Самоподписанный TLS-сертификат (детерминированный, по локальным IP)
- Просмотр файлов через браузер (directory listing)
- Дерево файлов WebDAV в GUI
- Выбор хоста/порта
- Потоковое воспроизведение аудио/видео через WebDAV
- Проброс WebDAV через зашифрованный туннель (доступен удалённо без прямого IP)

## Видеозвонки
`F-Droid: Voice & Video Chat` `FreeDesktop: Network;VideoConference` `MS Store: Social` `Google Play: Communication`

- Видеозвонки P2P через WebRTC (встроенная HTML-страница)
- Демонстрация экрана: вкладка браузера, окно приложения или весь рабочий стол
- Комнаты видеозвонков (создание/подключение/ожидание/завершение)
- Передача видео+аудио в реальном времени через WebSocket
- Превью (зеркало) до подключения второго участника
- Серверная запись видеозвонков (WebM/MP4)
- Выбор кодека и разрешения

## Чат
`F-Droid: Messaging` `FreeDesktop: Network;Chat` `MS Store: Social` `Google Play: Communication`

- Встроенный чат с текстовыми сообщениями
- История сообщений
- Автоматическое открытие чата при получении сообщения
- Отправка скриншотов и видеозаписей в чат

## Запись видео/аудио
`F-Droid: Multimedia` `FreeDesktop: AudioVideo;Recorder` `MS Store: Photo + video` `Google Play: Video Players & Editors`

- Запись видео+аудио через веб-камеру/микрофон браузера
- Запись с временной меткой (YYYYMMDD_HHMMSS_mmm)
- Публикация записей в чат

## Вебкамера
`F-Droid: Multimedia` `FreeDesktop: AudioVideo` `MS Store: Photo + video` `Google Play: Photography`

- Захват с веб-камеры через браузер
- Скриншоты с видеозвонка

## Безопасность
`F-Droid: Security` `FreeDesktop: Security` `MS Store: Security` `Google Play: Tools`

- Сквозное шифрование (end-to-end encryption) на базе PAKE
- Одноразовые пароли (TOTP) с таймером
- Секрет из поля ввода или переменной среды CROC_SECRET
- Выбор эллиптической кривой шифрования
- Хеширование: imohash, md5, xxhash, highway
- Пароль ретранслятора
- Зашифрованный туннель через ретранслятор

## QR-код и Deep Links
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- Генерация QR-кода с кодом-фразой
- Сканирование QR-кодов через камеру
- Поддержка множества сканеров Android (Xiaomi, Samsung, OPlus, BinaryEye, Lens, ZXing, Chrome, Via, Samsung Browser, Opera mini, Microsoft, Firefox)
- Deep Links: `https://abakum.github.io/croc#...` (base64-encoded настройки)
- Deep Links: `davX:` / `webdavX:` для открытия WebDAV

## Профили ретрансляторов
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- Список профилей ретрансляторов (сохранение/загрузка)
- Переключение между ретрансляторами
- Локальный ретранслятор с управлением из GUI
- Кастомный адрес, IPv6, порты, пароль
- Поддержка прокси: SOCKS5 (в т.ч. Tor), HTTP

## Настройки передачи
`F-Droid: Connectivity` `FreeDesktop: Network` `MS Store: Productivity` `Google Play: Tools`

- Отключение локальной передачи (только через relay)
- Подключение только к локальным отправителям
- .gitignore поддержка
- Перезапись файлов
- Сжатие вкл/выкл
- Мультиплексирование вкл/выкл
- ZIP-папок при отправке
- Ограничение скорости upload

## Интерфейс
`F-Droid: Personalization` `FreeDesktop: Settings` `MS Store: Personalization` `Google Play: Personalization`

- Темы: system, light, grey, dark, black
- Выбор шрифта (встроенные шрифты + системные)
- Многоязычность (en-US, tr-TR, ja-JP, zh-CN, ru-RU)
- Скрытие логотипа
- Цветной протокол передачи
- Android: протокол через `adb logcat -s croc`

## CLI режим

Если передан хотя бы один параметр, не являющийся файлом, crocson работает как croc CLI:
- Возобновление прерванных передач (resuming)
- Pipes (stdin/stdout): `cat file | crocson send`
- Отправка текста: `crocson send --text "hello"`
- QR-код для мобильных устройств
- Тихий режим (quiet mode) для скриптов
- Копирование кода в буфер обмена
- Исключение папок (`--exclude`)
- Переменная среды CROC_SECRET для безопасности процесса

---

## Ранжирование категорий по каталогам

| Функция | F-Droid | FreeDesktop | MS Store | Google Play |
|---|---|---|---|---|
| Передача файлов | **File Transfer** | Network;FileTransfer | Productivity | Communication |
| WebDAV | **Connectivity**; Cloud Storage & File Sync | Network | Productivity | Productivity |
| Видеозвонки | **Voice & Video Chat** | Network;VideoConference | Social | Communication |
| Чат | **Messaging** | Network;Chat | Social | Communication |
| Запись видео/аудио | **Multimedia** | AudioVideo;Recorder | Photo + video | Video Players & Editors |
| Вебкамера | **Multimedia** | AudioVideo | Photo + video | Photography |
| Безопасность | **Security** | Security | Security | Tools |
| QR-код / Deep Links | **Connectivity** | Network | Productivity | Tools |
| Профили ретрансляторов | **Connectivity** | Network | Productivity | Tools |
| Настройки передачи | **Connectivity** | Network | Productivity | Tools |
| Интерфейс | **Personalization** | Settings | Personalization | Personalization |
