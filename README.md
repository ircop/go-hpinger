# go-hpinger
hydra equipment pinger written on golang

Замена https://github.com/ircop/hpinger

Работает быстрее и качественнее предшественника.

Отсутствуют false-positives, иногда проявлявшиеся в hpingeer

Один raw-сокет вместо сотен потоков.

--


Параметры конфига:

- switch_root_id - ID позиции "Коммутатор" в номенклатуре, которая является родительской для пингуемых свитчей.
- alive_param_id - ID доп.параметра для коммутаторов, определяющего, "живой" он, или нет. Тип - флаг. Как-то так: <img src="http://i.imgur.com/OZR5r2Z.png">
