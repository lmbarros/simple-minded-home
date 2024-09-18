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
	borrowedNetMain()

	setupNetworking()

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

func setupNetworking() {
	/***
	_, stack, _, err := SetupWithDHCP(SetupConfig{
		Hostname: "why-does-it-matter",
		// Logger:   logger,
		TCPPorts: 1, // For HTTP over TCP.
		UDPPorts: 1, // For DNS.
	})

	if err != nil {
		panic("setup DHCP:" + err.Error())
	}

	start := time.Now()
	svAddr, err := netip.ParseAddrPort(serverAddrStr)
	if err != nil {
		panic("parsing server address:" + err.Error())
	}
	***/
}
