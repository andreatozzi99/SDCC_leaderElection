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
// 1) Siamo in un sistema distribuito SINCRONO: conosciamo il tempo massimo di trasmissione.
// 2) Tutti i Nodi nella rete, non solo il Leader, possono avere un Crash e Tornare attivi
var serverAddress = "localhost:8080"

// ---------------- Variabili che rappresentano lo stato di un nodo ------------------
var nodeList = make([]Node, 0) // Lista dei nodi di cui un nodo è a conoscenza
var leaderID = -1              // ID dell'attuale leader. Se < 0 => Leader sconosciuto
var leaderAddress = ""         // Indirizzo dell'attuale leader+Porta, inizialmente sconosciuto
var election = false           // True = Elezione in corso | False = Nessuna Elezione in corso
var electionMutex sync.Mutex   // Lucchetto per l'accesso alla variabile election
var hbState = HeartBeatState{active: false, interrupt: make(chan struct{})}
var hbStateMutex sync.Mutex // Lucchetto per l'accesso alla variabile hbState

// HeartBeatState Struct per gestire lo stato della heartBeatRoutine
type HeartBeatState struct {
	active    bool
	interrupt chan struct{} // Canale per interrompere heartBeatRoutine
}

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
	go n.addNodeInNodeList(senderNode)
	//########## modificare con verbose
	fmt.Printf("Nodo %d <-- ELECTION Nodo %d\n", n.ID, senderNode.ID)

	/*if n.ID == leaderID {// Se sono il leader, non devo avviare un elezione
	return nil}
	*/ //############## Eliminato dato che in seguito a eventi di crash e resume un nodo potrebbe cercare il leader anche se l'attuale leader è attivo

	// Devo interrompere la routine di HeartBeat, è preferibile non contattare il leader,
	// porterebbe alla scoperta del crash anche per questo nodo, con conseguente elezione
	// Elimino i dati del Leader che ho attualmente? --> Pericoloso
	// go interruptHeartBeatRoutine()
	leaderID = -1 // Per evitare che la routine di HeartBeat contatti il leader
	// ######### Può portare problemi perché un nodo potrebbe ricevere msg election dopo aver impostato correttamente i valori, e perdere il riferimento al leader
	leaderAddress = ""
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
			}
		}(client)
		// -------------- Invocazione metodo STOP sul senderNode ------------------------
		// non può diventare leader se il suo ID non è il massimo nella rete
		*reply = false
		err = client.Call("Node.STOP", n, &reply)
		if err != nil {
			fmt.Println("Errore durante la chiamata RPC per invocare il metodo 'STOP' sul nodo", senderNode.ID, err)
		}
		fmt.Printf("Nodo %d: STOP --> Nodo %d\n", n.ID, senderNode.ID)
		// -------------- Avvio Elezione ---------------------
		// #################: non devo avviare un elezione se sono già stato interrotto (STOP) una volta
		// Esempio: sono 2, crash nodo Leader 5, inizio elezione, 4 mi invia STOP, fermo elezione,
		// 			il nodo 1 mi invia ELECTION, inizio elezione=> NON VA BENE
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
	election = false // Indagato per bug: eletto nodo con id non massimo
	electionMutex.Unlock()
	//  ---------- Aggiungo senderNode nella nodeList in caso non sia già presente -------------
	// Notare che per un grande numero di nodi nella rete, eseguire questo controllo per ogni messaggio Coordinator, potrebbe generare problemi.
	go n.addNodeInNodeList(senderNode)

	// --------------------- Aggiorno il riferimento al Leader -----------------------
	leaderID = senderNode.ID
	leaderAddress = senderNode.IPAddress + ":" + strconv.Itoa(senderNode.Port)
	// ################### AGGIUNGERE VERBOSE ###################
	fmt.Printf("Nodo %d <-- COORDINATOR Nodo %d . Leader = %d \n", n.ID, senderNode.ID, senderNode.ID)

	// --------- Riattiva routine di HeartBeat ----------
	// n.startHeartBeatRoutine()
	// Restituisci true per indicare che il leader è stato aggiornato con successo
	*reply = true
	return nil
}

// STOP verrà invocata dai nodi della rete per fermare il nostro proposito di diventare LEADER
func (n *Node) STOP(senderNode Node, reply *bool) error {
	// Imposta la variabile election come FALSE per interrompere l'elezione in corso
	electionMutex.Lock()
	if !election {
		// Non c'è un elezione in corso, ma è stato invocato il metodo STOP -> Print per analizzare il sistema
		fmt.Printf("Nodo %d <-- STOP Nodo %d.\n", n.ID, senderNode.ID)
		electionMutex.Unlock()
		*reply = true
		return nil
	}
	election = false
	electionMutex.Unlock()
	fmt.Printf("Nodo %d <-- STOP Nodo %d. Elezione interrotta.\n", n.ID, senderNode.ID)
	*reply = true
	return nil
}

