package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"
	"time"
)

// Assunzioni preliminari:
// Siamo in un sistema distribuito SINCRONO: conosciamo il tempo massimo di trasmissione.
// Timeout da attendere senza nessuna risposta prima di diventare Leader in seguito a un processo di ELEZIONE

// ------------- Variabili che rappresentano lo stato (variabile) di un nodo --------------
var nodeList []Node    // Lista dei nodi di cui un nodo è a conoscenza
var leaderID = -1      // ID dell'attuale leader
var leaderAddress = "" // Indirizzo dell'attuale leader+Porta, inizialmente sconosciuto
var election = false   // True = Elezione in corso | False = Nessuna Elezione in corso
//var verbose = true // Prendere da file di configurazione per attivare/disattivare commenti aggiuntivi
// ----------------------------------------------------------------------------------------

// Node Struttura per rappresentare un nodo
type Node struct {
	ID        int
	IPAddress string
	Port      int
}

// ELECTION Metodo invocato come rpc dagli altri nodi della rete
// In tal modo notificano la loro volontà di diventare Leader
func (n *Node) ELECTION(senderNode Node, reply *bool) error {
	if true { //########## modificare con verbose
		fmt.Println("Ricevuto messaggio ELECTION da:", senderNode.ID)
	}

	// Se l'ID del chiamante è più basso di quello del nodo corrente, invoca il metodo "STOP" del chiamante
	if senderNode.ID < n.ID {
		// qui sto dando per implicito che tutti gli indirizzi siano "localhost:", ma non va bene
		client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", n.IPAddress, n.Port))
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
		*reply = false
		err = client.Call("Node.Stop", 0, &reply)
		if err != nil {
			fmt.Println("Errore durante la chiamata RPC per invocare il metodo 'STOP' sul nodo", senderNode.ID)
			return err
		}
	}
	// Non fare nulla se l' ID del chiamante è più alto dell' ID locale
	*reply = true
	return nil
}

// COORDINATOR verrà invocata da un nodo nella rete,
// per comunicare che è lui il NUOVO LEADER
func (n *Node) COORDINATOR(senderNode Node, reply *bool) error {
	// TODO
	// Verifico se il senderNode è presente nella mia lista dei nodi, in caso contrario, lo aggiungo
	// Imposto i parametri del leader
	return nil
}

// STOP verrà invocata dai nodi della rete per fermare il nostro proposito di diventare LEADER
func (n *Node) STOP(senderNode Node, reply *bool) {
	// TODO
	// Devo fare qualcosa per bloccare la funzione "startElection"
	// che sta inviando ancora messaggi o attendendo il timeout prima di autoproclamarsi LEADER
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
	if node.Port == 0 {
		fmt.Println("Errore nella ricerca di una porta")
	}
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
	fmt.Printf("Regisrazione sulla rete avvenuta. ID %d: Assigned port %d\n", n.ID, n.Port)

	// Chiamata RPC per richiedere la lista dei nodi al server di registrazione
	// ############ Nome servizio su file CONF ###################
	err = client.Call("ServerRegistry.GetNodeList", 0, &nodeList)
	if err != nil {
		fmt.Println("Errore durante la richiesta della lista dei nodi:", err)
		return
	}
	// Funzione ausiliaria per stampa del vettore dei nodi (nodi che il serverRegistry conosce)
	printNodeList()

	// Se la lista dei nodi online è composta solo da me stesso, sono io il Leader!
	// Verifica se la dimensione di nodeList è 1 e se il nodo presente è il nodo corrente
	if len(nodeList) == 1 {
		if nodeList[0].ID == n.ID {
			// Sono l'unico nodo nella rete -> Sono il Leader !
			leaderID = n.ID
			leaderAddress = fmt.Sprintf("%s:%d", n.IPAddress, n.Port)
		}
	}

	// Non sono io il Leader -> inizio processo di Elezione.
	// Contro: Più messaggi sulla rete, ma non richiede ulteriore funzioni e si sfrutta l'algoritmo di elezione
	n.startElection() // Non è possibile farla in una goRoutine, le operazioni sottostanti dipendono dall'indirizzo del Leader

	// Avvia la goroutine per heartbeat
	go n.heartBeatRoutine()
}

// Meccanismo di Failure Detection:
// contattare il nodo leader periodicamente (Invocazione del metodo remoto: HeartBeat)
func (n *Node) heartBeatRoutine() {
	// Questa routine dovrebbe essere interrotta durante il processo di Elezione di un nuovo leader(?)
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
				client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", n.IPAddress, n.Port))
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
				err = client.Call("Node.ELECTION", n.ID, &reply)
				if err != nil {
					fmt.Println("Errore durante la chiamata RPC per avviare l'elezione sul nodo", nodeID)
					return
				}
			}(node.ID)
		}
	}
	// Dopo che ho inviato tutte le ELECTION, dovrei assicurarmi di non aver o aver ricevuto una rpc STOP
	// Metto un timer(?)
}

// Funzione per trovare una porta disponibile casuale
func findAvailablePort() int {
	// Cerca una porta disponibile partendo da una porta casuale
	startPort := 30000
	for port := startPort; port < 65535; port++ {
		_, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			return port
		}
	}
	return 0 // Ritorna 0 se non viene trovata alcuna porta disponibile
}

// Stampa dei nodi nella rete
func printNodeList() {
	fmt.Print("I nodi sulla rete sono:")
	for _, node := range nodeList {
		fmt.Printf("\nID: %d, IP: %s, Port: %d", node.ID, node.IPAddress, node.Port)
	}
}
