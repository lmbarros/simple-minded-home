# Simple Minded Home

Not smart, but nor really dumb either.

Collection of projects I am using to add some slightly-dim brains to my home.

## Notes to self

### TODO

* Temperature & Humidity sensor
    * Button to turn the display off and on.
        * Probably will [want to use
          interrupts](https://pragmatik.tech/gpio-and-interrupts-on-the-pico).
    * Consider updating only every minute, or maybe 5 minutes.
    * Check if I can get to some low-power state in between updates.
    * Send data somewhere.

### Reference

* `sudo ~/bin/tinygo/bin/tinygo flash -target=pico -stack-size=8kb -monitor ./examples/blinky`

* [Tinygo drivers](https://github.com/tinygo-org/drivers/tree/release)
* [Preliminary Pico W support](https://github.com/soypat/cyw43439)
* [Tiny FS](https://github.com/tinygo-org/tinyfs) might come handy.
* [TinyGo Pi Pico reference](https://tinygo.org/docs/reference/microcontrollers/pico/)
