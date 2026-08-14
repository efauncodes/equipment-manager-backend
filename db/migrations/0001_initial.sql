CREATE TABLE users (
  id TEXT PRIMARY KEY NOT NULL,
  email TEXT NOT NULL,
  display_name TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('admin', 'mitglied')),
  is_active INTEGER NOT NULL CHECK (is_active IN (0, 1)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE equipment (
  id TEXT PRIMARY KEY NOT NULL,
  serial_number TEXT NOT NULL,
  type TEXT NOT NULL,
  size TEXT NOT NULL,
  purchase_date TEXT,
  manufacturer TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('on_stock', 'issued', 'lost', 'damaged', 'written_off')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE issuances (
  id TEXT PRIMARY KEY NOT NULL,
  equipment_id TEXT NOT NULL REFERENCES equipment(id),
  member_id TEXT NOT NULL REFERENCES users(id),
  issued_by_user_id TEXT NOT NULL REFERENCES users(id),
  issued_at TEXT,
  confirmation_status TEXT NOT NULL CHECK (confirmation_status IN ('pending_confirmation', 'confirmed', 'cancelled')),
  member_confirmed_at TEXT,
  returned_at TEXT,
  closed_at TEXT,
  closure_reason TEXT CHECK (closure_reason IS NULL OR closure_reason IN ('returned', 'lost', 'damaged')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE returns (
  id TEXT PRIMARY KEY NOT NULL,
  issuance_id TEXT NOT NULL UNIQUE REFERENCES issuances(id),
  initiated_by_user_id TEXT NOT NULL REFERENCES users(id),
  returned_at TEXT,
  confirmation_status TEXT NOT NULL CHECK (confirmation_status IN ('pending_confirmation', 'confirmed', 'cancelled')),
  member_confirmed_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE magic_links (
  id TEXT PRIMARY KEY NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL,
  purpose TEXT NOT NULL CHECK (purpose IN ('login', 'confirm')),
  issuance_id TEXT REFERENCES issuances(id),
  return_id TEXT REFERENCES returns(id),
  expires_at TEXT NOT NULL,
  used_at TEXT,
  created_at TEXT NOT NULL,
  CHECK (
    (purpose = 'login' AND issuance_id IS NULL AND return_id IS NULL)
    OR
    (purpose = 'confirm' AND ((issuance_id IS NOT NULL AND return_id IS NULL) OR (issuance_id IS NULL AND return_id IS NOT NULL)))
  )
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY NOT NULL,
  user_id TEXT NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  revoked_at TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE equipment_history (
  id TEXT PRIMARY KEY NOT NULL,
  equipment_id TEXT NOT NULL REFERENCES equipment(id),
  event_type TEXT NOT NULL CHECK (event_type IN ('created', 'status_changed', 'issued', 'returned', 'written_off')),
  from_status TEXT CHECK (from_status IS NULL OR from_status IN ('on_stock', 'issued', 'lost', 'damaged', 'written_off')),
  to_status TEXT CHECK (to_status IS NULL OR to_status IN ('on_stock', 'issued', 'lost', 'damaged', 'written_off')),
  issuance_id TEXT REFERENCES issuances(id),
  return_id TEXT REFERENCES returns(id),
  changed_by_user_id TEXT NOT NULL REFERENCES users(id),
  occurred_at TEXT NOT NULL,
  note TEXT
);

CREATE UNIQUE INDEX users_email_ci ON users(lower(email));
CREATE UNIQUE INDEX equipment_serial_number ON equipment(serial_number);
CREATE UNIQUE INDEX magic_links_token_hash ON magic_links(token_hash);
CREATE UNIQUE INDEX sessions_token_hash ON sessions(token_hash);
CREATE UNIQUE INDEX returns_issuance_id ON returns(issuance_id);
CREATE UNIQUE INDEX one_active_issuance_per_equipment ON issuances(equipment_id)
  WHERE confirmation_status <> 'cancelled' AND closed_at IS NULL;
CREATE INDEX equipment_history_equipment_occurred ON equipment_history(equipment_id, occurred_at);
CREATE INDEX issuances_equipment ON issuances(equipment_id);
CREATE INDEX sessions_user ON sessions(user_id);
