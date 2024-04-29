// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import "time"

const (
	serverAddressAndPort = "localhost:8080" // Dipende da dove viene eseguito il nodo
	localAddress         = "localhost"
	electionAlg          = "Raft"
	emulateLocalCrash    = false
	crashProbability     = 10 // Valori da 0 a 99
	maxRttTime           = 5  // Espresso in secondi
	electionTimerMin     = time.Millisecond * 5000
	electionTimerMax     = time.Millisecond * 15000
)
