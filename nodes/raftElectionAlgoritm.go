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

var (
	electionTimer *time.Timer // Timer per il conteggio dell'elezione
	randGen       = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// ---------------- Strutture: Argomenti e valori di ritorno delle Chiamate RPC ------------------

// HeartBeatArgs contiene gli argomenti per il messaggio di HEARTBEAT inviato dal Leader verso i follower
type HeartBeatArgs struct {
	Term     int // Termine corrente del leader IMPORTANTE
	LeaderID int // ID del leader [DA 0 A N-1] con N = numero nodi nella rete
}

// HeartBeatReply contiene la risposta al messaggio di HEARTBEAT
type HeartBeatReply struct {
	Term    int  // Termine corrente del nodo ricevente
	Success bool // True se l'HeartBeat è stato accettato, false altrimenti
}

// RequestVoteArgs contiene gli argomenti per la richiesta di voto
type RequestVoteArgs struct {
	Node RaftNode
}

// RequestVoteReply contiene la risposta alla richiesta di voto
type RequestVoteReply struct {
	Term        int  // Termine corrente del nodo ricevente
	VoteGranted bool // True se il voto è stato concesso, false altrimenti
}

// RaftNode ----------------------------
type RaftNode struct {
	ID        int
	IPAddress string
	Port      int
	//------------- Aggiunte rispetto un classico NodeBully
	CurrentTerm int
	VotedFor    int
}

// --------------------- Metodi Esposti per RPC (Messaggi scambiati tra nodi) ----------------------------

// REQUESTVOTE gestisce la ricezione di un messaggio di Richiesta voto da parte di un altro nodo
func (n *RaftNode) REQUESTVOTE(args RequestVoteArgs, reply *RequestVoteReply) error {
	// Ho ricevuto una richiesta di voto da un altro nodo -> Il leader è in crash oppure è un nuovo nodo
	// Aggiungo il nodo alla lista dei nodi conosciuti
	senderNode := NodeBully{
		ID:        args.Node.ID,
		IPAddress: args.Node.IPAddress,
		Port:      args.Node.Port,
	}
	go addNodeInNodeList(senderNode) // Aggiungi il nodo alla lista dei nodi conosciuti
	fmt.Printf("Node %d (T:%d) <-- REQUESTVOTE from Node %d (T:%d)\n", n.ID, n.CurrentTerm, args.Node.ID, args.Node.CurrentTerm)
	// Se il mandato del mittente è maggiore del mandato corrente
	if args.Node.CurrentTerm > n.CurrentTerm {
		n.becomeFollower(args.Node.CurrentTerm) // Aggiorna il mandato corrente e diventa follower
	} else {
		if args.Node.CurrentTerm == n.CurrentTerm { // Se abbiamo lo stesso mandato, devo controllare se ho già votato
			if n.VotedFor != -1 {
				// Se ha già votato per un altro candidato, rifiuta la richiesta di voto
				*reply = RequestVoteReply{
					Term:        n.CurrentTerm,
					VoteGranted: false, // Non concede il voto
				}
				fmt.Println("Voted: No")
				return nil
			}
			// Se abbiamo stesso mandato e non ho votato, concedo il voto
		} else {
			// Se il mittente ha un mandato minore
			*reply = RequestVoteReply{
				Term:        n.CurrentTerm,
				VoteGranted: true, // Non concede il voto
			}
			fmt.Println("Voted: Yes")
			return nil
		}
	}
	n.VotedFor = args.Node.ID
	*reply = RequestVoteReply{
		Term:        n.CurrentTerm,
		VoteGranted: true, // Concede il voto
	}
	fmt.Println("Voted: Yes")
	return nil
}

// HEARTBEAT gestisce la ricezione di un messaggio di HeartBeat da parte di un altro nodo
func (n *RaftNode) HEARTBEAT(args HeartBeatArgs, reply *HeartBeatReply) error {
	fmt.Printf("Node %d (T:%d) <-- HEARTBEAT from Leader %d (T:%d)\n", n.ID, n.CurrentTerm, args.LeaderID, args.Term)
	n.resetElectionTimer() // Resetta il timer di elezione
	// Se il termine del messaggio è maggiore del termine corrente. Oppure se il termine è uguale ma l'ID del mittente è maggiore
	if hbState && args.Term > n.CurrentTerm {
		// Se sono il leader, ma ricevo un heartbeat da un nodo(Leader) con ID maggiore, divento follower
		fmt.Println("C'è un altro leader con Mandato maggiore, devo dimettermi dalla carica di leader")
		n.becomeFollower(args.Term)
		leaderID = args.LeaderID
	}
	if args.Term > n.CurrentTerm {
		leaderID = args.LeaderID
		n.CurrentTerm = args.Term
		//n.becomeFollower(args.Term) // Diventa follower e aggiorna il termine corrente
		*reply = HeartBeatReply{
			Term:    n.CurrentTerm,
			Success: true, // Accetta l'HeartBeat
		}
	} else {
		if args.Term == n.CurrentTerm {
			// Ignora il messaggio se il termine del messaggio è uguale al termine corrente
			*reply = HeartBeatReply{
				Term:    n.CurrentTerm,
				Success: true, // Rifiuta l'HeartBeat (Valore non utilizzato dal leader)
			}
		} else {
			// Il leader che invia HeartBeat ha un mandato inferiore al locale
			fmt.Println("################ Comportamento non previsto in rete fully connected ################")
			*reply = HeartBeatReply{
				Term:    n.CurrentTerm,
				Success: false, // Rifiuta l'HeartBeat (Valore non utilizzato dal leader)
			}
		}
	}
	return nil
}

//--------------------------------------------------------------------------------------------------------

// Metodo per avviare il nodo e registrarne l'indirizzo nel server di registrazione
// e recuperare la lista dei nodi in rete
func (n *RaftNode) start() {
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

	// ------------- Esposizione dei metodi per le chiamate RPC -------------
	err := rpc.Register(n)
	if err != nil {
		fmt.Println("Errore durante la registrazione dei metodi RPC:", err)
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
			fmt.Println("Errore nella chiusura della connessione con il NodeRegistry server:", err)
			return
		}
	}(client)
	// ---------- RPC per Registrare il nodo --------------
	var reply int
	// ? Devo creare una struttura node, impostare i parametri a partire da quelli di n, e inviare quelli al NodeRegistry
	node := &NodeBully{
		ID:        n.ID, // Per il nodeRegistry, che assegnerà un nuovo ID
		IPAddress: n.IPAddress,
		Port:      n.Port,
	}
	err = client.Call("NodeRegistry.RegisterNode", node, &reply)
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
	// ############# Salva lo stato nel file di log ###############
	if runInContainer {
		if err := saveState(n.ID, n.IPAddress, n.Port); err != nil {
			fmt.Println("Errore durante il salvataggio dello stato:", err)
			return
		}
	}
	// Visualizza l'ID e la porta assegnati al nodo
	fmt.Printf("-------- Registrato sulla rete con ID:%d, Porta:%d --------\n", n.ID, n.Port)

	// ---------- RPC per ricevere la lista dei nodi nella rete  ----------
	err = client.Call("NodeRegistry.GetRegisteredNodes", n, &nodeList)
	if err != nil {
		fmt.Println("Errore durante la richiesta della lista dei nodi:", err)
		return
	}
	// ------------ Divento un candidato, per scoprire la situazione della rete, non conosco il term attuale nella rete,
	// Gli altri nodi non sanno della mia esistenza, devo almeno contattarli ------------------------------------------
	go n.becomeCandidate()
	return
}

