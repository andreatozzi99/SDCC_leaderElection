// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import (
	"fmt"
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
	runInContainer       = false //getRunInContainer()
)

func getServerAddressAndPort() string {
	if value, exists := os.LookupEnv("SERVER_ADDRESS_AND_PORT"); exists {
		runInContainer = true
		fmt.Println("Run in container: ", runInContainer)
		return value
	}
	return "localhost:8080" // Valore di default
}

func getRunInContainer() bool {
	if _, exists := os.LookupEnv("RUN_IN_CONTAINER"); exists {
		fmt.Println("################################################################################")
		return true // se esiste la variabile d'ambiente allora il nodo è in un container
	}
	return false // Se non esiste la variabile d'ambiente allora il nodo è in locale
}
