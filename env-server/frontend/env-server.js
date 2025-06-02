//
// Environmental Data Server: The Frontend
//

const colors = ['#e33b3b', '#e3a13b', '#e3d53b', '#3be33f', '#3bc7e3', '#3b49e3', '#a43be3'];
let theChart = {};

async function addPlot() {
  function unixTimestampFromDatetimeLocal(elemId) {
    const elem = document.getElementById(elemId);
    const date = new Date(elem.value);
    return (date.getTime() / 1000);
  }

  const location = document.getElementById('locations').value;
  const sensor = document.getElementById('sensors').value;
  const fromTimestamp = unixTimestampFromDatetimeLocal('timeFrom');
  const toTimestamp = unixTimestampFromDatetimeLocal('timeTo');

  const data = await getData(location, sensor, fromTimestamp, toTimestamp);

  const nextIndex = theChart.data.datasets.length;
  theChart.data.datasets.push({
    label: `${location} / ${sensor}`,
    data: data.map((e) => { return { x: e.ts, y: e.value } }),
    borderColor: colors[nextIndex % colors.length],
    backgroundColor: colors[nextIndex % colors.length]
  });

  theChart.update();
}

function clearPlot() {
  theChart.data.datasets = [];
  theChart.update();
}

async function populateUI() {
  // Locations.
  const locationsElem = document.getElementById('locations');
  const locations = await getLocations();
  for (const location of locations) {
    const option = document.createElement('option');
    option.setAttribute('value', location);
    option.appendChild(document.createTextNode(location));
    locationsElem.appendChild(option);
  }

  // Sensors.
  const sensorsElem = document.getElementById('sensors');
  const sensors = await getSensors();
  for (const sensor of sensors) {
    const option = document.createElement('option');
    option.setAttribute('value', sensor);
    option.appendChild(document.createTextNode(sensor));
    sensorsElem.appendChild(option);
  }

  // Chart.
  const ctx = document.getElementById('the-chart');
  theChart = new Chart(ctx, {
    type: 'line',
    options: {
      scales: {
        x: {
          type: 'timeseries',
          title: {
            display: true,
            text: 'Time'
          }
        }
      }
    },
    datasets: [ ]
  });
}

async function getLocations() {
  try {
    const response = await fetch('/api/v0/locations');
    if (!response.ok) {
      throw new Error(`HTTP error fetching locations! Status: ${response.status}`);
    }
    const data = await response.json();
    return data;
  } catch (error) {
    console.error('Error fetching locations:', error);
  }
}

async function getSensors() {
  try {
    const response = await fetch('/api/v0/sensors');
    if (!response.ok) {
      throw new Error(`HTTP error fetching sensors! Status: ${response.status}`);
    }
    const data = await response.json();
    return data;
  } catch (error) {
    console.error('Error fetching locations:', error);
  }
}

async function getData(location, sensor, fromTimestamp, toTimestamp) {
  try {
    const response = await fetch('/api/v0/get_data', {
    method: "POST",
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      "unix_timestamp_from": fromTimestamp,
      "unix_timestamp_to": toTimestamp,
      "location": location,
      "sensor": sensor
    })});

    if (!response.ok) {
      throw new Error(`HTTP error! Status: ${response.status}`);
    }

    return response.json();
  } catch (error) {
    console.error('Fetch error:', error);
  }
}
