//
// Environmental Data Server
//

pub const INSERT_LOCATION_SQL: &str = "INSERT INTO locations(name) VALUES (?);";
pub const INSERT_SENSOR_SQL: &str = "INSERT INTO sensors(name) VALUES (?);";
pub const INSERT_DATA_SQL: &str =
    "INSERT INTO data(timestamp, location, sensor, value) VALUES (?, ?, ?, ?);";

pub const ID_FROM_LOCATION_SQL: &str = "SELECT id FROM locations WHERE name = ?;";
pub const ID_FROM_SENSOR_SQL: &str = "SELECT id FROM sensors WHERE name = ?;";
