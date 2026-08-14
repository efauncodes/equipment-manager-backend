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
