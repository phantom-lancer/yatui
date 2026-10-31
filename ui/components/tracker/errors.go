package tracker

// errorTexts translates the internal keys of the player errors into the
// messages the user sees under the player. A key that is not here is shown as
// it is, so a new error is never hidden by a missing translation.
var errorTexts = map[string]string{
	"missing token":                          "Нет токена — войдите заново: yatui --login",
	"unable to connect to the Yandex server": "Нет связи с Яндексом — проверьте сеть или прокси",
	"unable to init rotor session":           "Не удалось запустить волну — проверьте сеть",
	"wave session":                           "Не удалось обновить волну — проверьте сеть",
	"radio session":                          "Не удалось запустить радио — проверьте сеть",
	"stations list":                          "Не удалось получить список станций — проверьте сеть",
	"station session":                        "Не удалось включить станцию — проверьте сеть",
	"next track obtain failure":              "Не удалось получить следующий трек — проверьте сеть",

	"track download":  "Не удалось загрузить трек — проверьте сеть и попробуйте ещё раз",
	"track buffering": "Трек не буферизуется — проверьте сеть",
	"track lyrics":    "Не удалось загрузить текст песни",

	"cache list":                           "Не удалось прочитать кэш — файлы кэша могут быть повреждены",
	"cache open":                           "Не удалось открыть файл кэша",
	"cache write":                          "Не удалось записать в кэш — проверьте место на диске",
	"cache remove":                         "Не удалось удалить трек из кэша",
	"can't remove currently playing track": "Играющий трек нельзя убрать из кэша",

	"liked tracks":    "Не удалось загрузить любимые треки — проверьте сеть",
	"liked playlists": "Не удалось загрузить любимые плейлисты — проверьте сеть",
	"liked artists":   "Не удалось загрузить любимых артистов — проверьте сеть",
	"liked albums":    "Не удалось загрузить любимые альбомы — проверьте сеть",
	"pinned albums":   "Не удалось загрузить закреплённые альбомы — проверьте сеть",
	"playlists":       "Не удалось загрузить плейлисты — проверьте сеть",

	"artist tracks":      "Не удалось получить треки артиста — проверьте сеть",
	"artist tracks info": "Не удалось получить информацию об артисте — проверьте сеть",

	"search":            "Не удалось выполнить поиск — проверьте сеть",
	"search seggestion": "Не удалось получить подсказки поиска — проверьте сеть",

	"playlist tracks":       "Не удалось загрузить треки плейлиста — проверьте сеть",
	"playlist create":       "Не удалось создать плейлист — проверьте сеть",
	"playlist add":          "Не удалось добавить трек в плейлист — проверьте сеть",
	"playlist rename":       "Не удалось переименовать плейлист — проверьте сеть",
	"playlist remove":       "Не удалось удалить плейлист — проверьте сеть",
	"playlist remove track": "Не удалось убрать трек из плейлиста — проверьте сеть",
}