// Funzione InizioElezione(): Avvia un'elezione se non ricevo comunicazioni dal leader per un tempo superiore al timeout
func (n *RaftNode) startRaftElection() {
	fmt.Printf("Node %d: Started RAFT-election\n", n.ID)
	electionMutex.Lock()
	if election {
		electionMutex.Unlock()
		return // Se sono già in corso elezioni, termina
	}
	// --------------- Avvia elezione ----------------
	election = true
	electionMutex.Unlock()
	// Incrementa il termine corrente e vota per se stesso
	n.CurrentTerm++
	n.VotedFor = n.ID        // TODO Impostare questo valore è giusto? Posso votare solo per me o concedere un altro voto?
	votesReceived := 1       // Contatore per i voti ricevuti (contiene già il voto di se stesso)
	successfulResponses := 0 // Contatore per le risposte ricevute senza errori (nodi non in crash)
	// --------- Invia richieste di voto agli altri nodi --------------
	var wg sync.WaitGroup
	results := make(chan error, len(nodeList)) // Canale per gli errori delle goroutine

	for _, node := range nodeList {
		if node.ID != n.ID {
			wg.Add(1) // Aumenta il contatore per una nuova goroutine
			go func(node NodeBully) {
				defer wg.Done() // Quando la goroutine termina, decrementa il contatore
				err := n.sendRequestVoteMessage(node, &votesReceived)
				results <- err // Invia il risultato (errore o nil) nel canale
			}(node)
		}
	}
	time.Sleep(maxRttTime) // Attesa per ricevere le risposte dai nodi
	// Goroutine che chiuderà il canale solo dopo che tutte le altre sono terminate
	go func() {
		wg.Wait()      // Aspetta che tutte le goroutine abbiano invocato wg.Done()
		close(results) // Chiude il canale dopo che tutte le goroutine hanno terminato
	}()
	// Raccogli i risultati dal canale
	for err := range results {
		if err == nil {
			successfulResponses++ // Conta solo le risposte senza errori
		}
	}
	electionMutex.Lock()
	if election == false { // Elezione interrotta durante il periodo di timeout
		electionMutex.Unlock()
		return // Sono diventato follower in seguito all'invio di uno dei messaggi RequestVote
	}
	fmt.Println("Node: Nodi attivi nella rete:", successfulResponses+1, "Voti ricevuti:", votesReceived)
	// Verifica se la maggioranza delle risposte ricevute (senza errori) ha votato a favore
	if votesReceived < (successfulResponses+1)/2 { // "+1" include il voto per se stesso
		election = false // Se non ha ricevuto la maggioranza dei voti, avvia una nuova elezione
		electionMutex.Unlock()
		n.startRaftElection()
		return
	}
	// ------- Diventa Leader ----------
	electionMutex.Unlock()
	n.becomeLeader()
}

