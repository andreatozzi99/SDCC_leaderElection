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

var nodeList = make([]Node, 0)         // Lista dei nodi di cui un nodo è a conoscenza
var leaderID = -1                      // ID dell'attuale leader
var leaderAddress = ""                 // Indirizzo dell'attuale leader+Porta, inizialmente sconosciuto
var election = false                   // True = Elezione in corso | False = Nessuna Elezione in corso
var electionMutex sync.Mutex           // Lucchetto per l'accesso alla variabile election
var controlHeartBeat = make(chan bool) // Canale bidirezionale per attivare/disattivare la goroutine di HeartBeat quando necessario
//var timerInSecond = 5

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
	// Devo interrompere la routine di HeartBeat, invio segnale
	controlHeartBeat <- false

	go n.addNodeInNodeList(senderNode)

	// Se l'ID del chiamante è più basso di quello del nodo corrente, invoca il metodo "STOP" del chiamante
	if senderNode.ID < n.ID {
		// Connessione con nodo
		client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", senderNode.IPAddress, senderNode.Port))
		if err != nil {
			fmt.Println("Errore nella connessione con il nodo :", senderNode.ID, ",", senderNode.IPAddress, ":", senderNode.Port)
			return err
		}
		defer func(client *rpc.Client) {
			err := client.Close()
			if err != nil {
				return
			}
		}(client)

		// Invocazione metodo STOP sul nodo, dato che ID sender < ID locale, lui non può diventare leader
		*reply = false
		err = client.Call("Node.STOP", 0, &reply)
		if err != nil {
			fmt.Println("Errore durante la chiamata RPC per invocare il metodo 'STOP' sul nodo", senderNode.ID)
			return err
		}
	}
	// Non fare nulla se l' ID del chiamante è più alto dell' ID locale
	// Questo non dovrebbe succedere secondo l'algoritmo bully che invia messaggi di election solo a nodi con id maggiore
	*reply = true
	return nil
}

// COORDINATOR viene invocata da un nodo nella rete,
// per comunicare che è lui il NUOVO LEADER-> cambio valori di leaderID e leaderAddress.
func (n *Node) COORDINATOR(senderNode Node, reply *bool) error {

	//  ---------- Aggiungo senderNode nella nodeList in caso non sia già presente -------------
	// Notare che per un grande numero di nodi nella rete, eseguire questo controllo per ogni messaggio Coordinator, potrebbe generare problemi.
	go n.addNodeInNodeList(senderNode)

	// --------------------- Aggiorno il riferimento al Leader -----------------------
	leaderID = senderNode.ID
	leaderAddress = senderNode.IPAddress + ":" + strconv.Itoa(senderNode.Port)
	// ################### AGGIUNGERE VERBOSE #############à
	fmt.Printf("Ho ricevuto COORDINATOR da parte del nodo -->%d, è lui il nuovo leader \n", senderNode.ID)

	// Avvia routine di HeartBeat
	controlHeartBeat <- true

	// Restituisci true per indicare che il leader è stato aggiornato con successo
	*reply = true

	return nil
}

// STOP verrà invocata dai nodi della rete per fermare il nostro proposito di diventare LEADER
func (n *Node) STOP(senderNode Node, reply *bool) {
	// Acquisisci il mutex per garantire la mutua esclusione sull'accesso alla variabile election
	electionMutex.Lock()
	defer electionMutex.Unlock()

	// Imposta la variabile election a false per interrompere l'elezione in corso
	election = false
	*reply = true
	fmt.Printf("Nodo: %d, Il nodo %d ha invocato il metodo STOP. L'elezione è stata interrotta.\n", n.ID, senderNode.ID)
}

// HEARTBEAT Metodo per contattare il nodo Leader e verificare se esse è ancora attivo
func (n *Node) HEARTBEAT(senderID int, reply *bool) error {
	*reply = true // Se la funzione viene avviata, vuol dire che il nodo è funzionante
	// ################### AGGIUNGERE VERBOSE #############à
	fmt.Printf("Nodo LEADER: %d, Msg di HEARTBEAT da parte ddel nodo: %d\n", n.ID, senderID)
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
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
		}
	}(listener)

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

	// Avvio heartBeatRoutine per contattare periodicamente il Leader
	go n.heartBeatRoutine(controlHeartBeat)
	return
}

// Meccanismo di Failure Detection:
// Contattare il nodo leader periodicamente (Invocazione del metodo remoto: HEARTBEAT)
func (n *Node) heartBeatRoutine(control <-chan bool) {
	for {
		// Attendiamo un segnale per verificare se la routine deve essere attiva o meno
		active := <-control

		if active {
			// Continua il normale flusso di heartbeat solo se la routine è attiva
			// Verifica se il nodo leader è raggiungibile
			fmt.Printf("Node %d, Contatto il Leader %d con metodo rpc HEARTBEAT\n", n.ID, leaderID)
			if !n.sendHeartBeatMessage() {
				// se il leader non è raggiungibile -> Indire un processo di Elezione
				go n.startElection()

				// La heartBeatRoutine deve essere interrotta fino a che non ci sarà un nuovo leader
				controlHeartBeat <- false
				continue
			}

			// Attendi un intervallo casuale prima di effettuare un nuovo heartbeat
			// ######### Selezione intervallo su file di configurazione ##########
			randomSeconds := rand.Intn(10-1) + 1
			interval := time.Duration(randomSeconds) * time.Second
			time.Sleep(interval)
		}

		// Attendi un breve intervallo prima di iniziare il prossimo ciclo
		// per ridurre l'uso della CPU
		time.Sleep(100 * time.Millisecond)
	}
}

