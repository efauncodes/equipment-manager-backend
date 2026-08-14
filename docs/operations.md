---
type: concept
title: Equipment Manager backend operations
status: draft
---

# Betrieb und Workflow

## Lokaler Start

Issue #1 definiert den reproduzierbaren Docker-Desktop-Start. Ports, Umgebungsvariablen, Healthcheck und Smoke-Test werden dort verbindlich dokumentiert.

Für die Persistenz aus Issue #4 startet `docker compose up --build` den
endpoint-freien Backend-Prozess und führt die versionierten SQLite-Migrationen
beim Öffnen der Datenbank aus. Die Datenbank liegt im Container unter
`/app/data/equipment.db`; Compose bindet dieses Verzeichnis an das benannte
Volume `equipment-data`. Das Volume darf nicht durch einen Container-Neustart
oder ein neues Image ersetzt werden. Für lokale Tests kann `EQUIPMENT_DB_PATH`
auf einen anderen Dateipfad gesetzt werden.

```sh
docker compose up --build
docker compose down
```

`docker compose down -v` löscht das benannte Persistenz-Volume und damit die
lokalen Daten ausdrücklich; dieser Befehl ist nicht Teil des normalen Starts.

## Entwicklungsprozess

Manager erstellt Issues -> Product Owner setzt `ready-for-dev` -> Entwickler arbeitet auf Feature-Branch -> QA prüft -> Product Owner merged nach `test` -> erfolgreicher Test wird nach `main` promoted. Der Main-Merge verwendet `Closes #<issue-number>`.

Bei einem fehlgeschlagenen Test geht die Rückmeldung an den Manager; die nächste Iteration wird erst nach erneuter Product-Owner-Freigabe gestartet.
