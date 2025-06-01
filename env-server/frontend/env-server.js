
async function test_api() {
  try {
    const response = await fetch('/api/v0/get_data', {
    method: "POST",
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      "unix_timestamp_from": 0,
      "unix_timestamp_to": 1848639261,
      "location": "bathroom-social",
      "sensor": "temperature"
    })});

    if (!response.ok) {
      throw new Error(`HTTP error! Status: ${response.status}`);
    }

    const data = await response.json();
    console.log(data);
  } catch (error) {
    console.error('Fetch error:', error);
  }
}
