PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS locations (
	id   INTEGER PRIMARY KEY,
	name TEXT UNIQUE NOT NULL
);

CREATE INDEX IF NOT EXISTS location_by_name_index ON locations(name);

CREATE TABLE IF NOT EXISTS sensors (
	id   INTEGER PRIMARY KEY,
	name TEXT UNIQUE NOT NULL
);

CREATE INDEX IF NOT EXISTS sensor_by_name_index ON sensors(name);

CREATE TABLE IF NOT EXISTS data (
	id        INTEGER PRIMARY KEY,
	timestamp INTEGER NOT NULL,
	location  INTEGER NOT NULL,
	sensor    INTEGER NOT NULL,
	value     REAL NOT NULL,
	FOREIGN KEY(location) REFERENCES locations(id),
	FOREIGN KEY(sensor)   REFERENCES sensors(id)
);

CREATE INDEX IF NOT EXISTS main_data_index ON data(timestamp, sensor, location);
