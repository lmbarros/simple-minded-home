# Environmental Data Server (SMH)

## SQLite as a time-series database?

Quick test: create synthetic data for a scenario like this:

* 63 sensors
* generating samples every 5 minutes
* for 50 years
* indexing by timestamp/sensor/location

This results in a 16GB file with 331,128,000 rows. This is much more data than I
realistically expect to ever have.

Let's try a time-series-like query on this dataset:

```sql
SELECT
    strftime('%Y-%m-%d', timestamp, 'unixepoch') AS day,
    AVG(value) AS average_value
FROM
    data
WHERE
	timestamp BETWEEN unixepoch('2025-05-30') AND unixepoch('2025-08-21') AND sensor = 1 AND location = 6
GROUP BY
     day
ORDER BY
     day;
```

On my aging desktop (Core i7-8700) it takes around 150ms to run. On a Raspberry
Pi 3 (a single-board computer released 9 years ago in 2016), it runs in 3.3
seconds--too slow for comfortable interactive usage, but not *that* bad all
things considered!

My conclusions from that? First: yes, SQLite can work as a poor man's
time-series DB for simple use cases like mine. And second: this is true even
when running on old, slow hardware (again, for my simplistic use case). So, I
guess I'll be running this on a Pi 3 for the foreseeable future, and with the
amount of data I expect to have in the next couple of years, I shall be able to
query the data at comfortably interactive speeds.
