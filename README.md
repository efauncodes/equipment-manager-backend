# Equipment Manager Backend

Go-Backend des Equipment Manager Produkts.

## Projektstatus

Das Repository ist frisch initialisiert. Issue #1 definiert die verbindliche Containerisierung als ersten technischen Schritt.

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

## Zuständigkeit

Miharu koordiniert Anforderungen und Issues. Go-Entwicklung und QA laufen über die gebundenen Studio-Rollen. `ready-for-dev` wird ausschließlich vom Product Owner gesetzt.
