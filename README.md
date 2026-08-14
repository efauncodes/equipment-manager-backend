# Equipment Manager Backend

Go-Backend des Equipment Manager Produkts.

## Projektstatus

Das Repository enthält den containerisierten Go-HTTP-Server und die
SQLite-Persistenz. Das Datenbankmodell und die Geschäftsregeln folgen der
Referenz aus Issue #3.

## Wiki

- [Wiki-Index](docs/index.md)
- [Architektur](docs/architecture.md)
- [Betrieb und Workflow](docs/operations.md)
- [Containerisierung](docs/containerization.md)
- [Entscheidungen](docs/decisions.md)
- [MVP-Domänenmodell](docs/mvp-domain-model.md)

## Lokaler Start mit Docker

Voraussetzungen: Docker Desktop mit Compose v2 und `curl`.

```sh
docker compose up --build
curl --fail http://127.0.0.1:8080/healthz
```

Der Backend-Port kann mit `BACKEND_PORT=8081 docker compose up --build` geändert
werden. Der Container meldet sich über `/healthz` als healthy; der Endpoint
antwortet mit `{"status":"ok"}`. Der reproduzierbare Smoke-Test ist:

```sh
./scripts/smoke-test.sh
```

Die SQLite-Datenbank wird beim Start migriert und liegt unter
`/app/data/equipment.db` im benannten Compose-Volume `equipment-data`. Dadurch
bleiben Daten bei Container-Neustarts und Image-Neubauten erhalten. Ein
abweichender Datenbankpfad kann über `EQUIPMENT_DB_PATH` konfiguriert werden.

## Zuständigkeit

Miharu koordiniert Anforderungen und Issues. Go-Entwicklung und QA laufen über die gebundenen Studio-Rollen. `ready-for-dev` wird ausschließlich vom Product Owner gesetzt.
