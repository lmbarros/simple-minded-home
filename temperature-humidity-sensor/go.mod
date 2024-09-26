module main

go 1.23.0

require (
	github.com/soypat/cyw43439 v0.0.0-20240627234239-a62ee4027d66
	github.com/soypat/seqs v0.0.0-20240527012110-1201bab640ef
	tinygo.org/x/drivers v0.28.0
	tinygo.org/x/tinyfont v0.3.0
)

require (
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/tinygo-org/pio v0.0.0-20231216154340-cd888eb58899 // indirect
	golang.org/x/exp v0.0.0-20230728194245-b0cb94b80691 // indirect
)

// Use my own seqs fork, which include enough hacks to allow me to parse an HTTP
// response.
replace github.com/soypat/seqs v0.0.0-20240527012110-1201bab640ef => github.com/lmbarros/seqs v0.0.0-20240926163717-fa870a09d078
