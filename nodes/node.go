// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"
	"sync"
	"time"
)

// Assunzioni preliminari:
// Siamo in un sistema distribuito SINCRONO: conosciamo il tempo massimo di trasmissione.
// Timeout da attendere senza nessuna risposta prima di diventare Leader in seguito a un processo di ELEZIONE

// ---------------- Variabili che rappresentano lo stato di un nodo ------------------

var nodeList = make([]Node, 0) // Lista dei nodi di cui un nodo è a conoscenza
var leaderID = -1              // ID dell'attuale leader
var leaderAddress = ""         // Indirizzo dell'attuale leader+Porta, inizialmente sconosciuto
var election = false           // True = Elezione in corso | False = Nessuna Elezione in corso
var electionMutex sync.Mutex   // Lucchetto per l'accesso alla variabile election
var hbStateMutex sync.Mutex

// Definiamo una variabile globale per lo stato della heartBeatRoutine
var hbState = HeartBeatState{active: true, interrupt: make(chan struct{})}

// HeartBeatState Struct per gestire lo stato della heartBeatRoutine
type HeartBeatState struct {
	active    bool
	interrupt chan struct{} // Canale per interrompere heartBeatRoutine
}

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
// Se ID del senderNode è minore dell' ID locale, invio messaggio STOP e avvio nuova Elezione
func (n *Node) ELECTION(senderNode Node, reply *bool) error {
	go n.addNodeInNodeList(senderNode) // Potenzialmente inutile
	//########## modificare con verbose
	fmt.Println("Ricevuto messaggio ELECTION da:", senderNode.ID)

	if n.ID == leaderID { // Se sono il leader, non devo avviare un elezione
		return nil
	}
	// Devo interrompere la routine di HeartBeat, è preferibile non contattare il leader,
	// porterebbe alla scoperta del crash anche per questo nodo, con conseguente elezione
	go interruptHeartBeatRoutine()
	// ---------------- Se l'ID del chiamante è più basso di quello del nodo locale ----------------
	if senderNode.ID < n.ID {
		// ------------- Connessione con senderNode --------------
		client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", senderNode.IPAddress, senderNode.Port))
		if err != nil {
			fmt.Println("Errore nella connessione con il nodo per metodo STOP :", senderNode.ID, ",", senderNode.IPAddress, ":", senderNode.Port)
			return err
		}
		defer func(client *rpc.Client) {
			err := client.Close()
			if err != nil {
				return
			}
		}(client)

		// -------------- Invocazione metodo STOP sul senderNode ------------------------
		// non può diventare leader se il suo ID non è il massimo nella rete
		*reply = false
		err = client.Call("Node.STOP", n, &reply)
		if err != nil {
			fmt.Println("Errore durante la chiamata RPC per invocare il metodo 'STOP' sul nodo", senderNode.ID, err)
		}
		// -------------- Avvio Elezione ---------------------
		go n.startElection()
	}
	// ------------ Se l' ID del chiamante è più alto dell' ID locale --------------------
	// Questo non dovrebbe succedere secondo l'algoritmo bully che invia messaggi di election solo a nodi con id maggiore
	*reply = true
	return nil
}

// COORDINATOR viene invocata da un nodo nella rete,
// per comunicare che è lui il NUOVO LEADER-> cambio valori di leaderID e leaderAddress.
func (n *Node) COORDINATOR(senderNode Node, reply *bool) error {
	// --------------------- Interrompi un eventuale elezione in corso --------------
	electionMutex.Lock()
	election = false
	electionMutex.Unlock()
	//  ---------- Aggiungo senderNode nella nodeList in caso non sia già presente -------------
	// Notare che per un grande numero di nodi nella rete, eseguire questo controllo per ogni messaggio Coordinator, potrebbe generare problemi.
	go n.addNodeInNodeList(senderNode)

	// --------------------- Aggiorno il riferimento al Leader -----------------------
	leaderID = senderNode.ID
	leaderAddress = senderNode.IPAddress + ":" + strconv.Itoa(senderNode.Port)
	// ################### AGGIUNGERE VERBOSE #############
	fmt.Printf("Ho ricevuto COORDINATOR da parte del nodo -->%d, è lui il nuovo leader \n", senderNode.ID)

	// --------- Riattiva routine di HeartBeat ----------
	n.restoreHeartBeatRoutine()
	// Restituisci true per indicare che il leader è stato aggiornato con successo
	*reply = true
	return nil
}

// STOP verrà invocata dai nodi della rete per fermare il nostro proposito di diventare LEADER
func (n *Node) STOP(senderNode Node, reply *bool) error {
	// Imposta la variabile election come FALSE per interrompere l'elezione in corso
	electionMutex.Lock()
	if !election {
		electionMutex.Unlock()
		*reply = true
		return nil
	}
	election = false
	electionMutex.Unlock()
	fmt.Printf("Nodo: %d, invocato il metodo STOP dal nodo %d. L'elezione è stata interrotta.\n", n.ID, senderNode.ID)
	*reply = true
	return nil
}