// HeartBeatRoutine Funzione: Invia un messaggio di HeartBeat agli altri nodi
func (n *RaftNode) HeartBeatRoutine() {
	fmt.Printf("Leader %d | Start Heart-Beat Routine\n", n.ID)
	for {
		if !hbState {
			return
		}
		fmt.Printf("Leader %d | Contacting all followers...\n", n.ID)
		for _, node := range nodeList {
			if node.ID != n.ID {
				// Invia il messaggio di HeartBeat in una goroutine
				go n.sendHeartBeatMessage(node)
			}
		}
		time.Sleep(heartbeatTime) // Attesa tra un messaggio di HeartBeat e l'altro
	}
}

func (n *RaftNode) sendHeartBeatMessage(node NodeBully) {
	// Connessione al nodo remoto
	client, err := rpc.Dial("tcp", node.IPAddress+":"+strconv.Itoa(node.Port))
	if err != nil {
		//fmt.Printf("Errore durante la connessione al nodo %d: %v\n", node.ID, err)
		return
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			// Gestione dell'errore nella chiusura del client
		}
	}(client)
	// Effettua la chiamata RPC per inviare l'HeartBeat
	args := HeartBeatArgs{
		Term:     n.CurrentTerm,
		LeaderID: n.ID,
	}
	var reply HeartBeatReply
	err = client.Call("RaftNode.HEARTBEAT", args, &reply)
	if err != nil {
		fmt.Printf("Errore durante l'invio di HeartBeat al nodo %d:\n", node.ID)
		return
	}
	// Aggiorna il termine corrente del nodo in base alla risposta ricevuta
	if reply.Term > n.CurrentTerm {
		// Se il termine nella risposta è maggiore del termine corrente,
		// aggiorniamo il termine corrente del nodo, ma non diventiamo follower
		n.CurrentTerm = reply.Term
	}
}

