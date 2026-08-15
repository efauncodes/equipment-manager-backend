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

Der Smoke-Test akzeptiert eine echte HTTPS-Basis-URL und verwendet keine
localhost-Annahme:

```sh
BASE_URL=https://api-staging.example.org \
ADMIN_SESSION_TOKEN='<session-from-staging-mail>' \
./../../scripts/remote-smoke-test.sh
```

Mit `RUN_FULL_FLOW=true` prüft das Skript zusätzlich den vollständigen
Ausgabe-/Rückgabeablauf. Dafür wird ein bereits angelegtes Testmitglied und
dessen Session aus dem geschützten Staging-Mailweg benötigt; die Confirm-
Endpunkte werden ohne Body über diese Member-Session aufgerufen:

```sh
BASE_URL=https://api-staging.example.org \
ADMIN_SESSION_TOKEN='<admin-session>' \
MEMBER_ID='<test-member-id>' \
MEMBER_SESSION_TOKEN='<member-session>' \
RUN_FULL_FLOW=true \
./../../scripts/remote-smoke-test.sh
```

Das Skript benötigt `curl` und `jq`. Ohne die erforderlichen Tokens bricht es
absichtlich mit einer klaren Anweisung ab, statt Development-Tokens zu
aktivieren.
