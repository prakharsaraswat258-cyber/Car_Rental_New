-- Saarthi schema. Runs on every start; everything is IF NOT EXISTS.
-- Times: UTC text '2006-01-02T15:04:05Z' (string order = time order).
-- Money: INTEGER paise.

CREATE TABLE IF NOT EXISTS users (
  id             TEXT PRIMARY KEY,
  role           TEXT NOT NULL CHECK (role IN ('renter','provider','admin')),
  name           TEXT NOT NULL,
  email          TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash  TEXT NOT NULL,
  phone          TEXT NOT NULL DEFAULT '',
  licence_status TEXT NOT NULL DEFAULT 'pending' CHECK (licence_status IN ('pending','verified','rejected')),
  created_at     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  token      TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS cities (
  id      TEXT PRIMARY KEY,
  name    TEXT NOT NULL,
  slug    TEXT NOT NULL UNIQUE,
  aliases TEXT NOT NULL DEFAULT '', -- comma list, lower case (D-013)
  lat     REAL NOT NULL,
  lng     REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS areas (
  id      TEXT PRIMARY KEY,
  city_id TEXT NOT NULL REFERENCES cities(id) ON DELETE CASCADE,
  name    TEXT NOT NULL,
  aliases TEXT NOT NULL DEFAULT '', -- comma list, lower case
  lat     REAL NOT NULL,
  lng     REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS areas_city ON areas(city_id);

CREATE TABLE IF NOT EXISTS shops (
  id         TEXT PRIMARY KEY,
  owner_id   TEXT NOT NULL REFERENCES users(id),
  name       TEXT NOT NULL,
  slug       TEXT NOT NULL UNIQUE,
  phone      TEXT NOT NULL DEFAULT '',
  whatsapp   TEXT NOT NULL DEFAULT '', -- digits only, with country code
  address    TEXT NOT NULL DEFAULT '',
  city_id    TEXT NOT NULL REFERENCES cities(id),
  area_id    TEXT REFERENCES areas(id),
  lat        REAL NOT NULL,
  lng        REAL NOT NULL,
  status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified','suspended')),
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS shops_owner ON shops(owner_id);
CREATE INDEX IF NOT EXISTS shops_city ON shops(city_id);

CREATE TABLE IF NOT EXISTS vehicles (
  id                      TEXT PRIMARY KEY,
  shop_id                 TEXT NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
  type                    TEXT NOT NULL CHECK (type IN ('scooter','bike','car','ev','cycle')),
  make                    TEXT NOT NULL,
  model                   TEXT NOT NULL,
  year                    INTEGER NOT NULL DEFAULT 0,
  reg_number              TEXT NOT NULL DEFAULT '',
  fuel                    TEXT NOT NULL DEFAULT '', -- petrol|diesel|electric|none
  transmission            TEXT NOT NULL DEFAULT '', -- manual|automatic|none
  seats                   INTEGER NOT NULL DEFAULT 2,
  features                TEXT NOT NULL DEFAULT '', -- comma list
  description             TEXT NOT NULL DEFAULT '',
  hourly_paise            INTEGER NOT NULL CHECK (hourly_paise >= 0),
  daily_paise             INTEGER NOT NULL CHECK (daily_paise >= 0),
  deposit_paise           INTEGER NOT NULL DEFAULT 0 CHECK (deposit_paise >= 0),
  km_limit_per_day        INTEGER NOT NULL DEFAULT 0, -- 0 = unlimited
  extra_km_paise          INTEGER NOT NULL DEFAULT 0,
  late_fee_per_hour_paise INTEGER NOT NULL DEFAULT 0,
  min_hours               INTEGER NOT NULL DEFAULT 1,
  instant_book            INTEGER NOT NULL DEFAULT 1,
  status                  TEXT NOT NULL DEFAULT 'live' CHECK (status IN ('draft','live','hidden')),
  odometer_km             INTEGER NOT NULL DEFAULT 0,
  rating_avg              REAL NOT NULL DEFAULT 0,
  rating_count            INTEGER NOT NULL DEFAULT 0,
  created_at              TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS vehicles_shop ON vehicles(shop_id);
CREATE INDEX IF NOT EXISTS vehicles_type ON vehicles(type, status);

CREATE TABLE IF NOT EXISTS vehicle_photos (
  id         TEXT PRIMARY KEY,
  vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
  path       TEXT NOT NULL, -- relative to data/uploads
  position   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS vehicle_photos_vehicle ON vehicle_photos(vehicle_id, position);

CREATE TABLE IF NOT EXISTS addons (
  id          TEXT PRIMARY KEY,
  shop_id     TEXT NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  price_paise INTEGER NOT NULL CHECK (price_paise >= 0),
  per         TEXT NOT NULL CHECK (per IN ('trip','day')),
  fits        TEXT NOT NULL DEFAULT '' -- comma list of two_wheeler|four_wheeler|cycle; '' = all (D-014)
);
CREATE INDEX IF NOT EXISTS addons_shop ON addons(shop_id);

CREATE TABLE IF NOT EXISTS blocks (
  id         TEXT PRIMARY KEY,
  vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
  start_at   TEXT NOT NULL,
  end_at     TEXT NOT NULL,
  note       TEXT NOT NULL DEFAULT '',
  CHECK (end_at > start_at)
);
CREATE INDEX IF NOT EXISTS blocks_vehicle ON blocks(vehicle_id, start_at);

CREATE TABLE IF NOT EXISTS bookings (
  id             TEXT PRIMARY KEY,
  code           TEXT NOT NULL UNIQUE,
  renter_id      TEXT NOT NULL REFERENCES users(id),
  vehicle_id     TEXT NOT NULL REFERENCES vehicles(id),
  shop_id        TEXT NOT NULL REFERENCES shops(id),
  start_at       TEXT NOT NULL,
  end_at         TEXT NOT NULL,
  status         TEXT NOT NULL CHECK (status IN ('pending_payment','awaiting_shop','confirmed','ready','active','returned','closed','cancelled','declined','expired')),
  rent_paise     INTEGER NOT NULL,
  addons_paise   INTEGER NOT NULL DEFAULT 0,
  tax_paise      INTEGER NOT NULL,
  total_paise    INTEGER NOT NULL,
  deposit_paise  INTEGER NOT NULL,
  km_included    INTEGER NOT NULL DEFAULT 0,
  price_lines    TEXT NOT NULL DEFAULT '[]', -- JSON []domain.Line
  odometer_start INTEGER,
  odometer_end   INTEGER,
  returned_at    TEXT,
  created_at     TEXT NOT NULL,
  updated_at     TEXT NOT NULL,
  CHECK (end_at > start_at)
);
CREATE INDEX IF NOT EXISTS bookings_vehicle_time ON bookings(vehicle_id, start_at, end_at);
CREATE INDEX IF NOT EXISTS bookings_renter ON bookings(renter_id, start_at);
CREATE INDEX IF NOT EXISTS bookings_shop ON bookings(shop_id, start_at);
CREATE INDEX IF NOT EXISTS bookings_status ON bookings(status);

CREATE TABLE IF NOT EXISTS booking_events (
  id         TEXT PRIMARY KEY,
  booking_id TEXT NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
  status     TEXT NOT NULL,
  note       TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS booking_events_booking ON booking_events(booking_id, created_at);

CREATE TABLE IF NOT EXISTS trip_photos (
  id         TEXT PRIMARY KEY,
  booking_id TEXT NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
  phase      TEXT NOT NULL CHECK (phase IN ('checkin','return')),
  angle      TEXT NOT NULL CHECK (angle IN ('front','front_left','left','rear_left','rear','rear_right','right','front_right','odometer','fuel')),
  path       TEXT NOT NULL, -- relative to data/uploads
  taken_at   TEXT NOT NULL,
  UNIQUE (booking_id, phase, angle)
);

CREATE TABLE IF NOT EXISTS deposits (
  booking_id     TEXT PRIMARY KEY REFERENCES bookings(id) ON DELETE CASCADE,
  held_paise     INTEGER NOT NULL,
  deducted_paise INTEGER NOT NULL DEFAULT 0,
  status         TEXT NOT NULL CHECK (status IN ('held','inspection','released','credited')),
  released_at    TEXT,
  credited_at    TEXT
);

CREATE TABLE IF NOT EXISTS deductions (
  id           TEXT PRIMARY KEY,
  booking_id   TEXT NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
  kind         TEXT NOT NULL CHECK (kind IN ('extra_km','late','fuel','damage','cleaning')),
  amount_paise INTEGER NOT NULL CHECK (amount_paise >= 0),
  note         TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS deductions_booking ON deductions(booking_id);

CREATE TABLE IF NOT EXISTS reviews (
  id         TEXT PRIMARY KEY,
  booking_id TEXT NOT NULL UNIQUE REFERENCES bookings(id) ON DELETE CASCADE,
  author_id  TEXT NOT NULL REFERENCES users(id),
  vehicle_id TEXT NOT NULL REFERENCES vehicles(id),
  shop_id    TEXT NOT NULL REFERENCES shops(id),
  rating     INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
  body       TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS reviews_vehicle ON reviews(vehicle_id, created_at);
