# MVP API

Die API ist ein lokaler, testbarer HTTP-Vertical-Slice für den Equipment
Manager. Alle fachlichen Endpunkte verwenden `/api/v1`; `GET /healthz` ist
öffentlich.

## Lokaler Start

Für reproduzierbare Frontend- und Integrationstests wird ein Development-
Mailer verwendet. Der rohe Token wird ausschließlich ausgegeben, wenn beide
Schalter explizit gesetzt sind:

```sh
APP_ENV=development \
DEV_EXPOSE_TOKENS=true \
ADMIN_EMAILS=admin@example.com \
EQUIPMENT_DB_PATH=./equipment.db \
go run ./cmd/server
```

Für Docker Desktop:

```sh
ADMIN_EMAILS=admin@example.com docker compose up --build
curl --fail http://127.0.0.1:8080/healthz
```

`DEV_EXPOSE_TOKENS` darf in produktiven Umgebungen nicht gesetzt werden.
Dieser MVP enthält keinen produktiven SMTP- oder Provider-Mailer.

## Antwortformat

Erfolgreiche API-Antworten haben immer diese Form:

```json
{
  "data": {},
  "meta": {
    "request_id": "c19d…",
    "local_time": "2026-08-15 14:30:00"
  }
}
```

Fehler verwenden stabile Codes und dieselben Metadaten:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "request is invalid"
  },
  "meta": {
    "request_id": "c19d…",
    "local_time": "2026-08-15 14:30:00"
  }
}
```

Zeitstempel verwenden `Europe/Berlin` und `YYYY-MM-DD HH:mm:ss`; reine
Datumswerte verwenden `YYYY-MM-DD`. Ein eigener `X-Request-ID`-Header wird
übernommen, sofern er höchstens 128 Zeichen lang ist.

## Authentifizierung

1. `POST /api/v1/auth/admin/magic-links` mit einer Adresse aus `ADMIN_EMAILS`
   aufrufen.
2. Im Development-Modus den zurückgegebenen `token` an
   `POST /api/v1/auth/consume` senden.
3. `session_token` als `Authorization: Bearer <session_token>` verwenden.

Administratoren starten Verwaltungs- und Ausgabe-/Rückgabevorgänge. Mitglieder
bestätigen ausschließlich ihren eigenen Vorgang. Confirm-Links sind einmalig
und zehn Minuten gültig.

## Endpunkte

### Betrieb

`GET /healthz` ist öffentlich und liefert `{"status":"ok"}`.

### Authentifizierung

```sh
curl -X POST http://127.0.0.1:8080/api/v1/auth/admin/magic-links \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com"}'

curl -X POST http://127.0.0.1:8080/api/v1/auth/consume \
  -H 'Content-Type: application/json' \
  -d '{"token":"<development-token>"}'

curl http://127.0.0.1:8080/api/v1/me \
  -H 'Authorization: Bearer <session_token>'

curl -X POST http://127.0.0.1:8080/api/v1/auth/logout \
  -H 'Authorization: Bearer <session_token>'
```

Die Magic-Link-Anfrage antwortet auch bei unbekannter Adresse mit `202` und
`accepted: true`; sie verrät nicht, ob eine Adresse registriert ist.

### Mitglieder

Admin-only:

- `GET /api/v1/members`
- `POST /api/v1/members` mit `{ "email": "…", "display_name": "…", "is_active": true }`
- `GET /api/v1/members/{member_id}`
- `PATCH /api/v1/members/{member_id}` mit beliebigen Feldern aus dem Create-Request

### Equipment

Admin-only:

- `GET /api/v1/equipment`
- `POST /api/v1/equipment` mit `serial_number`, `type`, `size`, `manufacturer` und optionalem `purchase_date`
- `GET /api/v1/equipment/{equipment_id}`
- `PATCH /api/v1/equipment/{equipment_id}`
- `POST /api/v1/equipment/{equipment_id}/write-off` mit optionalem `{ "note": "…" }`
- `GET /api/v1/equipment/{equipment_id}/history`

Ausgebuchte Objekte sind standardmäßig nicht in der Liste. Mit
`?include_written_off=true` werden sie einbezogen. Der Write-off folgt den
Fachregeln aus #4 und ist für `lost` oder `damaged` möglich.

### Ausgabe und Rückgabe

Admin-only:

- `POST /api/v1/issuances` mit `{ "equipment_id": "…", "member_id": "…" }`
- `GET /api/v1/issuances`
- `POST /api/v1/returns` mit `{ "issuance_id": "…" }`
- `GET /api/v1/returns`

Die Create-Antworten enthalten im Development-Modus zusätzlich
`confirmation_token`. Dieser wird an den jeweiligen Confirm-Endpunkt gesendet:

```sh
curl -X POST http://127.0.0.1:8080/api/v1/issuances/<id>/confirm \
  -H 'Content-Type: application/json' \
  -d '{"token":"<issuance-confirm-token>"}'

curl -X POST http://127.0.0.1:8080/api/v1/returns/<id>/confirm \
  -H 'Content-Type: application/json' \
  -d '{"token":"<return-confirm-token>"}'
```

Alternativ kann ein eingeloggtes Mitglied den Confirm-Endpunkt ohne Body
aufrufen. Das Backend prüft dann die Zuordnung serverseitig. Doppelte
Bestätigungen, abgelaufene Tokens, falsche Mitglieder und parallele
Bestandsänderungen liefern strukturierte Konflikt-/Tokenfehler.

## Fehlercodes

| HTTP | Code | Bedeutung |
| --- | --- | --- |
| 400 | `VALIDATION_ERROR` | Ungültige JSON-, Pflichtfeld-, Datums- oder Query-Eingabe |
| 401 | `UNAUTHORIZED` | Session fehlt, ist ungültig oder widerrufen |
| 401 | `TOKEN_EXPIRED` | Magic Link oder Session ist abgelaufen |
| 403 | `FORBIDDEN` | Rolle darf den Endpunkt nicht verwenden |
| 403 | `INACTIVE_USER` | Benutzer ist deaktiviert |
| 404 | `NOT_FOUND` | ID oder Ressource ist unbekannt |
| 409 | `CONFLICT` | Fachlicher oder paralleler Zustandskonflikt |
| 409 | `TOKEN_USED` | Magic Link wurde bereits verwendet |
| 409 | `EQUIPMENT_UNAVAILABLE` | Equipment ist nicht ausgabefähig |

Die vollständigen Request-/Response-Schemas, Statuscodes und Berechtigungen
stehen in [openapi.yaml](openapi.yaml).
