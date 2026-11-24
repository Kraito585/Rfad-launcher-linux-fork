# Общее улучшение производительности около 30%

## Основные изменения

- Уход от GE-Proton.
- Уход от стороннего префикса.
- Функционал геймпадов восстановлен.
- Патч для префикса был обновлён.

## Подробнее об обновлении

### Уход от GE-Proton

Использование GE-Proton, а именно встроенного GE-Wine, изначально было ошибкой, поскольку он использует Open Source [`wine-mono`](https://github.com/wine-mono/wine-mono) вместо проприетарного .NET Framework, что крайне критично для модов Skyrim, которые часто используют специфические вызовы из фреймворка, которых попросту нет в [`wine-mono`](https://github.com/wine-mono/wine-mono).

### Уход от стороннего префикса

Мы изначально были заложниками стороннего префикса, поскольку именно он позволял обходить wine-mono, но он не решал многих проблем, связанных с GE-Proton.

В новом префиксе уже решены почти все зависимости, необходимые для сборки, а зависимости [`ENB`](http://enbdev.com/download_mod_tesskyrimse.html) лаунчер решает патчем при установке.

### Функционал геймпадов восстановлен

Патч, который я устанавливал в старый префикс, приводил к поломке совместимости с геймпадами.
Патч был изменён и более не ломает совместимость.

### Патч для префикса был обновлён

Раньше патч для префикса внедрял большой список библиотек, в их числе специфические библиотеки DX11 для [`ENB`](http://enbdev.com/download_mod_tesskyrimse.html) и свежий [`DXVK`](https://github.com/doitsujin/DXVK) для [`Community Shader`](https://www.nexusmods.com/skyrimspecialedition/mods/86492), а также несколько библиотек для лучшей совместимости.

Сейчас лаунчер при установке патчит только библиотеки для [`ENB`](http://enbdev.com/download_mod_tesskyrimse.html).

## Minor

- Пересборка на [`Wails3.0.7 Beta -> Wails3.1.0 Stable`](https://v3.wails.io/changelog/).
- Убрано назойливое окно MO2 при запуске игры.
  ![Здесь должно быть изображение](https://ik.imagekit.io/3o5rv0kou/Screenshot_20261124_204223.png)
- Исправлена ошибка, из-за которой при первом запуске игры происходил вылет. Ошибка была связана с тем, что в префиксе отсутствовали директории `drive_c/users/{username}/AppData/Local/Skyrim Special Edition` и `drive_c/users/{username}/Documents/My Games/Skyrim Special Edition`. [`src-wails/utils/recovery/recovery.go:RecoverPrefix()`](https://github.com/Kraito585/Rfad-launcher-linux-fork/blob/main/src-wails/utils/recovery/recovery.go#L53) теперь решает и эту проблему.
- Очередные множественные изменения в прослойке совместимости с Flatpak.

## Послесловие

Скачать Prefix и Wine для изучения или использования можно на главной странице репозитория.

Почему в префиксе нет библиотек для [`ENB`](http://enbdev.com/download_mod_tesskyrimse.html)? Я просто поленился вручную править реестр. [`winetricks`](https://github.com/winetricks/winetricks), на котором основан [`Protontricks`](https://github.com/matoking/protontricks), не имеет инструмента для инджекта определённого DLL-файла в префикс.

Архив с файлами для инджекта можно посмотреть [`здесь`](https://github.com/Kraito585/Rfad-launcher-linux-fork/blob/main/embed/libs.zip).

Список файлов, которые инджектит [`src-wails/utils/recovery/recovery.go:RecoverPrefix()`](https://github.com/Kraito585/Rfad-launcher-linux-fork/blob/main/src-wails/utils/recovery/recovery.go):

- D3DCompiler_43.dll
- d3dcompiler_46.dll
- d3dcompiler_46e.dll
- D3DX9_42.dll
- D3DX9_43.dll
- d3dx11_42.dll
- D3DX11_43.dll

Я знаю, что разработчики [`Wails3`](https://v3.wails.io/) в патче [`v310-nightly20261028---2026-10-28`](https://v3.wails.io/changelog/#v310-nightly20261028---2026-10-28) наконец смогли исправить важную проблему с AppImage, связанную с разными названиями lib-директорий на разных системах (`/usr/lib`, `/usr/lib64`, `/usr/libexec`), но по какой-то причине AppImage всё ещё не работает, причём падает без ошибок. Я провёл немного времени с попытками сбилдить рабочий AppImage, и всё безуспешно, так что в ближайшее время я не смогу его добавить, да и, насколько я понимаю, нужды в этом особо нет.