func (n *RaftNode) sendRequestVoteMessage(node NodeBully, votesReceived *int) error {
	// ---------------- Connessione al nodo remoto -----------------
	client, err := rpc.Dial("tcp", node.IPAddress+":"+strconv.Itoa(node.Port))
	if err != nil {
		fmt.Printf("Errore durante la connessione al nodo %d\n", node.ID)
		return err
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
		}
	}(client)
	// ------------------- Effettua la chiamata RPC per richiedere il voto -----------------
	args := RequestVoteArgs{
		Node: *n,
	}
	var reply RequestVoteReply
	err = client.Call("RaftNode.REQUESTVOTE", args, &reply)
	if err != nil {
		fmt.Printf("Errore durante la richiesta di voto al nodo %d\n", node.ID)
		return err
	}
	// Aggiorna il termine corrente del nodo in base alla risposta ricevuta
	if reply.Term > n.CurrentTerm {
		n.becomeFollower(reply.Term) // Aggiorno term
		//return nil
	}
	// Controlla se il voto è stato concesso
	if reply.VoteGranted {
		// Incrementa il conteggio dei voti ricevuti
		*votesReceived++
	}
	return nil
}

// Funzione becomeFollower(): Avvia la routine di un nodo follower: Interrompi l'elezione e aggiorna mandato
func (n *RaftNode) becomeFollower(term int) {
	fmt.Printf("Node %d (T:%d) I'm a follower \n", n.ID, n.CurrentTerm)
	hbState = false
	electionMutex.Lock()
	election = false
	electionMutex.Unlock()
	n.CurrentTerm = term
	n.VotedFor = -1
}

// Funzione becomeLeader(): Avvia la routine di un nodo leader
func (n *RaftNode) becomeLeader() {
	fmt.Printf("Node %d (T:%d) I'm LEADER \n", n.ID, n.CurrentTerm)
	electionMutex.Lock()
	election = false
	electionMutex.Unlock()
	n.VotedFor = -1
	hbState = true
	// Inizia a inviare messaggi di HeartBeat agli altri nodi
	go n.HeartBeatRoutine()
}

// Funzione becomeCandidate(): Avvia la routine di un nodo candidato
func (n *RaftNode) becomeCandidate() {
	fmt.Printf("Node %d I'm a Candidate \n", n.ID)
	// Avvia un'elezione
	n.startRaftElection()
}

func (n *RaftNode) resetElectionTimer() {
	// Resetta il timer di elezione se esiste già uno attivo
	if electionTimer != nil {
		electionTimer.Stop()
	}

	// Genera un numero casuale compreso tra electionTimerMin e electionTimerMax
	randomDuration := electionTimerMin + time.Duration(randGen.Int63n(int64(electionTimerMax-electionTimerMin)))

	// Crea un nuovo timer con la durata casuale generata
	electionTimer = time.NewTimer(randomDuration)

	// Avvia una goroutine per attendere il timer e avviare un'elezione quando scade
	go func() {
		<-electionTimer.C // Attende che il timer scada
		// Avvia un'elezione se il timer scade
		// Evitarlo se il nodo è già leader
		if !hbState {
			fmt.Println("################## Election timer expired ##################")
			n.startRaftElection()
		}
	}()

	// Stampa il tempo rimanente prima che scada il timer (per scopi di debug)
	fmt.Printf("Election timer set to %v\n", randomDuration)
}
