//
// Environmental Data Server
//

mod queries;

use queries::*;

#[macro_use]
extern crate rocket;

use rocket::fairing::AdHoc;
use rocket::http::Status;
use rocket::serde::{Deserialize, json::Json};
use rocket::{Build, Rocket, fairing};
use rocket_db_pools::sqlx::{self};
use rocket_db_pools::{Connection, Database};
use serde::Serialize;
use sqlx::prelude::FromRow;

//
// Rocket initialization
//

#[launch]
fn rocket() -> _ {
    let migrations_fairing = AdHoc::try_on_ignite("SQLx Migrations", run_migrations);
    rocket::build()
        .attach(EnvServerDb::init())
        .attach(migrations_fairing)
        .mount("/", routes![index])
        .mount(
            "/api/v0",
            routes![get_data, put_location, put_sensor, put_data],
        )
}

//
// Database
//

/// The database itself. This is attached to rocket and injected into handlers.
#[derive(Database)]
#[database("env_server_db")]
struct EnvServerDb(sqlx::SqlitePool);

/// Run the migrations. This is attached to rocket and run during startup
/// (AKA "ignition").
async fn run_migrations(rocket: Rocket<Build>) -> fairing::Result {
    match EnvServerDb::fetch(&rocket) {
        Some(db) => match sqlx::migrate!("./src/db/migrations").run(&**db).await {
            Ok(_) => Ok(rocket),
            Err(e) => {
                error!("Failed to run database migrations: {}", e);
                Err(rocket)
            }
        },
        None => Err(rocket),
    }
}

//
// API Handlers
//

/// Input data passed to queries dealing with locations.
#[derive(Deserialize)]
#[serde(crate = "rocket::serde")]
struct LocationInput {
    location: String,
}

/// Creates a new location in the database.
#[put("/location", data = "<input>")]
async fn put_location(
    mut db: Connection<EnvServerDb>,
    input: Json<LocationInput>,
) -> Option<String> {
    let success = sqlx::query(INSERT_LOCATION_SQL)
        .bind(&input.location)
        .execute(&mut **db)
        .await
        .is_ok();

    if success {
        Some("Ok".to_string())
    } else {
        None
    }
}

/// Input data passed to queries dealing with sensors.
#[derive(Deserialize)]
#[serde(crate = "rocket::serde")]
struct SensorInput {
    sensor: String,
}

/// Creates a new sensor entry in the database.
#[put("/sensor", data = "<input>")]
async fn put_sensor(mut db: Connection<EnvServerDb>, input: Json<SensorInput>) -> Option<String> {
    let success = sqlx::query(INSERT_SENSOR_SQL)
        .bind(&input.sensor)
        .execute(&mut **db)
        .await
        .is_ok();

    if success {
        Some("Ok".to_string())
    } else {
        None
    }
}

/// Input data needed to create a data sample.
#[derive(Deserialize)]
#[serde(crate = "rocket::serde")]
struct CreateDataInput {
    unix_timestamp: i64,
    location: String,
    sensor: String,
    value: f32,
}

/// Creates a new data sample entry in the database.
#[put("/data", data = "<input>")]
async fn put_data(mut db: Connection<EnvServerDb>, input: Json<CreateDataInput>) -> Option<String> {
    let location_id = id_from_location(&mut db, &input.location).await?;
    let sensor_id = id_from_sensor(&mut db, &input.sensor).await?;

    let success = sqlx::query(INSERT_DATA_SQL)
        .bind(input.unix_timestamp)
        .bind(location_id)
        .bind(sensor_id)
        .bind(input.value)
        .execute(&mut **db)
        .await
        .is_ok();

    if success {
        Some("Ok".to_string())
    } else {
        None
    }
}

/// Input data needed to query data.
#[derive(Deserialize)]
#[serde(crate = "rocket::serde")]
struct QueryDataInput {
    unix_timestamp_from: i64,
    unix_timestamp_to: i64,
    location: String,
    sensor: String,
}

#[derive(Serialize, FromRow)]
#[serde(crate = "rocket::serde")]
struct Measurement {
    ts: String,
    value: f64,
}

