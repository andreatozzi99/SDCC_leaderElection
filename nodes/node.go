// Studente Andrea Tozzi, MATRICOLA: 0350270
// Assunzioni preliminari:
// 1) Siamo in un sistema distribuito SINCRONO: conosciamo il tempo massimo di trasmissione.
// 2) Tutti i Nodi nella rete, non solo il Leader, possono avere un Crash e Tornare attivi
package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
)

// ---------------- Variabili che rappresentano lo stato di un nodo ------------------
var (
	nodeList      = make([]Node, 0) // Lista dei nodi di cui un nodo è a conoscenza
	leaderID      = -1              // ID dell'attuale leader. Se < 0 => Leader sconosciuto
	leaderAddress = ""              // Indirizzo dell'attuale leader+Porta, inizialmente sconosciuto
	election      = false           // True = Elezione in corso | False = Nessuna Elezione in corso
	electionMutex sync.Mutex        // Lucchetto per l'accesso alla variabile election
	hbState       = false
	hbStateMutex  sync.Mutex // Lucchetto per l'accesso alla variabile hbState
)

//------------------------------------------------------------------------------------------

// Node Struttura per rappresentare un nodo
type Node struct {
	ID        int
	IPAddress string
	Port      int
}

// HEARTBEAT Utilizzato dai nodi nella rete sul Leader come meccanismo di Failure Detection
func (n *Node) HEARTBEAT(senderID int, reply *bool) error {
	*reply = true // Se la funzione viene avviata, vuol dire che il nodo è funzionante
	fmt.Printf("LEADER %d| <-- HEARTBEAT Nodo %d\n", n.ID, senderID)
	return nil
}

func main() {
	// Creazione del nodo o del RaftNode in base all'algoritmo di elezione
	var node interface{}
	if electionAlg == "Bully" {
		node = &Node{
			ID:        -1, // Per il nodeRegistry, che assegnerà un nuovo ID
			IPAddress: localAddress,
		}
	} else if electionAlg == "Raft" {
		node = &RaftNode{
			ID:          -1, // Per il nodeRegistry, che assegnerà un nuovo ID
			IPAddress:   localAddress,
			CurrentTerm: 0,
			VotedFor:    -1,
		}
	} else {
		fmt.Println("Algoritmo di elezione non supportato:", electionAlg)
		return
	}

	// Verifica il tipo di nodo e imposta la porta
	switch n := node.(type) {
	case *Node:
		port, err := findAvailablePort()
		if err != nil {
			fmt.Println("Errore nella ricerca di una porta:", err)
			return
		}
		n.Port = port
		n.start()
	case *RaftNode:
		port, err := findAvailablePort()
		if err != nil {
			fmt.Println("Errore nella ricerca di una porta:", err)
			return
		}
		n.Port = port
		n.start()
	}
	select {} // Mantiene il programma in esecuzione
}

