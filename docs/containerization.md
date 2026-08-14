---
type: concept
title: Equipment Manager backend containerization
status: draft
---

# Containerisierung — Issue #1

Die erste technische Aufgabe ist ein reproduzierbares Backend-Container-Setup für Docker Desktop.

## Akzeptanzkriterien

- Dockerfile für das Go-Backend.
- Docker-Compose-Start ohne manuelle Infrastrukturarbeit.
- Dokumentierte Ports und Umgebungsvariablen.
- Healthcheck und deterministische Testdaten.
- Kurzer lokaler API-Smoke-Test.
- Keine produktiven Secrets oder Daten.
- Der Start ist gemeinsam mit dem separaten Flutter-Frontend integrierbar.

## Technischer Vertrag

Das Image wird in zwei Stufen gebaut: Der Go-Binary entsteht reproduzierbar in
`golang:1.19-alpine`; das Laufzeitimage enthält nur das Binary und Alpine mit
dem unprivilegierten Benutzer `app`.

Compose startet den Service `backend` auf Port `8080`. Der Host-Port ist über
`BACKEND_PORT` konfigurierbar und verwendet standardmäßig ebenfalls `8080`.
Die einzige notwendige Umgebungsvariable im Container ist `HTTP_ADDR`; Compose
setzt sie auf `:8080`.

`GET /healthz` liefert HTTP 200 mit `{"status":"ok"}`. Docker und Compose
verwenden diesen Endpoint für den Healthcheck. `GET /` bleibt als einfacher
erreichbarer Service-Endpunkt verfügbar.

## Befehle

```sh
# Start und Build
docker compose up --build

# Status inklusive Healthcheck
docker compose ps

# Lokaler API-Smoke-Test (startet und bereinigt den Stack selbst)
./scripts/smoke-test.sh

# Stoppen und Orphans bereinigen
docker compose down --remove-orphans
```

Es werden keine Datenbanken, Volumes, Secrets oder externen Infrastruktur-
abhängigkeiten eingeführt.
