package main

import (
	"fmt"
	"image/color"
	"log/slog"
	"machine"
	"os"
	"sync"
	"time"

	"tinygo.org/x/drivers/dht"
	"tinygo.org/x/drivers/ssd1306"
	"tinygo.org/x/tinyfont"
)

// pixelColor is the color we use when drawing things. We use a 1-bit display,
// so using an RGBA color here is merely a requirement from the interfaces used.
var pixelColor = color.RGBA{255, 255, 255, 255}

// temperature is the most recent temperature reading.
var temperature float32

// humidity is the most recent humidity reading.
var humidity float32

// muReadings is the mutex protecting `temperature` and `humidity`.
var muReadings sync.Mutex

// muGPIO is the mutex used tos serialize access to the GPIO pins on the Pi Pico
// W. I was getting some random timing I2C errors when running the program for a
// while, which I strongly believe were caused by the display and the DHT22
// sensor accessing the I2C interface simultaneously from different goroutines.
var muGPIO sync.Mutex

func main() {
	logger := createLogger()

	// It seems that it takes a while until the serial console is ready to be
	// written to. So we sleep for a while until we are sure that any subsequent
	// logging operations will actually go down the wire.
	time.Sleep(500 * time.Millisecond)

	logger.Info("The device is alive!")

	// TODO: Testing networking!
	// pn := NewPicoNet(logger)

	chClick, _ := initButton()
	dht22 := dht.New(machine.GPIO21, dht.DHT22)
	display, err := initDisplay()
	if err != nil {
		// Warn, but keep running without the display; hopefully we'll still be
		// able to send data via WiFi!
		logger.Warn("Initializing the display", slogError(err))
	}

	go sensorUpdateLoop(dht22, logger)

	chTicker := time.Tick(5 * time.Second)
	displayOn := true

	for {
		select {
		case d := <-chClick:
			if d.Seconds() > 4.0 {
				resetDevice(display, logger)
			}

			displayOn = !displayOn
			turnDisplayOnOff(display, displayOn)
		case <-chTicker:
			if displayOn {
				updateDisplay(display, logger)
			}
		}

		// TODO: Testing networking!
		// status := pn.Status()
		// logger.Info("PicoNet status", slog.String("status", status.String()))
		// time.Sleep(5 * time.Second)
		// deadline := time.Now().Add(5 * time.Second)
		// for time.Now().Before(deadline) {
		// 	// Nothing!
		// }

		// if button.Get() {
		// 	logger.Info("...")
		// } else {
		// 	logger.Info("!!!! BUTTON !!!!!")
		// }

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

func updateDisplay(d ssd1306.Device, logger *slog.Logger) {
	var t float32
	var h float32

	muReadings.Lock()
	t = temperature
	h = humidity
	muReadings.Unlock()

	textTemperature := fmt.Sprintf("🌡️%.1f°C", t)
	textHumidity := fmt.Sprintf("💧%.0f%%", h)

	muGPIO.Lock()
	defer muGPIO.Unlock()

	d.ClearBuffer()

	displayText(d, textTemperature, 0, 0)
	displayText(d, textHumidity, 0, 32)

	// This is an area of the screen that is generally empty, and therefore
	// usable for printing small error messages. If the air humidity gets to
	// 100%, there will be some overlap, so ideally debug messages should be
	// really short and aligned to the right hand side side of the screen.
	//
	// tinyfont.WriteLine(&display, &tinyfont.TomThumb, 84, 40, "Debug text!", pixelColor)
	// tinyfont.WriteLine(&display, &tinyfont.TomThumb, 84, 48, "More debug...", pixelColor)
	// tinyfont.WriteLine(&display, &tinyfont.TomThumb, 84, 56, "n' then some.", pixelColor)

	// TODO: Testing networking!
	// tinyfont.WriteLine(&display, &tinyfont.TomThumb, 84, 40, string(status), pixelColor)

	err := d.Display()
	if err != nil {
		logger.Warn("Updating the display", slogError(err))

		// Rebooting the device after a display issue may be a bit drastic, but
		// should be the most robust course of action I can take.
		//
		// The Sleep() is to make sure the log has enough time to be sent down
		// the serial port before the device resets. A much shorter sleep would
		// probably work, too.
		time.Sleep(5 * time.Second)
		machine.CPUReset()
	}
}

func turnDisplayOnOff(d ssd1306.Device, on bool) {
	d.Sleep(!on)
}

// sensorUpdateLoop is an infinite loop updating the sensor readings every so
// often. Meant to run in a separate goroutine.
func sensorUpdateLoop(d dht.Device, logger *slog.Logger) {
	chTick := time.Tick(5 * time.Second)

	for {
		muGPIO.Lock()
		t, err := d.TemperatureFloat(dht.C)
		if err != nil {
			logger.Warn("Reading temperature", slogError(err))
		}

		h, err := d.HumidityFloat()
		if err != nil {
			logger.Warn("Reading humidity", slogError(err))
		}
		muGPIO.Unlock()

		muReadings.Lock()
		temperature, humidity = t, h
		muReadings.Unlock()

		<-chTick
	}
}

func initButton() (chClick chan time.Duration, chDown chan time.Time) {
	button := machine.GPIO9
	button.Configure(machine.PinConfig{
		Mode: machine.PinInputPullup,
	})

	var timePressed time.Time = time.Now().Add(24 * time.Hour)
	chClick = make(chan time.Duration)
	chDown = make(chan time.Time)
	button.SetInterrupt(machine.PinFalling|machine.PinRising,
		func(p machine.Pin) {
			if !p.Get() {
				// Button pressed down.
				timePressed = time.Now()
				select {
				case chDown <- timePressed:
				default:
				}

			} else {
				// Button released.
				select {
				case chClick <- time.Now().Sub(timePressed):
				default:
				}
			}
		})

	return chClick, chDown
}

func resetDevice(display ssd1306.Device, logger *slog.Logger) {
	logger.Info("Reset requested")
	display.ClearBuffer()
	tinyfont.WriteLine(&display, &tinyfont.TomThumb, 40, 32, "Reseting...", pixelColor)
	display.Display()
	time.Sleep(3 * time.Second)
	logger.Info("Reseting now")
	machine.CPUReset()
}

func initDisplay() (ssd1306.Device, error) {
	displayI2C := machine.I2C1
	err := displayI2C.Configure(machine.I2CConfig{
		SCL:       machine.GPIO11,
		SDA:       machine.GPIO10,
		Frequency: 400 * machine.KHz,
	})
	if err != nil {
		return ssd1306.Device{}, fmt.Errorf("configuring I2C: %w", err)
	}

	display := ssd1306.NewI2C(displayI2C)
	display.Configure(ssd1306.Config{
		Width:    128,
		Height:   64,
		Address:  0x3C,
		VccState: ssd1306.SWITCHCAPVCC,
	})
	display.ClearDisplay()
	return display, nil
}
