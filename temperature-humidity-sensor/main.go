package main

import (
	"fmt"
	"image/color"
	"machine"
	"os"
	"time"

	"tinygo.org/x/drivers/dht"
	"tinygo.org/x/drivers/ssd1306"
)

// pixelColor is the color we use when drawing things. We use a 1-bit display,
// so using an RGBA color here is merely a requirement from the interfaces used.
var pixelColor = color.RGBA{255, 255, 255, 255}

func main() {
	logger := createLogger()

	// It seems that it takes a while until the serial console is ready to be
	// written to. So we sleep for a while until we are sure that any subsequent
	// logging operations will actually go down the wire.
	time.Sleep(500 * time.Millisecond)

	logger.Info("The device is alive!")

	// TODO: Testing networking!
	// pn := NewPicoNet(logger)

	dht11 := dht.New(machine.GPIO15, dht.DHT11)

	displayI2C := machine.I2C0
	err := displayI2C.Configure(machine.I2CConfig{
		SCL:       machine.GPIO1,
		SDA:       machine.GPIO0,
		Frequency: 400 * machine.KHz,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuring I2C:", err)
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
		// TODO: Testing networking!
		// status := pn.Status()
		// logger.Info("PicoNet status", slog.String("status", status.String()))
		time.Sleep(5 * time.Second)

		temperature, err := dht11.TemperatureFloat(dht.C)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Reading temperature: %v\n", err)
		}

		humidity, err := dht11.HumidityFloat()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Reading humidity: %v\n", err)
		}

		textTemperature := fmt.Sprintf("🌡️%.1f°C", temperature)
		textHumidity := fmt.Sprintf("💧%.0f%%", humidity)

		display.ClearBuffer()
		displayText(display, textTemperature, 0, 0)
		displayText(display, textHumidity, 0, 32)

		// This is an area of the screen that is generally empty, and therefore
		// usable for printing small error messages. If the air humidity gets to
		// 100%, there will be some overlap, so ideally debug messages should be
		// really short and aligned to the right hand side side of the screen.
		// (By teh way, the DHT11 will never get to 100% humidity, but we may
		// get there when upgrading to a DHT22.)
		//
		// tinyfont.WriteLine(&display, &tinyfont.TomThumb, 84, 40, "Debug text!", pixelColor)
		// tinyfont.WriteLine(&display, &tinyfont.TomThumb, 84, 48, "More debug...", pixelColor)
		// tinyfont.WriteLine(&display, &tinyfont.TomThumb, 84, 56, "n' then some.", pixelColor)

		// TODO: Testing networking!
		// tinyfont.WriteLine(&display, &tinyfont.TomThumb, 84, 40, string(status), pixelColor)

		// TODO: Testing networking!
		// if status == StatusReadyToGo {
		// 	res, err := pn.Get("http://stackedboxes.org/2021/12/30/from-bare-docs-to-bare-metal/")
		// 	if err != nil {
		// 		logger.Warn("GET request failed", slogError(err))
		// 		continue
		// 	}

		// 	logger.Info("GET RESPONSE!",
		// 		slog.String("proto", res.Proto),
		// 		slog.String("status", res.Status),
		// 		slog.Int("contentLength", res.ContentLength),
		// 		slog.Int("statusCode", res.StatusCode),
		// 	)

		// 	print(string(res.Body))

		// 	logger.Info("THAT'S IT!")
		// }

		display.Display()
	}
}

func displayText(display ssd1306.Device, text string, x, y int16) {
	for _, r := range text {
		if img, ok := runeToImage[r]; ok {
			err := display.DrawBitmap(x, y, img)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Displaying a character: %v", err)
			}
			w, _ := img.Size()
			x += int16(w)
		}
	}
}
