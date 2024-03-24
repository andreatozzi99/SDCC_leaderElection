package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"
	"time"
)

// Variabile che rappresentano lo stato (variabile) di un nodo
var nodeList []Node // Lista dei nodi di cui un nodo è a conoscenza
var leaderID = -1   // ID dell'attuale leader

// Node Struttura per rappresentare un nodo
type Node struct {
	ID        int
	IPAddress string
	Port      int
}

// HeartBeat Metodo per contattare il nodo Leader e verificare se esse è ancora attivo
func (n *Node) HeartBeat(senderID int, reply *bool) error {
	*reply = true // Se la funzione viene avviata, vuol dire che il nodo è funzionante
	fmt.Printf("LEADER %d :Msg di HeartBeat da parte di -->%d\n", n.ID, senderID)
	return nil
}

func main() {
	// Avvia un singolo nodo
	node := &Node{
		ID:        0, // cosi il serverRegistry assegna il giusto ID
		IPAddress: "localhost",
	}
	node.start()

	// Mantieni il programma in esecuzione
	select {}
}

// Metodo per avviare il nodo e registrarne l'indirizzo nel server di registrazione
// e prendere lista dei nodi in rete
func (n *Node) start() {
	// Genera una porta casuale disponibile per il nodo
	n.Port = findAvailablePort()

	// Connette il nodo al server di registrazione
	client, err := rpc.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Errore nella connessione al server di registrazione:", err)
		return
	}
	// Chiusura connessione a fine di questa funzione
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	// Chiamata RPC per registrare il nodo nel server di registrazione
	var reply bool
	// ############ Nome servizio su file CONF ###################
	err = client.Call("ServerRegistry.RegisterNode", n, &reply)
	if err != nil {
		fmt.Println("Errore durante la registrazione del nodo:", err)
		return
	}
	if !reply {
		fmt.Println("Il server di registrazione ha rifiutato la registrazione del nodo")
		return
	}

	// Visualizza l'ID e la porta assegnati al nodo
	fmt.Printf("Regisrazione sulla rete avvenuta ID %d: Assigned port %d\n", n.ID, n.Port)

	// Chiamata RPC per richiedere la lista dei nodi al server di registrazione
	// ############ Nome servizio su file CONF ###################
	err = client.Call("ServerRegistry.GetNodeList", 0, &nodeList)
	if err != nil {
		fmt.Println("Errore durante la richiesta della lista dei nodi:", err)
		return
	}

	printNodeList()

	// Avvia la goroutine per heartbeat
	go n.heartBeatRoutine()
}

// Meccanismo di Failure Detection:
// contattare il nodo leader periodicamente (Invocazione del metodo remoto: HeartBeat)
func (n *Node) heartBeatRoutine() {
	for {
		// Contatta il nodo leader
		if !n.isLeaderAlive() {
			go n.startElection()
		}

		// Attendi un intervallo casuale prima di effettuare un nuovo heartbeat
		// ######### Selezione intervallo su file di configurazione ##########
		randomSeconds := rand.Intn(5-1) + 1
		interval := time.Duration(randomSeconds) * time.Second
		time.Sleep(interval)
	}
}

// Metodo per contattare il nodo leader
func (n *Node) isLeaderAlive() bool {

	// Se il nodo stesso è il leader, esci
	if n.ID == leaderID {
		return true
	}

	// Una nuova connessione con il nodo leader per ogni ciclo di heartbeat?
	//########## INDIRIZZO SU FILE CONFIGURAZIONE #################
	client, err := rpc.Dial("tcp", fmt.Sprintf("localhost:%d", leaderID))
	if err != nil {
		// Se non è possibile contattare il leader, avvia un processo di elezione
		fmt.Println("Node", n.ID, "failed to contact leader Node", leaderID)
		return false
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	// Chiamata RPC per il heartbeat al nodo leader
	var reply bool
	err = client.Call("Node.HeartBeat", n.ID, &reply)
	if err != nil {
		// Se il leader non risponde, avvia un processo di elezione
		fmt.Println("Node", n.ID, "failed to contact leader Node", leaderID)
		return false
	}
	//chiamata andata a buon fine
	return true
}

// Metodo per avviare un processo di elezione seguendo algoritmo BULLY
func (n *Node) startElection() {
	// Devo chiamare la procedura remota esposta da ogni nodo nella rete
	// Nome della procedura: ELECTION (come messaggio dell'algoritmo bully)

	// Dopo aver completato l'elezione, notifica gli altri nodi
	// n.NotifyLeader()
}

// Funzione per trovare una porta disponibile casuale
func findAvailablePort() int {
	// Cerca una porta disponibile partendo da una porta casuale
	startPort := 30000
	for port := startPort; port < 65535; port++ {
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			err := ln.Close()
			if err != nil {
				return 0
			}
			return port
		}
	}
	return 0 // Ritorna 0 se non viene trovata alcuna porta disponibile
}

func printNodeList() {
	fmt.Print("I nodi sulla rete sono:")
	for _, node := range nodeList {
		fmt.Printf("\nID: %d, IP: %s, Port: %d", node.ID, node.IPAddress, node.Port)
	}
}
