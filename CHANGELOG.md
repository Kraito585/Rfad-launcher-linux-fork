## Основные изменения

- Фрэймоврк wails обвновлён `v2 -> v3`.
- Все новые версии будут публиковатся [`AppImage`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+AppImage%3F&sxsrf=APpeQntg0kXMvyQ9nv5v2snUp9C_h3jMqQ%3A1789287370533&uact=5), [`rpm`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+RPM+Package+Manage%3F&sxsrf=APpeQnuCLV0ulzDEaktzsU8ABePespf48A%3A1789287692406&uact=5), [`deb`](https://www.google.com/search?sxsrf=APpeQnsxhYKZT3JGoKJqYGwFmkunKKZOXw:1789287729279&q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+deb+Package+Manager?&spell=1&dpr=1).
- Удалена загрузка из [`зеркала CDN`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+%D0%B7%D0%B5%D1%80%D0%BA%D0%B0%D0%BB%D0%BE+%D0%B4%D0%B0%D0%BD%D0%BD%D1%8B%D0%B7&sxsrf=APpeQnv1E3f_HZnPe_ypzqyIyhUKFIzvXQ%3A1789287908092&uact=5) и все её упомнинания в коде.
- Загрузка [`Community Shader`](https://www.nexusmods.com/skyrimspecialedition/mods/86492) болие не зависит от зеркал.
- Конфигурация и расположение файлов игры теперь хронятся в `~/.config/rfad-launcher/launcher.conf`.
- Небольшое нововидение в установку.
- Значительно обновление окна установки.

## Подробние об обновлении

Обновление wails привело к обновлению зависимости `webkit2gtk-4.1 -> webkitgtk-6.0`. Основная причина обновления возможность нативного билда приложения под Linux в формате [`AppImage`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+AppImage%3F&sxsrf=APpeQntg0kXMvyQ9nv5v2snUp9C_h3jMqQ%3A1789287370533&uact=5), [`rpm`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+RPM+Package+Manage%3F&sxsrf=APpeQnuCLV0ulzDEaktzsU8ABePespf48A%3A1789287692406&uact=5), [`deb`](https://www.google.com/search?sxsrf=APpeQnsxhYKZT3JGoKJqYGwFmkunKKZOXw:1789287729279&q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+deb+Package+Manager?&spell=1&dpr=1).

Форматы [`AppImage`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+AppImage%3F&sxsrf=APpeQntg0kXMvyQ9nv5v2snUp9C_h3jMqQ%3A1789287370533&uact=5), [`rpm`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+RPM+Package+Manage%3F&sxsrf=APpeQnuCLV0ulzDEaktzsU8ABePespf48A%3A1789287692406&uact=5), [`deb`](https://www.google.com/search?sxsrf=APpeQnsxhYKZT3JGoKJqYGwFmkunKKZOXw:1789287729279&q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+deb+Package+Manager?&spell=1&dpr=1) позволят запустить лаунчер на любой платформе основоной на Linux без трудностей.

[`Community Shader`](https://www.nexusmods.com/skyrimspecialedition/mods/86492) по прежднему остаётся в лаунчере не смотря на отказ от зеркал.

Launcher привязывается к одной копии игры. Копию игры можно поменять в файле конфигурации `~/.config/rfad-launcher/launcher.conf`.
_Если лаунчер не сможет найти игру по пути у конфига он заного выдаст дилоговое окно установки._
Эта условность появляется из за [`rpm`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+RPM+Package+Manage%3F&sxsrf=APpeQnuCLV0ulzDEaktzsU8ABePespf48A%3A1789287692406&uact=5) и [`deb`](https://www.google.com/search?sxsrf=APpeQnsxhYKZT3JGoKJqYGwFmkunKKZOXw:1789287729279&q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+deb+Package+Manager?&spell=1&dpr=1) пакетов.
В целом большенсту людей больше 1 копии не требуется.

Теперь лаунчер предлогает выбрать расположение уже установленой игры(_не рекомендуется если установка была не через лаунчер_).
При установки игры теперь можно портировтаь все свои конфиги и моды.
А также лаунчер после установки или указния пути к игре создаст ярлык в меню пуск и на рабочем столе.

## Minore

- В [`readme.md`](https://github.com/Kraito585/Rfad-launcher-linux-fork/blob/main/readme.md) добавлен подробный гайд по билду проекта.
- Удалены некоторые повторяющиеся функции.
- Рефакторинг расположения некоторых функции.
- Зачищены заглушки в go старых не используемых мною вызавов из WebUI [`Amirust`](https://github.com/Amirust/rfad-launcher).
- Удалены неиспользуемые вызовы из vue WebUI [`Amirust`](https://github.com/Amirust/rfad-launcher).
- Функции `src-wails/utils/utils.go: CopyDir(), CopyFile()` теперь используют [`hard links`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+hard+link%3F&sxsrf=APpeQns_wmJkfKOnCEKMZWIlhj53Okndjw%3A1789287355222&uact=5) там где это возможно. Значительно ускорояет копирование файлов и уменьшая вероятность повреждения данных.
- Добавлена конфигурация `.vscode` [`Prettier`](https://www.google.com/search?q=%D1%87%D1%82%D0%BE+%D1%82%D0%B0%D0%BA%D0%BE%D0%B5+prettier&sxsrf=APpeQnufV_ou0FT5RhNwAgGAN67oWHhCsw%3A1789296105098&uact=5) для vue кода.
