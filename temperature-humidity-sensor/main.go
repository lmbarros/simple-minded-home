package main

import (
	"fmt"
	"image/color"
	"machine"
	"time"

	"tinygo.org/x/drivers/dht"
	"tinygo.org/x/drivers/ssd1306"
	"tinygo.org/x/tinyfont"
)

// pixelColor is the color we use when drawing things. We use a 1-bit display,
// so using an RGBA color here is merely a requirement from the interfaces used.
var pixelColor = color.RGBA{255, 255, 255, 255}

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
		textHumidity := fmt.Sprintf("💧%.0f%%", humidity)
		textTemperature := fmt.Sprintf("🌡️%.1f°C", temperature)
		printToDisplay(display, textHumidity, 4, 4)
		printToDisplay(display, textTemperature, 4, 34)

		// Dummy debugging
		tinyfont.WriteLine(&display, &tinyfont.TomThumb, 0, 63, "This is a fake debug text!", pixelColor)

		display.Display()
	}
}

func printToDisplay(display ssd1306.Device, text string, x, y int16) {
	for _, r := range text {
		if img, ok := runeToImage[r]; ok {
			display.DrawBitmap(x, y, img)
			w, _ := img.Size()
			x += int16(w)
		}
	}
}
