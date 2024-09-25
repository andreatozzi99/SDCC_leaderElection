// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import (
	"fmt"
	"os"
	"time"
)

const (
	// -------------------- Configurazione --------------------
	runInContainer    = true   // Se true, le componenti vengono eseguite ognuna in un container
	electionAlg       = "Raft" // Bully / Raft
	emulateLocalCrash = false
	crashProbability  = 10 // Valori da 0 a 99
	localAddress      = "localhost"
	containerAddress  = "node" // Nome del servizio docker
	logFilePath       = "/app/logfile.json"
	// -------------------- Raft parameter --------------------
	heartbeatTime    = time.Millisecond * 2000  // 2 secondi
	maxRttTime       = time.Millisecond * 1000  // 1 secondo
	electionTimerMin = time.Millisecond * 5000  // 5 secondi
	electionTimerMax = time.Millisecond * 15000 // 15 secondi
)

// Dipende da dove viene eseguito il nodo
var (
	serverAddressAndPort = getServerAddressAndPort()
)

func getServerAddressAndPort() string {
	if value, exists := os.LookupEnv("SERVER_ADDRESS_AND_PORT"); exists {
		fmt.Println("################################################################################")
		return value
	}
	return "localhost:8080" // Valore di default
}