// Metodo per avviare il nodo e registrarne l'indirizzo nel server di registrazione
// e recuperare la lista dei nodi in rete
func (n *Node) start() {
	// ------------- Esposizione dei metodi per le chiamate RPC -------------
	err := rpc.Register(n)
	if err != nil {
		return
	}
	// ------ Creazione listener RPC ------
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", n.Port))
	if err != nil {
		fmt.Println("Errore durante la creazione del listener RPC:", err)
		return
	}
	// -------- Accetta le connessioni per RPC in arrivo in una goroutine ------------
	fmt.Printf("Nodo in ascolto su porta %d per le chiamate RPC...\n", n.Port)
	go rpc.Accept(listener)
	// ------------- Ingresso nella rete tramite Node Registry ------------------
	client, err := rpc.Dial("tcp", serverAddressAndPort)
	if err != nil {
		fmt.Println("Errore nella connessione al NodeRegistry server:", err)
		return
	}
	// ---- Chiusura connessione con defer ----
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	// ---------- RPC per Registrare il nodo --------------
	var reply int
	// ################# Nome servizio su file CONF ###################
	err = client.Call("NodeRegistry.RegisterNode", n, &reply)
	if err != nil {
		fmt.Println("Errore durante la registrazione del nodo:", err)
		return
	}
	if reply == -1 {
		fmt.Println("Il server di registrazione ha rifiutato la registrazione del nodo")
		return
	}
	// Imposto il mio nuovo ID (restituito dalla chiamata rpc al nodeRegistry)
	n.ID = reply
	// Visualizza l'ID e la porta assegnati al nodo
	fmt.Printf("-------- Regisrato sulla rete. ID %d: Porta  %d --------\n", n.ID, n.Port)

	// ---------- RPC per ricevere la lista dei nodi nella rete  ----------
	// ############ Nome servizio su file CONF ###################
	err = client.Call("NodeRegistry.GetRegisteredNodes", n, &nodeList)
	if err != nil {
		fmt.Println("Errore durante la richiesta della lista dei nodi:", err)
		return
	}
	// ------------ Scoperta dell'attuale Leader ----------------
	n.startBullyElection() // Dovrebbe variare in base all'algoritmo selezionato
	// ------------ Avvio heartBeatRoutine per contattare periodicamente il Leader ------------
	n.startHeartBeatRoutine()
	// ------------ Avvio goRoutine per simulare il crash ------------------
	go emulateCrash(n, listener)
	return
}

// Meccanismo di Failure Detection:
// Contatta il nodo leader periodicamente (Invocazione del metodo remoto: HEARTBEAT)
func (n *Node) heartBeatRoutine() {
	fmt.Printf("\n -------- Nodo %d, heartBeatRoutine avviata --------\n", n.ID)
	randSource := rand.NewSource(time.Now().UnixNano())
	randGen := rand.New(randSource)
	for hbState {
		// Continua con heartBeatRoutine
		// Attendi un intervallo casuale prima di effettuare un nuovo heartbeat
		randomSeconds := randGen.Intn(5-1) + 1
		interval := time.Duration(randomSeconds) * time.Second
		timer := time.NewTimer(interval)
		<-timer.C
		// Verifica se il nodo leader è raggiungibile
		if !n.sendHeartBeatMessage() {
			//interruptHeartBeatRoutine()
			leaderID = -1
			leaderAddress = ""
			go n.startBullyElection() // Avvia un'elezione (posso sceglierla in base all'algoritmo selezionato)
		}
	}
	fmt.Printf("\n -------- Nodo %d, heartBeatRoutine interrotta --------\n", n.ID)
}

