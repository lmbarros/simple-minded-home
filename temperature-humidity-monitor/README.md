# Temperature and Humidity Sensor

Li'l device measuring the temperature and humidity.

## Hardware

I'm using a 0.01µF (labeled "103") ceramic capacitor in parallel with my switch
to debounce it, and it seems to work well-enough. A 0.1µF one (labeled "104")
also seems to do fine, for that matter.

## Case

[Design in OnShape](https://cad.onshape.com/documents/e987645894743680e4f71a9c/w/7ab77c4f7e5b5df48522bfbd/e/d8782f551b3195f70bd8c6d7).

![The case still in the 3D printer](./case/01-out-of-the-printer.jpg "The case still in the 3D printer.")

![Starting the assembly](./case/02-assembling-wire-wrap.jpg "Starting the assembly. Using wire wrapping instead of soldering, with my very OK wire wrapping tool. The spaces are very tight, and I have melted some bits of the case while placing the threaded inserts for the Pi Pico. Now, thinking of it, I guess I could have fixed the display directly on the lid to simplify the design and ease the assembly. Not sure it would fit there, though.")

![Assembly almost complete.](./case/03-assembling-almost-there.jpg "Assembly almost complete. The button fits tightly in place and doesn't need glue to stay there. Note de tiny ceramic capacitor for debouncing(this one is soldered, by the way).")

![The final thing, assembled and working](./case/04-working.jpg "The final thing, assembled and working.")

Some notable slicing settings I used:

* Infill Density: 25%
* Extra Infill Wall Count: 3 [for thicker walls around the insertion screws]
* In the first case I printed, I force-added supports under the "pillars" that
  hold the display. For the second print I didn't, and it still worked very
  well. (Those are on a 45° angle, which everyone says is fine -- now I
  believe everyone!)