/// Gets data samples for a given sensor and location combination, for a given
/// time interval.
///
/// Depending on the interval select, the returned data samples will be the
/// average of some interval (like an hour, or a day, or even a year if the
/// interval is really long).
#[post("/get_data", data = "<input>")]
async fn get_data(
    mut db: Connection<EnvServerDb>,
    input: Json<QueryDataInput>,
) -> Result<Json<Vec<Measurement>>, Status> {
    const DAY: i64 = 24 * 60 * 60;
    const YEAR: i64 = 365 * DAY;

    // Let's say that we want to return at most around 1000 samples to any given
    // request. And let's assume our sensors send a sample every 5 minutes, or
    // 12 samples per hour (love me some hardcoded assumptions 🤪!). So, we can
    // serve about 3.5 days worth of "raw", unaggregated data.
    const UNGROUPED_LIMIT: i64 = (DAY as f64 * 3.5) as i64;

    // Aggregating by hour, we can serve about 40 days of data an be within our
    // self-imposed limit of ~1000 data samples.
    const GROUP_BY_HOUR_LIMIT: i64 = DAY * 40;

    // Aggregating by day, we can serve some 2.7 years of data.
    const GROUP_BY_DAY_LIMIT: i64 = (YEAR as f64 * 2.7) as i64;

    // Aggregating by month, we can serve 1000 months, or 83 years. (I guess
    // I'll never need to go beyond that, but we'd group by year after this.)
    const GROUP_BY_MONTH_LIMIT: i64 = YEAR * 83;

    let location_id = id_from_location(&mut db, &input.location)
        .await
        .ok_or(Status::BadRequest)?;
    let sensor_id = id_from_sensor(&mut db, &input.sensor)
        .await
        .ok_or(Status::BadRequest)?;

    let interval_secs = input.unix_timestamp_to - input.unix_timestamp_from;
    let sql = match interval_secs {
        ..1 => return Err(Status::BadRequest),
        1..UNGROUPED_LIMIT => select_data_sql("%Y-%m-%dT%H:%M:%SZ"),
        UNGROUPED_LIMIT..GROUP_BY_HOUR_LIMIT => select_data_sql("%Y-%m-%dT%H:00:00Z"),
        GROUP_BY_HOUR_LIMIT..GROUP_BY_DAY_LIMIT => select_data_sql("%Y-%m-%dT00:00:00Z"),
        GROUP_BY_DAY_LIMIT..GROUP_BY_MONTH_LIMIT => select_data_sql("%Y-%m-01T00:00:00Z"),
        _ => select_data_sql("%Y-01-01T00:00:00Z"),
    };

    let result = sqlx::query_as::<_, Measurement>(&sql)
        .bind(input.unix_timestamp_from)
        .bind(input.unix_timestamp_to)
        .bind(sensor_id)
        .bind(location_id)
        .fetch_all(&mut **db)
        .await;

    match result {
        Ok(measurements) => Ok(Json(measurements)),
        Err(_) => Err(Status::InternalServerError),
    }
}

//
// Helpers
//

async fn id_from_location(db: &mut Connection<EnvServerDb>, location: &str) -> Option<i64> {
    let row: (i64,) = sqlx::query_as(ID_FROM_LOCATION_SQL)
        .bind(location)
        .fetch_one(db.as_mut())
        .await
        .ok()?;

    return Some(row.0);
}

async fn id_from_sensor(db: &mut Connection<EnvServerDb>, sensor: &str) -> Option<i64> {
    let row: (i64,) = sqlx::query_as(ID_FROM_SENSOR_SQL)
        .bind(sensor)
        .fetch_one(db.as_mut())
        .await
        .ok()?;

    return Some(row.0);
}

//
// xxxxxxxxxxxxxxxxxxxxxxxxxx test and temporary stuff xxxxxxxxxxxxxxxxxxxxxxxxxxxxx
//

#[get("/")]
fn index() -> &'static str {
    "Hello, sensory-time-seriesy world!"
}

// #[get("/test")]
// async fn test(mut db: Connection<EnvServerDb>) -> Option<String> {
//     let result: (i32,) = sqlx::query_as("SELECT 1").fetch_one(&mut **db).await.ok()?;
//     Some(format!("Result: {}", result.0))
// }
