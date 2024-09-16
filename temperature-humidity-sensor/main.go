package main

import (
	"fmt"
	"image/color"
	"machine"
	"math/rand"
	"time"

	"tinygo.org/x/drivers/dht"
	"tinygo.org/x/drivers/ssd1306"
)

func main() {
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})

	dht11 := dht.New(machine.GPIO15, dht.DHT11)

	displayI2C := machine.I2C0
	err := displayI2C.Configure(machine.I2CConfig{
		SCL:       machine.GPIO1,
		SDA:       machine.GPIO0,
		Frequency: 400 * machine.KHz,
	})
	if err != nil {
		println("could not configure I2C:", err)
		return
	}

	display := ssd1306.NewI2C(displayI2C)
	display.Configure(ssd1306.Config{
		Width:    128,
		Height:   64,
		Address:  0x3C,
		VccState: ssd1306.SWITCHCAPVCC,
	})
	display.ClearDisplay()

	for {
		// Blink LED
		led.High()
		time.Sleep(time.Second / 3)

		led.Low()
		time.Sleep(time.Second * 2)

		// Read sensor
		temperature, err := dht11.TemperatureFloat(dht.C)
		if err != nil {
			fmt.Printf("Error reading temperature: %v", err)
		}

		humidity, err := dht11.HumidityFloat()
		if err != nil {
			fmt.Printf("Error reading humidity: %v", err)
		}

		fmt.Printf("Temperature: %.1f°C\nHumidity: %.1f%%\n\n", temperature, humidity)

		display.ClearBuffer()
		for x := int16(0); x < 128; x++ {
			for y := int16(0); y < 64; y++ {
				p := (float32(x) / 128) * (float32(y) / 64)
				if p < rand.Float32() {
					display.SetPixel(x, y, color.RGBA{255, 255, 255, 255})
				}
			}
		}
		display.Display()
	}
}
