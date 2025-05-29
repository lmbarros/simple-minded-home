use std::{thread::sleep, time::Duration};

use chrono::{TimeDelta, prelude::*};
use sqlite::Connection;

const DB_FILE: &str = "sensors.db";

fn main() {
    let _db = open_database();

    // create_fake_data(db, 365 * 50);

    println!("All done. Sleeping now...");

    loop {
        sleep(Duration::from_secs(3600));
    }
}

// Opens and returns the database connection. If needed, initializes tables and
// basic data.
fn open_database() -> Connection {
    let db = match sqlite::open(DB_FILE) {
        Ok(db) => db,
        Err(error) => {
            println!("opening database: {error}");
            std::process::exit(1);
        }
    };

    match db.execute(INIT_DB_QUERY) {
        Ok(_) => (),
        Err(error) => {
            println!("initializing database: {error}");
            std::process::exit(1);
        }
    }

    db
}

// Creates `days` days worth of fake data.
//
// TODO: Lots of hardcoded IDs here! Yikes! (But works for my test!)
fn create_fake_data(db: Connection, days: i64) {
    // // Locations.
    // const BEDROOM: i64 = 1;
    // const BATH_E: i64 = 2;
    // const BATH_S: i64 = 3;
    // const OFFICE_B: i64 = 4;
    // const OFFICE_S: i64 = 5;
    // const GREAT_ROOM: i64 = 6;
    // const LAUNDRY: i64 = 7;
    // const BALCONY: i64 = 8;
    // const OUTDOORS: i64 = 9;

    // Sensors.
    const AIR_TEMP: i64 = 1;
    const AIR_HUMIDITY: i64 = 2;
    const AIR_QUALITY: i64 = 3;
    const AIR_PRESSURE: i64 = 4;
    const CO1: i64 = 5;
    const SOIL_HUMIDITY: i64 = 6;
    const WATER_LEVEL: i64 = 7;

    const SAMPLES_PER_HOUR: i64 = 12;
    const MINS_PER_SAMPLE: TimeDelta = TimeDelta::minutes(60 / SAMPLES_PER_HOUR);

    let mut timestamp = Utc::now();
    let mut air_temp = 22.0;
    let mut air_humidity = 50.0;
    let mut air_quality = 75.0;
    let mut air_pressure = 1000.0;
    let mut co1 = 20.0;
    let mut soil_humidity = 60.0;
    let mut water_level = 80.0;

    for i in 0..days * 24 * SAMPLES_PER_HOUR {
        db.execute("BEGIN TRANSACTION;").unwrap();

        for loc in 1..=9 {
            let tmp_air_temp = air_temp + sinoise(loc as f64, 11.1) * 0.2;
            let tmp_air_humidity = air_humidity + sinoise(loc as f64, 22.2) * 0.5;
            let tmp_air_quality = air_quality + sinoise(loc as f64, 33.3) * 0.4;
            let tmp_air_pressure = air_pressure + sinoise(loc as f64, 44.4) * 9.0;
            let tmp_co1 = co1 + sinoise(loc as f64, 55.5) * 0.1;
            let tmp_soil_humidity = soil_humidity + sinoise(loc as f64, 66.6) * 0.6;
            let tmp_water_level = water_level + sinoise(loc as f64, 77.7) * 1.2;

            insert_data(&db, timestamp, loc, AIR_TEMP, tmp_air_temp).unwrap();
            insert_data(&db, timestamp, loc, AIR_HUMIDITY, tmp_air_humidity).unwrap();
            insert_data(&db, timestamp, loc, AIR_QUALITY, tmp_air_quality).unwrap();
            insert_data(&db, timestamp, loc, AIR_PRESSURE, tmp_air_pressure).unwrap();
            insert_data(&db, timestamp, loc, CO1, tmp_co1).unwrap();
            insert_data(&db, timestamp, loc, SOIL_HUMIDITY, tmp_soil_humidity).unwrap();
            insert_data(&db, timestamp, loc, WATER_LEVEL, tmp_water_level).unwrap();
        }

        // Update state
        timestamp = timestamp.checked_add_signed(MINS_PER_SAMPLE).unwrap();
        air_temp += sinoise(i as f64, 11.1) * 0.2;
        air_humidity += sinoise(i as f64, 22.2) * 0.5;
        air_quality += sinoise(i as f64, 33.3) * 0.4;
        air_pressure += sinoise(i as f64, 44.4) * 9.0;
        co1 += sinoise(i as f64, 55.5) * 0.1;
        soil_humidity += sinoise(i as f64, 66.6) * 0.6;
        water_level += sinoise(i as f64, 77.7) * 1.2;

        db.execute("COMMIT;").unwrap();
    }
}

fn insert_data(
    db: &Connection,
    timestamp: DateTime<Utc>,
    location: i64,
    sensor: i64,
    value: f64,
) -> Result<(), sqlite::Error> {
    let mut statement = db.prepare(INSERT_DATA_QUERY)?;

    let unix_timestamp = timestamp.timestamp();

    statement.bind::<&[(_, sqlite::Value)]>(
        &[
            (":timestamp", unix_timestamp.into()),
            (":location", location.into()),
            (":sensor", sensor.into()),
            (":value", value.into()),
        ][..],
    )?;

    match statement.next() {
        Err(error) => Err(error),
        Ok(state) => {
            if state == sqlite::State::Done {
                Ok(())
            } else {
                Err(sqlite::Error {
                    code: Some(171),
                    message: Some("Insert not properly executed.".to_string()),
                })
            }
        }
    }
}

// Poor man's random noise.
fn sinoise(x: f64, seed: f64) -> f64 {
    let xx = seed + x;
    return 0.5 * ((2.0 * xx).sin() + (std::f64::consts::PI * xx).sin());
}

const INSERT_DATA_QUERY: &str = "
    INSERT INTO data(timestamp, location, sensor, value)
        VALUES (:timestamp, :location, :sensor, :value);
";

// TODO: This is very hardcoded to my own needs.
const INIT_DB_QUERY: &str = "
    BEGIN TRANSACTION;

    PRAGMA foreign_keys = ON;

    CREATE TABLE IF NOT EXISTS locations (
        id   INTEGER PRIMARY KEY,
        name TEXT UNIQUE
    );

    CREATE TABLE IF NOT EXISTS sensors (
        id   INTEGER PRIMARY KEY,
        name TEXT UNIQUE
    );

    CREATE TABLE IF NOT EXISTS data (
        id        INTEGER PRIMARY KEY,
        timestamp INTEGER,
        location  INTEGER,
        sensor    INTEGER,
        value     REAL,
        FOREIGN KEY(location) REFERENCES locations(id),
        FOREIGN KEY(sensor)   REFERENCES sensors(id)
    );

    CREATE UNIQUE INDEX IF NOT EXISTS main_data_index ON data(timestamp, sensor, location);

    INSERT OR IGNORE INTO locations(name) VALUES
        ('bedroom'),
        ('bathroom-ensuite'),
        ('bathroom-social'),
        ('office-back'),
        ('office-side'),
        ('great-room'),
        ('laundry'),
        ('balcony'),
        ('outdoors')
    ;

    INSERT OR IGNORE INTO sensors(name) VALUES
        ('air-temperature'),
        ('air-humidity'),
        ('air-quality'),
        ('air-pressure'),
        ('carbon-monoxide'),
        ('soil-humidity'),
        ('water-level')
    ;

    COMMIT;
";
