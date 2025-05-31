//
// Environmental Data Server
//

pub const INSERT_LOCATION_SQL: &str = "INSERT INTO locations(name) VALUES (?);";
pub const INSERT_SENSOR_SQL: &str = "INSERT INTO sensors(name) VALUES (?);";
pub const INSERT_DATA_SQL: &str =
    "INSERT INTO data(timestamp, location, sensor, value) VALUES (?, ?, ?, ?);";

pub const ID_FROM_LOCATION_SQL: &str = "SELECT id FROM locations WHERE name = ?;";
pub const ID_FROM_SENSOR_SQL: &str = "SELECT id FROM sensors WHERE name = ?;";

// By using a different timestamp format, we can change the grouping.
// Technically this will always include a "GROUP BY" clause, but using a full,
// to-the-second timestamp will have single-line groups, which should be the
// same as no grouping.
pub fn select_data_sql(timestamp_format: &str) -> String {
    "
        SELECT
            strftime('{TIMESTAMP_FORMAT}', timestamp, 'unixepoch') AS ts,
            AVG(value) AS value
        FROM
            data
        WHERE
            timestamp BETWEEN ? AND ? AND sensor = ? AND location = ?
        GROUP BY
            ts
        ORDER BY
            ts;
    "
    .replace("{TIMESTAMP_FORMAT}", timestamp_format)
}
