// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import (
	"os"
	"time"
)

const (
	// -------------------- Configurazione --------------------
	electionAlg       = "Bully" // Bully / Raft
	emulateLocalCrash = false
	crashProbability  = 10 // Valori da 0 a 99
	localAddress      = "localhost"
	containerAddress  = "node" // Nome del servizio docker
	// -------------------- Raft parameter --------------------
	maxRttTime       = 5                        // Espresso in secondi
	electionTimerMin = time.Millisecond * 5000  // 5 secondi
	electionTimerMax = time.Millisecond * 15000 // 15 secondi
)

// Dipende da dove viene eseguito il nodo
var (
	serverAddressAndPort = getServerAddressAndPort()
	runInContainer       = getRunInContainer()
)

func getServerAddressAndPort() string {
	if value, exists := os.LookupEnv("SERVER_ADDRESS_AND_PORT"); exists {
		return value
	}
	return "localhost:8080" // Valore di default
}

func getRunInContainer() bool {
	if _, exists := os.LookupEnv("RUN_IN_CONTAINER"); exists {
		return true
	}
	return false // Valore di default
}
