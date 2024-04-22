// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

var (
	serverAddressAndPort = "localhost:8080" // Dipende da dove viene eseguito il nodo
	localAddress         = "localhost"
	electionAlg          = "Bully"
	emulateLocalCrash    = false
	crashProbability     = 10 // Valori da 0 a 99
	maxRttTime           = 5  // Espresso in secondi
)
