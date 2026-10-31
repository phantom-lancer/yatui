# yatui

[![release](https://img.shields.io/github/v/release/phantom-lancer/yatui?label=release)](https://github.com/phantom-lancer/yatui/releases/latest)
[![test](https://github.com/phantom-lancer/yatui/actions/workflows/test.yml/badge.svg)](https://github.com/phantom-lancer/yatui/actions/workflows/test.yml)
[![license](https://img.shields.io/github/license/phantom-lancer/yatui)](LICENSE)

Яндекс Музыка в терминале: воспроизведение, «Моя волна», плейлисты, поиск,
эквалайзер, Discord RPC и Last.fm. Windows, Linux, macOS.

<img width="1115" height="704" alt="image" src="https://github.com/user-attachments/assets/531dd13b-51b8-4fa9-9a28-7134ea7315ff" />



## Возможности

- музыка играет напрямую с серверов Яндекса, браузер не нужен;
- «Моя волна» с настройкой по занятию, характеру, настроению и языку (`F10`);
- полное управление мышью: клики, наведение, контекстное меню по правой кнопке,
  колесо по спискам и громкости;
- глобальный поиск `ctrl+f` и быстрый фильтр `/` по открытому списку;
- 10-полосный эквалайзер с пресетами (`F9`), изменения слышны сразу;
- очередь с перестановкой треков, менеджер кэша, темы оформления;
- нормализация громкости по ITU-R BS.1770 (EBU R128);
- Discord RPC: обложка, трек, артисты, ссылки, настройка карточки;
- Last.fm-скробблинг;
- системные медиа-клавиши: SMTC в Windows, MPRIS в Linux,
  MPRemoteCommandCenter в macOS.

## Установка

**Windows 10/11.** Скачайте `yatui-windows-amd64.exe` со страницы
[Releases](https://github.com/phantom-lancer/yatui/releases/latest) и запустите.
Если Windows предупреждает о неизвестном издателе — это обычное предупреждение
для файлов без подписи, «Подробнее» → «Выполнить в любом случае».

**Linux.** `yatui-linux-amd64` из релизов (нужны `libx11` и `libasound`) или:

```bash
go install github.com/phantom-lancer/yatui@latest
```

**macOS.** Бинарника пока нет, соберите сами:

```bash
go install github.com/phantom-lancer/yatui@latest
```

Версии с суффиксом `-nomedia` собраны без поддержки системных медиа-клавиш —
для окружений, где она мешает.

## Вход

При первом запуске откроется окно входа:

1. нажмите **Sign in via browser** (или `enter` на пустом поле);
2. браузер откроется на `ya.ru/device`, короткий код уже в буфере обмена —
   вставьте его и подтвердите вход;
3. токен сохранится сам, приложение запустится.

Токен проверяется через API до сохранения, поэтому неудачный вход не затрёт
рабочий токен. Вставить токен вручную тоже можно — например, из расширения
[Chrome](https://chromewebstore.google.com/detail/yandex-music-token/lcbjeookjibfhjjopieifgjnhlegmkib)
или [Firefox](https://addons.mozilla.org/ru/firefox/addon/yandex-music-token/).

`yatui --login` открывает окно входа, не стирая сохранённый токен.

## Горячие клавиши

`?` показывает все клавиши. Основное:

| Клавиша | Действие |
|---|---|
| `space` | пауза |
| `→` `←` | следующий / предыдущий трек |
| `ctrl+→` `ctrl+←` | перемотка |
| `↑` `↓`, `pgup` `pgdn` | курсор по списку |
| `enter` | играть |
| `l` | нравится |
| `a` | добавить в плейлист |
| `ctrl+f` | поиск по Яндекс Музыке |
| `/` | фильтр по открытому списку |
| `q` | очередь |
| `c` | кэш треков |
| `F9` | эквалайзер |
| `F10` | настройка «Моей волны» |
| `F11` | настройки |
| `ctrl+q` | выход |

В очереди (`q`) треки переставляются `ctrl+↑/↓` или `k`/`j`, `p` ставит трек
в начало, `delete` убирает, `enter` играет.

Клавиши переназначаются в конфиге, секция `controls`:

```yaml
controls:
    quit: ctrl+q,ctrl+c
    wave-settings: f10
    equalizer: f9
```

## Файлы

```
~/.config/yamusic-tui/config.yaml    настройки и токен
~/.cache/yamusic-tui/                кэш треков и громкости
%LOCALAPPDATA%\yamusic-tui\          то же в Windows
<временная папка>/yamusic-tui/       лог
```

`config.yaml` создаётся сам, всё настраивается в приложении. Токен лежит в нём
в открытом виде — не выкладывайте файл и скриншоты с ним. Отозвать доступ можно
в Яндекс ID → «Доступ к данным».

## Настройки

Кнопка **Настройки** внизу боковой панели (`F11`): громкость и шаг, буфер,
перемотка, нормализация, тема, текст песни, Last.fm, Discord RPC, кэш.

- **Эквалайзер** (`F9`) — пресеты и своя настройка, сохраняется между запусками.
- **Моя волна** (`F10`) — занятие, характер, настроение, язык; выбор применяется
  к новым сессиям радио, текущий трек не прерывается.
- **Discord RPC** — по умолчанию компактная карточка: артист, без альбома,
  версии, иконки и подписи; карточка скрывается через 5 минут после паузы.
- **Last.fm** — нужен API key и secret с
  [last.fm/api/account/create](https://www.last.fm/api/account/create),
  авторизация проходит в браузере.

## Сборка

```bash
git clone https://github.com/phantom-lancer/yatui.git
cd yatui
go build -trimpath -ldflags="-w -s" -o yatui .
```

Нужен Go 1.24+. Для Linux также нужны `libx11-dev` и `libasound2-dev`.
Тесты: `go test ./...`. Демо-ролик собирается из `demo.tape` через
[VHS](https://github.com/charmbracelet/vhs).

## Отличия от апстрима

Публичный форк [dece2183/yamusic-tui](https://github.com/DECE2183/yamusic-tui).
Здесь добавлены мышь, поиск, «Моя волна», эквалайзер, Discord RPC, Last.fm,
темы, кэш, нормализация громкости и вход через браузер.

## Лицензия

GPL-3.0, как у оригинального проекта. Автор оригинала —
[DECE2183](https://github.com/DECE2183). Проект не связан с Яндексом и не
одобрен им.
