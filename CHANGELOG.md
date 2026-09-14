## Основные изменения

- Фреймворк [`wails`](https://v3.wails.io/) обновлён `v2 -> v3`.
- Все новые версии будут публиковаться [`AppImage`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+AppImage%3F&sxsrf=APpeQntg0kXMvyQ9nv5v2snUp9C_h3jMqQ%3A1789287370533&uact=5), [`rpm`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+RPM+Package+Manage%3F&sxsrf=APpeQnuCLV0ulzDEaktzsU8ABePespf48A%3A1789287692406&uact=5), [`deb`](https://www.google.com/search?sxsrf=APpeQnsxhYKZT3JGoKJqYGwFmkunKKZOXw:1789287729279&q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+deb+Package+Manager?&spell=1&dpr=1).
- Удалена загрузка из [`зеркала CDN`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+%D0%B7%D0%B5%D1%80%D0%BA%D0%B0%D0%BB%D0%BE+%D0%B4%D0%B0%D0%BD%D0%BD%D1%8B%D0%B7&sxsrf=APpeQnv1E3f_HZnPe_ypzqyIyhUKFIzvXQ%3A1789287908092&uact=5) и все её упоминания в коде.
- Загрузка [`Community Shader`](https://www.nexusmods.com/skyrimspecialedition/mods/86492) более не зависит от зеркал.
- Конфигурация и расположение файлов игры теперь хранятся в `~/.config/rfad-launcher/launcher.conf`.
- Небольшое нововведение в установку.
- Значительное обновление окна установки.
- Установка патчей/Распаковка Wine/Proton Prefix _(первый запуск)_ `src-wails/core/core.go: FirstInstall()` более не устанавливают обновление игры.

## Подробнее об обновлении

Обновление [`wails`](https://v3.wails.io/) привело к обновлению зависимости `webkit2gtk-4.1 -> webkitgtk-6.0`. Основная причина обновления [`wails`](https://v3.wails.io/) возможность нативного билда приложения под Linux в формате [`AppImage`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+AppImage%3F&sxsrf=APpeQntg0kXMvyQ9nv5v2snUp9C_h3jMqQ%3A1789287370533&uact=5), [`rpm`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+RPM+Package+Manage%3F&sxsrf=APpeQnuCLV0ulzDEaktzsU8ABePespf48A%3A1789287692406&uact=5), [`deb`](https://www.google.com/search?sxsrf=APpeQnsxhYKZT3JGoKJqYGwFmkunKKZOXw:1789287729279&q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+deb+Package+Manager?&spell=1&dpr=1).

Форматы [`AppImage`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+AppImage%3F&sxsrf=APpeQntg0kXMvyQ9nv5v2snUp9C_h3jMqQ%3A1789287370533&uact=5), [`rpm`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+RPM+Package+Manage%3F&sxsrf=APpeQnuCLV0ulzDEaktzsU8ABePespf48A%3A1789287692406&uact=5), [`deb`](https://www.google.com/search?sxsrf=APpeQnsxhYKZT3JGoKJqYGwFmkunKKZOXw:1789287729279&q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+deb+Package+Manager?&spell=1&dpr=1) позволят запустить лаунчер на любой платформе основанной на Linux без трудностей.

[`Community Shader`](https://www.nexusmods.com/skyrimspecialedition/mods/86492) по-прежнему остаётся в лаунчере несмотря на отказ от зеркал.

Launcher привязывается к одной копии игры. Копию игры можно поменять в файле конфигурации `~/.config/rfad-launcher/launcher.conf`.
_Если лаунчер не сможет найти игру по пути из конфига он заново выдаст диалоговое окно установки._
Эта условность появляется из-за [`rpm`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+RPM+Package+Manage%3F&sxsrf=APpeQnuCLV0ulzDEaktzsU8ABePespf48A%3A1789287692406&uact=5) и [`deb`](https://www.google.com/search?sxsrf=APpeQnsxhYKZT3JGoKJqYGwFmkunKKZOXw:1789287729279&q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+deb+Package+Manager?&spell=1&dpr=1) пакетов.
В целом большинству людей больше 1 копии не требуется.

Теперь лаунчер предлагает выбрать расположение уже установленной игры(_не рекомендуется если установка была не через лаунчер_).
А также лаунчер после установки или указания пути к игре создаст ярлык в меню пуск.
При установке игры теперь можно портировать все свои конфиги и моды, это делается почти моментально если вы устанавливаете новую копию игры на один накопитель со старой копией игры.

### Warn: Перенос в пределах одного накопителя не расходует место на диске, но вам всё равно нужно пространство на первичную распаковку игры. Перенос между разными накопителями расходует дисковое пространство. Во избежание повреждения данных символические ссылки (symlinks) при переносе не используются.

Установка обновлений [`RFAD`](https://skyrimrfad.com/) более не происходит при первом запуске, теперь нужно вручную нажать `обновить игру`. Это сделано для того чтобы при портировании существующих сохранений не сломать их новой версией [`RFAD`](https://skyrimrfad.com/).

## Minor

- В [`readme.md`](https://github.com/Kraito585/Rfad-launcher-linux-fork/blob/main/readme.md) добавлен подробный гайд по билду проекта.
- Удалены некоторые повторяющиеся функции.
- Рефакторинг расположения некоторых функции.
- Зачищены заглушки в go старых не используемых мною вызавов из WebUI [`Amirust`](https://github.com/Amirust/rfad-launcher).
- Удалены неиспользуемые вызовы из vue WebUI [`Amirust`](https://github.com/Amirust/rfad-launcher).
- Удалены остаточные конфигурации [`Taury`](https://v2.tauri.app/) из WebUI [`Amirust`](https://github.com/Amirust/rfad-launcher).
- Функции `src-wails/utils/utils.go: CopyDir(), CopyFile()` теперь используют [`hard links`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+hard+link%3F&sxsrf=APpeQns_wmJkfKOnCEKMZWIlhj53Okndjw%3A1789287355222&uact=5) там где это возможно.
  Значительно ускорояет копирование файлов и уменьшая вероятность повреждения данных.
- Добавлена конфигурация `.vscode` [`Prettier`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+prettier&sxsrf=APpeQnufV_ou0FT5RhNwAgGAN67oWHhCsw%3A1789296105098&uact=5) для vue кода.
- Отказ от сервисного аккаунта Google в пользу API ключа.