// Metodo per contattare il nodo leader tramite la chiamata rpc HEARTBEAT esposta dai nodi
// @return: True se il leader risponde, False altrimenti.
func (n *Node) sendHeartBeatMessage() bool {

	// Se il nodo che avvia questa funzione è il leader, esci
	if n.ID == leaderID {
		return true
	}

	// Una nuova connessione con il nodo leader per ogni ciclo di heartbeat?
	client, err := rpc.Dial("tcp", leaderAddress)
	if err != nil {
		// Se non è possibile contattare il leader, avvia un processo di elezione
		fmt.Println("Node", n.ID, "failed to contact leader Node", leaderID, ":", leaderAddress)
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
		fmt.Println("Node", n.ID, "failed to contact leader Node", leaderID)
		return false
	}
	//chiamata andata a buon fine
	return true
}

// Metodo per avviare un processo di elezione,
// e invocare la chiamata rpc ELECTION solo sui nodi nella rete con ID superiore all' ID locale
func (n *Node) startElection() {
	// Contatore per il numero di messaggi di elezione inviati
	electionMessagesSent := 0

	// Invia messaggi ELECTION a nodi con ID maggiore dell' ID locale
	for _, node := range nodeList {
		if node.ID > n.ID {
			// Avvia goroutine per inviare messaggio ELECTION al nodo con ID maggiore
			go func(node Node) {
				if n.sendElectionMessage(node) {
					// Incrementa il contatore se il messaggio di elezione è stato inviato con successo
					electionMessagesSent++
				}
			}(node)
		}
	}

	// Se non è stato inviato alcun messaggio di elezione, nessun nodo con id superiore trovato
	if electionMessagesSent == 0 {
		// Imposta i parametri del nodo corrente come parametri locali del leader
		leaderID = n.ID
		leaderAddress = fmt.Sprintf("%s:%d", n.IPAddress, n.Port)
		fmt.Printf("Nodo:%d -> Non ci sono nodi con ID superiore, invio messaggio COORDINATOR a tutti.\n", n.ID)
		n.sendCoordinatorMessages()
		return
	}

	// Canale per ricevere l'esito dell'elezione
	electionResult := make(chan bool)

	// Avvia una goroutine per controllare lo stato della variabile election
	// durante l'attesa del timer
	go func() {
		for {
			// Se la variabile election diventa false, invia false sul canale
			if !election {
				electionResult <- false
				return
			}
			// Attendi un breve intervallo prima di controllare nuovamente
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Attendi l'esito dell'elezione per un certo periodo
	select {
	case <-electionResult:
		// Se la variabile election diventa false durante l'attesa del timer,
		// termina l'elezione
		fmt.Println("Terminata l'elezione, non posso diventare il leader.")
		return
	case <-time.After(5 * time.Second):
		// Il timer è scaduto, avvia la procedura per diventare il leader
		fmt.Println("Sono il nuovo leader, invio il messaggio COORDINATOR.")
		// Invia messaggio COORDINATOR a tutti i nodi nella rete
		leaderID = n.ID
		leaderAddress = fmt.Sprintf("%s:%d", n.IPAddress, n.Port)
		n.sendCoordinatorMessages()
	}
	return
}

// Invoca procedura remota ELECTION sul nodo inserito come parametro
// @return: true se posso continuare elezione, false per interrompere processo di elezione
func (n *Node) sendElectionMessage(node Node) bool {
	// Effettua una chiamata RPC al nodo specificato per avviare un processo di elezione
	client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", node.IPAddress, node.Port))
	if err != nil {
		fmt.Println("Errore nella chiamata RPC ELECTION sul nodo, non è raggiungibile", node.IPAddress, ":", node.Port)
		return true
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	var reply = true
	err = client.Call("Node.ELECTION", n.ID, &reply)
	if err != nil {
		fmt.Println("Errore durante la chiamata RPC per avviare l'elezione sul nodo, non è raggiungibile", node.ID)
		return true
	}

	// Utilizzo per il momento il valore di ritorno della procedura ELECTION
	if !reply {
		// Il nodo con ID superiore mi ha bloccato, abbandono il proposito di diventare leader
		return false
	}
	return true
}

// Invoca la procedura remota COORDINATOR su tutti i nodi nella nodeList per notificare il nuovo Leader
func (n *Node) sendCoordinatorMessages() {
	// Itera su tutti i nodi nella nodeList
	for _, node := range nodeList {
		// Se il nodo è diverso da se stesso
		if node.ID != n.ID {
			// Avvia una goroutine per invocare la procedura remota COORDINATOR su questo nodo
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
						return
					}
				}(client)

				// Variabile per memorizzare la risposta remota
				var reply bool

				// Effettua la chiamata RPC
				err = client.Call("Node.COORDINATOR", *n, &reply)
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

func emulateCrash() {
	//TODO
	// Attualmente non può avvenire crash, le prove verranno effettuate da terminale
}

// Stampa dei nodi nella rete
func printNodeList() {
	fmt.Print("I nodi sulla rete sono:")
	for _, node := range nodeList {
		fmt.Printf("\nID: %d, IP: %s, Port: %d", node.ID, node.IPAddress, node.Port)
	}
}