// Metodo per contattare il nodo leader tramite la chiamata rpc HEARTBEAT esposta dai nodi
// @return: True se il leader risponde, False altrimenti.
func (n *Node) sendHeartBeatMessage() bool {
	if n.ID == leaderID { // Se il nodo che avvia questa funzione è il leader, non fare nulla
		fmt.Println("LEADER| Non invio HeartBeat")
		return true
	}
	if leaderID < 0 { // Se nessun leader è attualmente impostato, non fare nulla
		return true
	}
	// Una nuova connessione con il nodo leader per ogni ciclo di heartbeat?
	client, err := rpc.Dial("tcp", leaderAddress)
	if err != nil {
		// Se non è possibile contattare il leader, avvia un processo di elezione
		fmt.Println("Nodo", n.ID, "failed connection with leader Node", leaderID, ":", leaderAddress)
		return false
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	// Chiamata RPC HEARTBEAT per il meccanismo di failure detection al nodo leader
	var reply bool
	err = client.Call("Node.HEARTBEAT", n.ID, &reply)
	if err != nil {
		// Se il leader non risponde, avvia un processo di elezione
		fmt.Println("Nodo", n.ID, "failed to contact leader Node for HeartBeat", leaderID)
		return false
	}
	//chiamata andata a buon fine
	fmt.Printf("Nodo %d, messaggio HEARTBEAT al nodo Leader(ID %d) \n", n.ID, leaderID)
	return true
}

// Funzione per interrompere heartBeatRoutine
func interruptHeartBeatRoutine() {
	hbStateMutex.Lock()
	defer hbStateMutex.Unlock()
	if hbState {
		hbState = false
	}
}

// Funzione per ripristinare heartBeatRoutine se non è attiva. (Per impedire di attivarne più di una)
func (n *Node) startHeartBeatRoutine() {
	hbStateMutex.Lock()
	if hbState == false {
		hbState = true
		go n.heartBeatRoutine()
	}
	hbStateMutex.Unlock()
}

// Un nodo che è in crash:
// 1) SMETTE DI INVOCARE RPC SUGLI ALTRI NODI
// 2) NON ACCETTA CHIAMATE RPC DA ALTRI NODI
// Metodo utilizzato:
// Interrompere l'esposizione delle chiamate RPC (e riattivarla quando il nodo torna operativo)
// Per stabilire quando interrompere o riattivare il servizio RPC utilizzo
// un ciclo for che genera un numero random(compreso tra 0 e Numero definito come variabile globale)
// ogni 5 secondi, se il numero generato è uguale a 1, il nodo esegue lo switch di stato (Crashed oppure Attivo)
func emulateCrash(node *Node, listener net.Listener) {
	// Inizializza il generatore di numeri casuali con un seme univoco
	randSource := rand.NewSource(time.Now().UnixNano())
	randGen := rand.New(randSource)
	active := true // Variabile booleana per tenere traccia dello stato del nodo (true = attivo, false = in crash)
	for {
		time.Sleep(5 * time.Second) // Attendi 5 secondi prima di ogni iterazione
		// Genera un numero random tra 0 e 10
		randomNum := randGen.Intn(11)
		// Se il numero generato è 1, cambia lo stato del nodo da attivo a in crash o viceversa
		if randomNum == 1 {
			if active {
				leaderID = -1 // Cambio valori del leader perché non saranno più validi al momento del rientro
				leaderAddress = ""
				//interruptHeartBeatRoutine() // Interrompi heartBeatRoutine per evitare che il nodo comunichi con il Leader
				// Cambia lo stato del nodo da attivo a in crash
				active = false
				// Interrompi il listener RPC
				err := listener.Close()
				if err != nil {
				}
				fmt.Println(" \n-------------------- IL NODO E' IN CRASH --------------------\n ")

			} else {
				// Cambia lo stato del nodo da in crash ad attivo
				fmt.Println(" \n-------------------- IL NODO E' ATTIVO ---------------------\n ")
				active = true
				// Riavvia il listener RPC
				newListener, err := net.Listen("tcp", fmt.Sprintf(":%d", node.Port))
				if err != nil {
					fmt.Println("Errore durante la riapertura del listener RPC:", err)
					return
				}
				fmt.Printf("Nuovo listener RPC aperto sulla porta %d\n", node.Port)
				listener = newListener // Aggiorna il listener con quello appena creato
				// Avvia l'accettazione delle chiamate RPC sul nuovo listener
				go rpc.Accept(listener)

				// CONOSCO I NODI NELLA RETE E SONO REGISTRATO SULLA RETE, NON DEVO ESEGUIRE FUNZIONE DI START
				// NON CONOSCO IL NUOVO LEADER DELLA RETE -> Inizio un elezione
				node.startBullyElection()
			}
		}
	}
}

// Funzione ausiliaria per aggiungere il senderNode se non è presente nella nodeList
func addNodeInNodeList(senderNode Node) {
	found := false
	for _, node := range nodeList {
		if node.ID == senderNode.ID {
			// Il senderNode è già presente nella lista dei nodi
			found = true
			break
		}
	}
	// Se senderNode non è presente in nodeList
	if !found {
		// Aggiungi il senderNode alla lista dei nodi
		nodeList = append(nodeList, senderNode)
	}
}

// Funzione per trovare una porta disponibile casuale
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			return
		}
	}(listener)
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
