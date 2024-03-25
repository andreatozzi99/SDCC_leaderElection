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
var nodeList []Node    // Lista dei nodi di cui un nodo è a conoscenza
var leaderID = -1      // ID dell'attuale leader
var leaderAddress = "" // Indirizzo dell'attuale leader, inizialmente sconosciuto

// Node Struttura per rappresentare un nodo
type Node struct {
	ID        int
	IPAddress string
	Port      int
}

// ELECTION Metodo per avviare un processo di elezione
func (n *Node) ELECTION(senderNode Node, reply *bool) error {
	// Se l'ID del chiamante è più basso di quello del nodo corrente, invoca il metodo "STOP" del chiamante
	if senderNode.ID < n.ID {
		// qui sto dando per implicito che tutti gli indirizzi siano "localhost:", ma non va bene
		client, err := rpc.Dial("tcp", fmt.Sprintf(senderNode.IPAddress, senderNode.Port))
		if err != nil {
			fmt.Println("Errore nella connessione con il nodo :", senderNode.ID, ",", senderNode.IPAddress, senderNode.Port)
			return err
		}
		defer func(client *rpc.Client) {
			err := client.Close()
			if err != nil {
				return
			}
		}(client)
		// Invocazione metodo STOP sul nodo
		var reply bool
		err = client.Call("Node.Stop", 0, &reply)
		if err != nil {
			fmt.Println("Errore durante la chiamata RPC per invocare il metodo 'STOP' sul nodo", senderNode.ID)
			return err
		}
	}
	// Non fare nulla se l' ID del chiamante è più alto dell' ID locale
	return nil
}

// HeartBeat Metodo per contattare il nodo Leader e verificare se esse è ancora attivo
func (n *Node) HeartBeat(senderID int, reply *bool) error {
	*reply = true // Se la funzione viene avviata, vuol dire che il nodo è funzionante
	fmt.Printf("LEADER %d :Msg di HeartBeat da parte di -->%d\n", n.ID, senderID)
	return nil
}

func main() {
	// Creazione struttura nodo
	node := &Node{
		ID:        0, // cosi il serverRegistry assegna il giusto ID
		IPAddress: "localhost",
	}
	// Genera una porta casuale disponibile per il nodo
	node.Port = findAvailablePort()

	// Avvio nodo
	node.start()

	// Mantiene il programma in esecuzione
	select {}
}

// Metodo per avviare il nodo e registrarne l'indirizzo nel server di registrazione
// e prendere lista dei nodi in rete
func (n *Node) start() {

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
	// Funzione ausiliaria per stampa del vettore dei nodi (nodi che il serverRegistry conosce)
	printNodeList()

	// Scoperta dell'attuale leader della rete
	// -> inizio processo di Elezione, più messaggi sulla rete, ma non richiede ulteriore funzioni
	n.startElection() // Non è possibile farla in una goRoutine, le operazioni sottostanti dipendono dall'indirizzo del Leader

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

// Metodo per contattare il nodo leader tramite la chiamata rpc HeartBeat esposta dai nodi
func (n *Node) isLeaderAlive() bool {

	// Se il nodo stesso è il leader, esci
	if n.ID == leaderID {
		return true
	}

	// Una nuova connessione con il nodo leader per ogni ciclo di heartbeat?
	client, err := rpc.Dial("tcp", leaderAddress)
	if err != nil {
		// Se non è possibile contattare il leader, avvia un processo di elezione
		fmt.Println("Node", n.ID, "failed to contact leader Node", leaderID, ":", leaderAddress)
		// Dovrei contattare il serverRegistry per notificare il crash?
		return false
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	// Chiamata RPC HeartBeat per il meccanismo di failure detection al nodo leader
	var reply bool
	err = client.Call("Node.HeartBeat", n.ID, &reply)
	if err != nil {
		// Se il leader non risponde, avvia un processo di elezione
		fmt.Println("Node", n.ID, "failed to contact leader Node", leaderID)
		// Dovrei contattare il serverRegistry per notificare il crash?
		return false
	}
	//chiamata andata a buon fine
	return true
}

// Metodo per avviare un processo di elezione,
// e invocare la chiamata rpc ELECTION solo sui nodi nella rete con ID superiore all' ID locale
func (n *Node) startElection() {

	for _, node := range nodeList {
		if node.ID > n.ID {
			// Avviata go routine per contattare il nodo
			go func(nodeID int) {
				// Effettua una chiamata RPC al nodo specificato per avviare un processo di elezione
				client, err := rpc.Dial("tcp", fmt.Sprintf(node.IPAddress, node.Port))
				if err != nil {
					fmt.Println("Errore nella chiamata RPC per avviare l'elezione sul nodo", node.Port)
					return
				}
				defer func(client *rpc.Client) {
					err := client.Close()
					if err != nil {
						return
					}
				}(client)

				var reply bool
				err = client.Call("Node.Election", n.ID, &reply)
				if err != nil {
					fmt.Println("Errore durante la chiamata RPC per avviare l'elezione sul nodo", nodeID)
					return
				}
			}(node.ID)
		}
	}
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