// HEARTBEAT Metodo per contattare il nodo Leader e verificare se esse è ancora attivo
func (n *Node) HEARTBEAT(senderID int, reply *bool) error {
	*reply = true // Se la funzione viene avviata, vuol dire che il nodo è funzionante
	// ################### AGGIUNGERE VERBOSE #############à
	fmt.Printf("LEADER| Nodo %d, Msg di HEARTBEAT ricevuto dal nodo: %d\n", n.ID, senderID)
	return nil
}

func main() {
	// Creazione struttura nodo
	node := &Node{
		ID:        -1, // Per il nodeRegistry, che assegnerà un nuovo ID
		IPAddress: "localhost",
	}
	// Genera una porta casuale disponibile per il nodo
	node.Port, _ = findAvailablePort()
	if node.Port == 0 {
		fmt.Println("Errore nella ricerca di una porta")
	}

	// Avvio nodo (Ingresso nella rete tramite nodeRegistry ed esposizione dei servizi remoti del nodo)
	node.start()

	// Mantiene il programma in esecuzione
	select {}

}

// Metodo per avviare il nodo e registrarne l'indirizzo nel server di registrazione
// e recuperare la lista dei nodi in rete
func (n *Node) start() {
	// ------------- Avvio goRoutine per simulare il crash ------------------
	go emulateCrash() // Crash = Chiusura del programma tramite funzione di exit

	// ------------- Esposizione dei metodi per le chiamate RPC -------------
	err := rpc.Register(n)
	if err != nil {
		return
	}

	// Creazione listener RPC
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", n.Port))
	if err != nil {
		fmt.Println("Errore durante la creazione del listener RPC:", err)
		return
	}

	// -------- Accetta le connessioni per RPC in arrivo in una goroutine ------------
	fmt.Printf("Nodo in ascolto su porta %d per le chiamate RPC...\n", n.Port)
	go rpc.Accept(listener)

	// ------------- Ingresso nella rete tramite Node Registry ------------------
	client, err := rpc.Dial("tcp", "localhost:8080")
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
	fmt.Printf("Regisrato sulla rete. ID %d: Porta %d\n", n.ID, n.Port)

	// ---------- RPC per ricevere la lista dei nodi nella rete  ----------
	// ############ Nome servizio su file CONF ###################
	err = client.Call("NodeRegistry.GetRegisteredNodes", n, &nodeList)
	if err != nil {
		fmt.Println("Errore durante la richiesta della lista dei nodi:", err)
		return
	}
	// Funzione ausiliaria per stampa del vettore dei nodi restituita dal nodeRegistry
	printNodeList()

	// ---------------------- Scoperta dell'attuale Leader ----------------------
	n.startElection() // Non è possibile eseguirla in una goRoutine, le operazioni sottostanti dipendono dall'indirizzo del Leader
	// Contro: Più messaggi sulla rete
	// Pro: il Leader non è in crash, ma se un nuovo nodo entra, ha ID più alto.
	// Pro: Non richiede ulteriore funzioni e si sfrutta l'algoritmo di elezione
	if leaderID == -1 {
		_ = fmt.Errorf("Node:%d ERRORE, nessun leader impostato a seguito di una Elezione\n", n.ID)
	}
	// ------------ Avvio heartBeatRoutine per contattare periodicamente il Leader ------------
	go n.heartBeatRoutine()
	return
}

