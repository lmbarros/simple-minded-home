# Simple Minded Home

Not smart, but nor really dumb either.

Collection of projects I am using to add some slightly-dim brains to my home.

## Hardware

I'm using a 0.01µF (labeled "103") ceramic capacitor in parallel with my switch
to debounce it, and it seems to work well-enough. A 0.1µF one (labeled "104")
also seems to do fine, for that matter.

## Notes to self

### TODO

* Temperature & Humidity sensor
    * Check if I can get to some low-power state in between updates.
    * Send data somewhere.
        * Alright, at least got the code to make HTTP requests in place!

### Reference

* `sudo ~/bin/tinygo/bin/tinygo flash -target=pico -stack-size=8kb -monitor ./examples/blinky`

* [Tinygo drivers](https://github.com/tinygo-org/drivers/tree/release)
* [Preliminary Pico W support](https://github.com/soypat/cyw43439)
* [Tiny FS](https://github.com/tinygo-org/tinyfs) might come handy.
* [TinyGo Pi Pico reference](https://tinygo.org/docs/reference/microcontrollers/pico/)
