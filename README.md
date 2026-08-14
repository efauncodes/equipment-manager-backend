# Equipment Manager Backend

Go-Backend des Equipment Manager Produkts.

## Projektstatus

Das Repository enthält die SQLite-Persistenz aus Issue #4. Das Datenbankmodell
und die Geschäftsregeln folgen der Referenz aus Issue #3. HTTP-Endpunkte sind
weiterhin nicht Teil dieses Issues.

## Wiki

- [Wiki-Index](docs/index.md)
- [Architektur](docs/architecture.md)
- [Betrieb und Workflow](docs/operations.md)
- [Containerisierung](docs/containerization.md)
- [Entscheidungen](docs/decisions.md)
- [MVP-Domänenmodell](docs/mvp-domain-model.md)

## Persistenz lokal starten

```sh
docker compose up --build
```

Die Datenbank wird beim Start migriert und liegt persistent in
`/app/data/equipment.db` innerhalb des Compose-Volumes `equipment-data`.

## Zuständigkeit

Miharu koordiniert Anforderungen und Issues. Go-Entwicklung und QA laufen über die gebundenen Studio-Rollen. `ready-for-dev` wird ausschließlich vom Product Owner gesetzt.
