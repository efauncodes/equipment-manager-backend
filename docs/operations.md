---
type: concept
title: Equipment Manager backend operations
status: draft
---

# Betrieb und Workflow

## Lokaler Start

Voraussetzungen sind Docker Desktop mit Compose v2 sowie `curl` für den
Host-Smoke-Test. Der Stack startet mit `docker compose up --build`; danach
ist `http://127.0.0.1:8080/healthz` erreichbar. Mit `BACKEND_PORT=8081` kann
der Host-Port geändert werden. Die vollständige Befehlsreferenz steht in der
[Containerisierungsdokumentation](containerization.md).

Der Smoke-Test `./scripts/smoke-test.sh` baut den Stack, wartet bis `/healthz`
antwortet und fährt den Stack auch bei einem Fehlschlag wieder herunter.

Für die Persistenz aus Issue #4 öffnet der HTTP-Server beim Start die Datenbank
und führt die versionierten SQLite-Migrationen aus. Die Datenbank liegt im Container unter
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
