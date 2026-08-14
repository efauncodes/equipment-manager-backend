---
type: concept
title: Equipment Manager backend architecture
status: draft
---

# Architektur

- Technologie: Golang.
- Repository-Grenze: Backend/API/Services only.
- Das Flutter-Frontend liegt in `equipment-manager-frontend`.
- API-Vertrag, Ports, Healthcheck und persistente Abhängigkeiten werden in Issue #1 und den Folge-Issues festgelegt.
- Keine produktiven Daten oder Secrets in diesem Repository.