// HEARTBEAT Metodo per contattare il nodo Leader e verificare se esse è ancora attivo
func (n *Node) HEARTBEAT(senderID int, reply *bool) error {
	*reply = true // Se la funzione viene avviata, vuol dire che il nodo è funzionante
	// ################### AGGIUNGERE VERBOSE #############
	fmt.Printf("LEADER %d| <-- HEARTBEAT Nodo %d\n", n.ID, senderID)
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
		return
	}
	node.start() // Avvio nodo (Ingresso nella rete tramite nodeRegistry ed esposizione dei servizi remoti del nodo)
	select {}    // Mantiene il programma in esecuzione
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
	client, err := rpc.Dial("tcp", serverAddress)
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
	n.startElection()
	// ------------ Avvio heartBeatRoutine per contattare periodicamente il Leader ------------
	n.startHeartBeatRoutine()
	// ------------ Avvio goRoutine per simulare il crash ------------------
	go emulateCrash(n, listener)
	return
}

// Meccanismo di Failure Detection:
// Contattare il nodo leader periodicamente (Invocazione del metodo remoto: HEARTBEAT)
func (n *Node) heartBeatRoutine() {
	fmt.Printf("\n -------- Nodo %d, heartBeatRoutine avviata --------\n", n.ID)
	randSource := rand.NewSource(time.Now().UnixNano())
	randGen := rand.New(randSource)
	for hbState.active {
		select {
		case <-hbState.interrupt: //eliminare utilizzo del canale
			// heartBeatRoutine è stata interrotta
			fmt.Println("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
			return
		default: // Continua con heartBeatRoutine
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
				go n.startElection() // Avvia un'elezione
			}
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
	if hbState.active {
		hbState.active = false
		close(hbState.interrupt)
	}
}

// Funzione per ripristinare heartBeatRoutine se non è attiva. (Per impedire di attivarne più di una)
func (n *Node) startHeartBeatRoutine() {
	hbStateMutex.Lock()
	if hbState.active == false {
		hbState.active = true
		hbState.interrupt = make(chan struct{})
		go n.heartBeatRoutine()
	}
	hbStateMutex.Unlock()
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
		leaderID = -1
		leaderAddress = ""
	}
	electionMutex.Unlock()
	// --------- Invia messaggi ELECTION a nodi con ID maggiore dell'ID locale ----------
	fmt.Println("Avvio un processo di elezione")
	i := 0
	for _, node := range nodeList {
		if node.ID > n.ID {
			// Avvia goroutine per inviare messaggio ELECTION al nodo con ID maggiore
			go n.sendElectionMessage(node)
			i++
		}
	}
	// ---------- Avvia un timer per attendere messaggi di tipo STOP ------------
	if i != 0 {
		time.Sleep(time.Duration(1) * time.Second)
		// ---------- Se l'elezione è stata interrotta, termina la funzione -------------
		electionMutex.Lock()
		if !election {
			electionMutex.Unlock() // Il print non serve
			fmt.Println("L'elezione è stata interrotta, non posso diventare il Leader")
			return
		}
		electionMutex.Unlock()
	}
	// ----------- SONO IL LEADER ------------
	fmt.Printf("\nNodo %d SONO LEADER. COORDINATOR --> tutta la rete.\n", n.ID)
	leaderID = n.ID
	leaderAddress = n.IPAddress + ":" + strconv.Itoa(n.Port)
	go n.sendCoordinatorMessages()
	//go n.startHeartBeatRoutine()
	// ---------------- Imposto election come false, elezione terminata --------------
	electionMutex.Lock()
	election = false // PROVA: SPOSTATO SOTTO LA PRINT, INVECE CHE PRIMA
	electionMutex.Unlock()
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

// Un nodo che è in crash:
// 1) SMETTE DI INVOCARE RPC SUGLI ALTRI NODI
// 2) NON ACCETTA CHIAMATE RPC DA ALTRI NODI
// Metodo utilizzato:
// Interrompere l'esposizione delle chiamate RPC (e riattivarla quando il nodo torna operativo)
// Per decidere quando interrompere o riattivare il servizio RPC vorrei utilizzare
// un ciclo for che genera un numero random(compreso tra 1 e Numero definito come variabile globale)
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
		// Se il numero generato è 0, cambia lo stato del nodo da attivo a in crash o viceversa
		if randomNum == 0 {
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
				node.startElection()
			}
		}
	}
}
