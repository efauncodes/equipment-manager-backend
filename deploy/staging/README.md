# Staging Deployment

Diese Referenzinstallation stellt den Backend-Container ausschließlich über
Caddy auf TCP 80/443 bereit. Der Backend-Port 8080 ist nur im Compose-Netz
erreichbar. Caddy verwaltet Zertifikate automatisch für den konfigurierten
DNS-Namen.

## Voraussetzungen

- leerer Linux-Host mit Docker Engine und Compose v2
- DNS-A- oder AAAA-Record für `STAGING_DOMAIN` auf den Host
- eingehende TCP-Ports 80 und 443, sonst keine öffentliche Backend-Portfreigabe
- `curl`, `jq` und `sqlite3` für die Betriebs- und Smoke-Test-Helfer
- vom Product Owner bereitgestellte Admin-Adresse und ein sicherer Staging-
  Mailweg aus Issue #5 oder ein gleichwertiger geschützter Mail-Catcher

## Installation

```sh
git clone https://github.com/efauncodes/equipment-manager-backend.git
cd equipment-manager-backend/deploy/staging
cp .env.example .env
chmod 600 .env
$EDITOR .env
mkdir -p "$(sed -n 's/^STAGING_DATA_DIR=//p' .env)"
docker compose --env-file .env config
docker compose --env-file .env up -d --build
docker compose --env-file .env ps
```

`STAGING_DOMAIN` muss bereits auf den Host zeigen, damit Caddy das TLS-
Zertifikat ausstellen kann. `DEV_EXPOSE_TOKENS` ist im Staging-Compose fest auf
`false` gesetzt und wird nicht aus `.env` übernommen. Der Server darf keine
Development-Token ausgeben.

## Betrieb

```sh
docker compose --env-file .env ps
docker compose --env-file .env logs --tail=100 backend
docker compose --env-file .env logs --tail=100 caddy
docker compose --env-file .env restart
docker compose --env-file .env pull
docker compose --env-file .env up -d --build
```

Vor jedem Image-Update muss ein Backup erstellt werden. SQLite-Migrationen
laufen beim Backend-Start; ein Rollback auf ein älteres Image darf erst nach
Prüfung der Migrationskompatibilität erfolgen.

## Backup und Restore

Die Datenbank liegt auf dem Host unter
`${STAGING_DATA_DIR}/equipment.db`. Der Backup-Helfer verwendet SQLite `.backup`
für einen konsistenten Snapshot:

```sh
../../scripts/staging-backup.sh
```

Für einen Restore den Stack stoppen, die Zieldatei durch den geprüften Snapshot
ersetzen und den Stack wieder starten:

```sh
docker compose --env-file .env down
cp /srv/equipment-manager/backups/equipment-YYYYMMDD-HHMMSS.db "$STAGING_DATA_DIR/equipment.db"
docker compose --env-file .env up -d
```

Der Rollback eines Releases besteht aus dem Start des zuvor markierten Image-
Commits und einem anschließenden Remote-Smoke-Test. Die Datenbankdatei wird
dabei nicht gelöscht oder durch ein Compose-Volume ersetzt.

## Remote-Test

Der Smoke-Test prüft immer read-only `/healthz`, `/readyz`, einen erlaubten
CORS-Preflight für `/api/v1/me` sowie die read-only API-Listen. Die
Staging-Basis-URL muss HTTPS verwenden und wird niemals beschrieben:

```sh
BASE_URL=https://equipment-api.sentient-octopus.dev \
ADMIN_SESSION_TOKEN='<session-from-staging-mail>' \
./../../scripts/remote-smoke-test.sh
```

Der schreibende Full Flow ist fail-closed und darf ausschließlich gegen ein
explizit bereitgestelltes, separates Disposable-Testziel laufen. Dafür müssen
`DISPOSABLE_TEST_BASE_URL`, `TEST_RUN_ID`, ein Disposable-Admin-Token, ein
Disposable-Testmitglied und dessen Member-Session gesetzt sein. Der
`TEST_RUN_ID` wird in die erzeugten Testdaten geschrieben; das Disposable-Ziel
muss nach dem Lauf samt temporärer Datenbank verworfen werden:

```sh
BASE_URL=https://equipment-api.example.org \
ADMIN_SESSION_TOKEN='<admin-session>' \
DISPOSABLE_TEST_BASE_URL='http://127.0.0.1:18082' \
DISPOSABLE_ADMIN_SESSION_TOKEN='<disposable-admin-session>' \
DISPOSABLE_MEMBER_ID='<disposable-test-member-id>' \
DISPOSABLE_MEMBER_SESSION_TOKEN='<disposable-member-session>' \
TEST_RUN_ID='qa-20260907-01' \
RUN_FULL_FLOW=true \
./../../scripts/remote-smoke-test.sh
```

Ohne Disposable-Ziel oder `TEST_RUN_ID` bricht das Skript vor jedem Netzwerk-
und Schreibzugriff ab. Das Skript benötigt `curl`, `grep` und für den
Disposable-Full-Flow zusätzlich `jq`.
