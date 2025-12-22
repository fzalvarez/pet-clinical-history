-- 001_init.sql
-- Postgres schema for pet-clinical-history (MVP)

BEGIN;

CREATE TABLE IF NOT EXISTS pets (
  id            text PRIMARY KEY,
  owner_user_id text NOT NULL,

  name      text NOT NULL,
  species   text NOT NULL DEFAULT '',
  breed     text NOT NULL DEFAULT '',
  sex       text NOT NULL DEFAULT '',
  birth_date date NULL,
  microchip text NOT NULL DEFAULT '',
  notes     text NOT NULL DEFAULT '',

  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pets_owner_user_id ON pets(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_pets_created_at ON pets(created_at);

CREATE TABLE IF NOT EXISTS pet_events (
  id          text PRIMARY KEY,
  pet_id      text NOT NULL REFERENCES pets(id) ON DELETE CASCADE,

  type        text NOT NULL,
  occurred_at timestamptz NOT NULL,
  recorded_at timestamptz NOT NULL,

  title       text NOT NULL DEFAULT '',
  notes       text NOT NULL DEFAULT '',

  actor_type  text NOT NULL,
  actor_id    text NOT NULL,

  source      text NOT NULL DEFAULT 'manual',
  visibility  text NOT NULL DEFAULT 'shared',

  status      text NOT NULL DEFAULT 'active'
);

CREATE INDEX IF NOT EXISTS idx_events_pet_occurred_at ON pet_events(pet_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_pet_recorded_at ON pet_events(pet_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_type ON pet_events(type);

CREATE TABLE IF NOT EXISTS access_grants (
  id             text PRIMARY KEY,
  pet_id         text NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
  owner_user_id  text NOT NULL,
  grantee_user_id text NOT NULL,

  scopes   text[] NOT NULL DEFAULT '{}',
  status   text NOT NULL DEFAULT 'invited',

  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  revoked_at timestamptz NULL
);

CREATE INDEX IF NOT EXISTS idx_grants_pet_id ON access_grants(pet_id);
CREATE INDEX IF NOT EXISTS idx_grants_grantee ON access_grants(grantee_user_id);
CREATE INDEX IF NOT EXISTS idx_grants_owner ON access_grants(owner_user_id);

-- fast lookup for active grants
CREATE INDEX IF NOT EXISTS idx_grants_active_lookup
  ON access_grants(pet_id, grantee_user_id, updated_at DESC)
  WHERE status = 'active';

COMMIT;
