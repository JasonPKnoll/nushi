package main

import "os"

// signalName is the signal that stopped the daemon as Go names it: on
// Windows only an interrupt (Ctrl+C, Ctrl+Break) or a console's close
// comes as one.
func signalName(sig os.Signal) string { return sig.String() }
