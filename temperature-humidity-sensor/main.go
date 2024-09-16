package main

import (
	"fmt"
	"machine"
	"time"

	"tinygo.org/x/drivers/dht"
)

func main() {
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})

	dht11 := dht.New(machine.GPIO15, dht.DHT11)

	for {
		// Blink
		led.High()
		time.Sleep(time.Second / 3)

		led.Low()
		time.Sleep(time.Second * 2)

		// Read sensor
		temperature, err := dht11.TemperatureFloat(dht.C)
		if err == nil {
			fmt.Printf("Error reading temperature: %v", err)
		}

		humidity, err := dht11.HumidityFloat()
		if err != nil {
			fmt.Printf("Error reading humidity: %v", err)
		}

		fmt.Printf("Temperature: %.1f°C\nHumidity: %.1f%%\n\n", temperature, humidity)
	}
}
