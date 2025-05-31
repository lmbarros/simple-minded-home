//
// Environmental Data Server
//

mod queries;

use queries::*;

#[macro_use]
extern crate rocket;

use rocket::fairing::AdHoc;
use rocket::serde::{Deserialize, json::Json};
use rocket::{Build, Rocket, fairing};
use rocket_db_pools::sqlx::{self};
use rocket_db_pools::{Connection, Database};

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
        .mount("/api/v0", routes![put_location, put_sensor, put_data])
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

/// Input data needed to a data sample.
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