// Meccanismo di Failure Detection:
// Contattare il nodo leader periodicamente (Invocazione del metodo remoto: HEARTBEAT)
func (n *Node) heartBeatRoutine() {
	fmt.Printf("\nNodo %d, heartBeatRoutine avviata\n", n.ID)
	for hbState.active {
		select {
		case <-hbState.interrupt:
			// heartBeatRoutine è stata interrotta
			fmt.Println("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
			return
		default:
			// Continua con heartBeatRoutine
			// Verifica se il nodo leader è raggiungibile
			if !n.sendHeartBeatMessage() {
				// Se il leader non è raggiungibile, avvia un'elezione
				leaderID = -1
				go n.startElection() //prova: ho tolto go qui
				return
			}
			// Attendi un intervallo casuale prima di effettuare un nuovo heartbeat
			randomSeconds := rand.Intn(20-1) + 1
			interval := time.Duration(randomSeconds) * time.Second
			timer := time.NewTimer(interval)
			<-timer.C
		}
	}
	fmt.Printf("\nNodo %d, heartBeatRoutine interrotta\n", n.ID)
}

// Metodo per contattare il nodo leader tramite la chiamata rpc HEARTBEAT esposta dai nodi
// @return: True se il leader risponde, False altrimenti.
func (n *Node) sendHeartBeatMessage() bool {

	if n.ID == leaderID { // Se il nodo che avvia questa funzione è il leader, non fare nulla
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
	fmt.Printf("Nodo %d, HEARTBEAT al nodo Leader(ID %d) \n", n.ID, leaderID)
	return true
}

// Funzione per interrompere heartBeatRoutine
func interruptHeartBeatRoutine() {
	hbStateMutex.Lock()
	defer hbStateMutex.Unlock()
	if hbState.active {
		hbState.active = false
		close(hbState.interrupt)
	}
}

// Funzione per ripristinare heartBeatRoutine
func (n *Node) restoreHeartBeatRoutine() {
	hbStateMutex.Lock()
	hbState.active = true
	hbState.interrupt = make(chan struct{})
	defer hbStateMutex.Unlock()
	go n.heartBeatRoutine()
}

// Metodo per avviare un processo di elezione,
// invio messaggio ELECTION solo sui nodi nella rete con ID superiore all' ID locale
func (n *Node) startElection() {
	// ---------------- Verifico se c'è già un elezione in atto --------------
	electionMutex.Lock()
	if election { // C'è già un elezione in corso, non devo avviare un altro processo di elezione
		electionMutex.Unlock()
		return
	} else { // Non c'era un elezione in corso, si procede con l'elezione
		election = true
	}
	electionMutex.Unlock()
	// --------- Invia messaggi ELECTION a nodi con ID maggiore dell'ID locale ----------
	fmt.Println("Avvio un processo di elezione")
	for _, node := range nodeList {
		if node.ID > n.ID {
			// Avvia goroutine per inviare messaggio ELECTION al nodo con ID maggiore
			go n.sendElectionMessage(node)
		}
	}
	// ---------- Avvia un timer per attendere messaggi di tipo STOP ------------
	time.Sleep(5 * time.Second)
	// ---------- Se l'elezione è stata interrotta, termina la funzione -------------
	electionMutex.Lock()
	if !election {
		electionMutex.Unlock() // Il print non serve
		fmt.Println("L'elezione è stata interrotta, non posso diventare il Leader")
		return
	}
	fmt.Printf("\nNodo:%d -> Nessun nodo con ID superiore ha interrotto l'elezione. Invio messaggio COORDINATOR a tutti.\n", n.ID)
	// ---------------- Imposto election come false, elezione terminata --------------
	election = false
	electionMutex.Unlock()
	leaderID = n.ID
	leaderAddress = n.IPAddress + ":" + strconv.Itoa(n.Port)
	go n.sendCoordinatorMessages()
	go n.restoreHeartBeatRoutine()
}

// Invoca procedura remota ELECTION sul nodo inserito come parametro
// @return: true se posso continuare elezione, false per interrompere processo di elezione
func (n *Node) sendElectionMessage(node Node) bool {
	// Effettua una chiamata RPC al nodo specificato per avviare un processo di elezione
	client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", node.IPAddress, node.Port))
	if err != nil {
		fmt.Println("Errore nella chiamata ELECTION sul nodo", node.ID, ",non è raggiungibile", node.IPAddress, ":", node.Port, "ERRORE:", err)
		return false
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	var reply = true
	err = client.Call("Node.ELECTION", n, &reply)
	if err != nil {
		fmt.Println("Errore durante la chiamata RPC per avviare l'elezione sul nodo, non è raggiungibile", node.ID)
		return true
	}
	return true
}

// Invoca la procedura remota COORDINATOR su tutti i nodi nella nodeList per notificare il nuovo Leader
// Invoca la procedura remota COORDINATOR su tutti i nodi nella nodeList per notificare il nuovo Leader
func (n *Node) sendCoordinatorMessages() {
	// Itera su tutti i nodi nella nodeList
	for _, node := range nodeList {
		if node.ID != n.ID { // Se il nodo nella è diverso da me stesso
			// Avvia una goroutine per invocare la procedura remota COORDINATOR sul nodo
			go func(node Node) {
				// Effettua una chiamata RPC per invocare la procedura remota COORDINATOR
				client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", node.IPAddress, node.Port))
				if err != nil {
					fmt.Printf("Errore durante la connessione al nodo %d: %v\n", node.ID, err)
					return
				}
				defer func(client *rpc.Client) {
					err := client.Close()
					if err != nil {
						fmt.Printf("Errore durante la chiusura della connessione al nodo %d: %v\n", node.ID, err)
					}
				}(client)
				// Variabile per memorizzare la risposta remota
				var reply bool
				// Effettua la chiamata RPC
				err = client.Call("Node.COORDINATOR", n, &reply)
				if err != nil {
					fmt.Printf("Errore durante la chiamata RPC COORDINATOR al nodo %d: %v\n", node.ID, err)
					return
				}
				// Stampa un messaggio di conferma
				fmt.Printf("Chiamata RPC COORDINATOR al nodo %d completata con successo\n", node.ID)
			}(node)
		}
	}
}

// Funzione ausiliaria per aggiungere il senderNode se non è presente nella nodeList
func (n *Node) addNodeInNodeList(senderNode Node) {
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

// Funzione per emulare il crash di un nodo. Come avviene:
func emulateCrash() {
	//TODO
	// Attualmente non può avvenire crash, le prove verranno effettuate da terminale
}

// Stampa dei nodi nella rete
func printNodeList() {
	fmt.Print("I nodi sulla rete sono:")
	for _, node := range nodeList {
		fmt.Printf("ID: %d, IP: %s, Port: %d\n", node.ID, node.IPAddress, node.Port)
	}
}
