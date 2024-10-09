// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import (
	"fmt"
	"os"
	"time"
)

const (
	// -------------------- Configurazione --------------------
	//electionAlg         = "Raft" // Bully / Raft
	emulateLocalCrash   = false
	crashProbability    = 10 // Range di numeri per simulare crash di un nodo, un numero random generato ogni 5 secondi.
	recoveryProbability = 5  // Range numeri per simulare la riattivazione del nodo
	// --------------------------------------------------------
	localAddress     = "localhost"
	containerAddress = "node" // Nome del servizio docker
	logFilePath      = "/app/logfile.json"
	maxRttTime       = time.Millisecond * 1000 // 1 secondo
	// -------------------- Raft parameter --------------------
	heartbeatTime    = time.Millisecond * 2000  // 2 secondi
	electionTimerMin = time.Millisecond * 5000  // 5 secondi
	electionTimerMax = time.Millisecond * 15000 // 15 secondi
)

// Dipende da dove viene eseguito il nodo
var (
	electionAlg          = "Bully" // Bully / Raft
	runInContainer       = false
	serverAddressAndPort = getEnvVariabiles()
)

func getEnvVariabiles() string {
	if value, exists := os.LookupEnv("SERVER_ADDRESS_AND_PORT"); exists {
		runInContainer = true
		fmt.Println("Run in container: ", runInContainer)
		return value
	}
	return "localhost:8080" // Valore di default
}
