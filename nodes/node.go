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
	"time"
)

//------------------------------------------------------------------------------------------

// NodeBully Struttura per rappresentare un nodo
type NodeBully struct {
	ID        int
	IPAddress string
	Port      int
}

// Metodo per avviare il nodo e registrarne l'indirizzo nel server di registrazione
// e recuperare la lista dei nodi in rete
func (n *NodeBully) start() {

	// #################### Esposizione dei metodi per le chiamate RPC ####################
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

	// #################### Ingresso nella rete tramite NodeBully Registry ####################
	// ----- Connessione con il server di registrazione dei nodi -----
	client, err := rpc.Dial("tcp", serverAddressAndPort)
	if err != nil {
		fmt.Println("Errore nella connessione al NodeRegistry server:", err)
		return
	}
	// ----- Chiusura connessione con defer -----
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	// --------------------- Chiamate RPC per Registrare il nodo ---------------------
	var reply int
	// TODO ################# Nome servizio su file CONF ###################
	if runInContainer {
		localIP, err := getLocalIP()
		if err != nil {
			fmt.Println("Errore durante l'ottenimento dell'indirizzo IP locale:", err)
			return
		}
		n.IPAddress = localIP
	} else {
		n.IPAddress = localAddress
	}
	fmt.Println("My IP Address: ", n.IPAddress)

	err = client.Call("NodeRegistry.RegisterNode", n, &reply)
	if err != nil {
		fmt.Println("Errore durante la registrazione del nodo:", err)
		return
	}
	if reply == -1 {
		fmt.Println("Il server di registrazione ha rifiutato la mia richiesta di registrazione")
		return
	}
	// Imposto il mio nuovo ID (restituito dalla chiamata rpc al nodeRegistry)
	n.ID = reply
	// Visualizza l'ID e la porta assegnati al nodo
	fmt.Printf("-------- Regisrato sulla rete. ID %d: Porta  %d --------\n", n.ID, n.Port)

	// ############# Salva lo stato nel file di log ###############
	if runInContainer {
		if err := saveState(n.ID, n.IPAddress, n.Port); err != nil {
			fmt.Println("Errore durante il salvataggio dello stato:", err)
			return
		}
	}

	// ################# RPC per ricevere la lista dei nodi nella rete  #################
	// TODO ############ Nome servizio su file CONF ###################
	err = client.Call("NodeRegistry.GetRegisteredNodes", n, &nodeList)
	if err != nil {
		fmt.Println("Errore durante la richiesta della lista dei nodi:", err)
		return
	}
	// ------------ Scoperta dell'attuale Leader ----------------
	n.startBullyElection()

	// ------------ Avvio heartBeatRoutine per contattare periodicamente il Leader ------------
	n.startHeartBeatRoutine()
	// ------------ Avvio goRoutine per simulare il crash ------------------
	if emulateLocalCrash {
		go emulateCrash(n, nil, listener)
	}
	return
}

// Meccanismo di Failure Detection:
// Contatta il nodo leader a intervalli casuali (Invocazione del metodo remoto: HEARTBEAT)
func (n *NodeBully) heartBeatRoutine() {
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
func (n *NodeBully) sendHeartBeatMessage() bool {
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
		fmt.Println("Nodo", n.ID, "failed connection with leader NodeBully", leaderID, ":", leaderAddress)
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
	err = client.Call("NodeBully.HEARTBEAT", n.ID, &reply)
	if err != nil {
		// Se il leader non risponde, avvia un processo di elezione
		fmt.Println("Nodo", n.ID, "failed to contact leader NodeBully for HeartBeat", leaderID)
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
func (n *NodeBully) startHeartBeatRoutine() {
	hbStateMutex.Lock()
	if hbState == false {
		hbState = true
		go n.heartBeatRoutine()
	}
	hbStateMutex.Unlock()
}
