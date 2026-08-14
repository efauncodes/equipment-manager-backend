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

## Entwicklungsprozess

Manager erstellt Issues -> Product Owner setzt `ready-for-dev` -> Entwickler arbeitet auf Feature-Branch -> QA prüft -> Product Owner merged nach `test` -> erfolgreicher Test wird nach `main` promoted. Der Main-Merge verwendet `Closes #<issue-number>`.

Bei einem fehlgeschlagenen Test geht die Rückmeldung an den Manager; die nächste Iteration wird erst nach erneuter Product-Owner-Freigabe gestartet.
