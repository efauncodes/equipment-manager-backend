---
type: concept
title: Equipment Manager backend decisions
status: draft
---

# Entscheidungen

## 0001 — Getrennte Repository-Grenze

Backend und Flutter-Frontend werden in getrennten Repositories entwickelt und über einen dokumentierten API-Vertrag integriert.

## 0002 — Container zuerst

Docker-Desktop-Start, Healthcheck und Smoke-Test werden vor externer CI/CD und Cloud-Deployment fertiggestellt.

## 0003 — Test vor Main

Nur ein erfolgreicher Product-Owner-Merge nach `test` darf nach `main` promoted werden; erst der Main-Merge schließt das Issue.
