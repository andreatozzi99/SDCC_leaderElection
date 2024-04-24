package main

import (
	"fmt"
	"net"
	"net/rpc"
	"strconv"
	"time"
)

// --------------- Descrizione Algoritmo --------------------
/* Quando è necessario eleggere un nuovo Leader, nel cluster viene avviato un nuovo mandato(Term).
// Un mandato è un periodo di tempo arbitrario sul nodo per il quale deve essere eletto un nuovo leader.
// Ogni mandato inizia con l'elezione di un leader.
// Se l'elezione viene completata con successo (cioè viene eletto un singolo leader), il mandato continua con le normali operazioni orchestrate dal nuovo leader.
// Se l'elezione è un fallimento, inizia un nuovo mandato, con una nuova elezione.
// L'elezione di un leader viene avviata da un nodo, nello stato "Candidato"
// Un nodo diventa un candidato se non riceve alcuna comunicazione dal leader per un periodo chiamato timeout elettorale,
// quindi presume che non ci sia più un leader in carica.
// Inizia l'elezione aumentando il contatore dei termini, votando per se stesso come nuovo leader e inviando un messaggio
// a tutti gli altri nodo richiedendo il loro voto. Un nodo voterà solo una volta per mandato, in base all'ordine di arrivo.
// Se un candidato riceve un messaggio da un altro nodo con un numero di mandato superiore al mandato corrente del candidato,
// l'elezione del candidato viene sconfitta e il candidato si trasforma in un follower e riconosce il leader come legittimo.
// Se un candidato riceve la maggioranza dei voti, diventa il nuovo leader.
// Se nessuna delle due cose accade, ad esempio a causa di un voto disgiunto, inizia un nuovo mandato e inizia una nuova elezione.
//
// Raft utilizza un timeout elettorale casuale per garantire che i problemi di voto diviso vengano risolti rapidamente.
// Questo dovrebbe ridurre la possibilità di un voto disgiunto perché i nodi non diventeranno candidati allo stesso tempo:
// un singolo nodo andrà in timeout, vincerà le elezioni,
// quindi diventerà leader e invierà messaggi heartbeat ad altri nodo prima che uno qualsiasi dei follower possa diventare candidato.
*/
// ----------------- Tempistiche -----------------------------
/* Tempistica e disponibilità
Il tempismo è fondamentale in Raft per eleggere e mantenere un leader stabile nel tempo,
al fine di avere una perfetta disponibilità del cluster.
La stabilità è garantita dal rispetto dei requisiti di temporizzazione dell'algoritmo:
	broadcastTime << electionTimeout << MTBF
broadcastTime è il tempo medio impiegato da un nodo per inviare una richiesta a tutti i nodi del cluster e ricevere le risposte.
È relativo all'infrastruttura utilizzata.
MTBF (Mean Time Between Failures) è il tempo medio tra i guasti per un nodo. È anche relativo all'infrastruttura.
electionTimeout è lo stesso descritto nella sezione Elezione del leader. È qualcosa che il programmatore deve scegliere.
I numeri tipici per questi valori possono essere compresi tra 0,5 ms e 20 ms per broadcastTime,
il che implica che il programmatore imposta electionTimeout tra 10 ms e 500 ms.
Possono essere necessarie diverse settimane o mesi tra un errore di un singolo nodo,
il che significa che i valori sono sufficienti per un cluster stabile.
*/

// ---------------- Strutture per Argomenti e valori di ritorno per Chiamate RPC ------------------

// HeartBeatArgs contiene gli argomenti per il messaggio di ?
type HeartBeatArgs struct {
	Term     int // Termine corrente del leader IMPORTANTE
	LeaderID int // ID del leader [DA 0 A N-1] con N = numero nodi nella rete
}

// HeartBeatReply contiene la risposta al messaggio di ?
type HeartBeatReply struct {
	Term    int  // Termine corrente del nodo ricevente
	Success bool // True se l'HeartBeat è stato accettato, false altrimenti
}

// RequestVoteArgs contiene gli argomenti per la richiesta di voto
type RequestVoteArgs struct {
	node Node
	Term int // Termine corrente del candidato
}

// RequestVoteReply contiene la risposta alla richiesta di voto
type RequestVoteReply struct {
	Term        int  // Termine corrente del nodo ricevente
	VoteGranted bool // True se il voto è stato concesso, false altrimenti
}

// RaftNode ----------------------------
type RaftNode struct {
	ID          int
	IPAddress   string
	Port        int
	CurrentTerm int
	VotedFor    int
}

// --------------------- Metodi Esposti per RPC (Messaggi scambiati tra nodi) ----------------------------

// REQUESTVOTE gestisce la ricezione di un messaggio di Richiesta voto da parte di un altro nodo
func (n *RaftNode) REQUESTVOTE(args RequestVoteArgs, reply *RequestVoteReply) error {
	// Aggiungo il nodo alla lista dei nodi conosciuti
	go addNodeInNodeList(args.node)
	fmt.Printf("Node %d <-- REQUESTVOTE Node %d\n", n.ID, args.node.ID)
	// Se il termine del messaggio è maggiore del termine corrente del nodo locale
	if args.Term > n.CurrentTerm {
		// Aggiorna il termine corrente e diventa follower
		n.becomeFollower(args.Term)
		// Verifica se il nodo locale ha già votato per un candidato in un termine precedente
		if n.VotedFor != -1 {
			// Se ha già votato per un altro candidato, rifiuta la richiesta di voto
			*reply = RequestVoteReply{
				Term:        n.CurrentTerm,
				VoteGranted: false, // Non concede il voto
			}
			return nil
		}
		// Se non ha votato oppure ha già votato per il mittente, concede il voto
		n.VotedFor = args.node.ID
		*reply = RequestVoteReply{
			Term:        n.CurrentTerm,
			VoteGranted: true, // Concede il voto
		}
	} else {
		if n.VotedFor != -1 {
			// Se ha già votato per un altro candidato, rifiuta la richiesta di voto
			*reply = RequestVoteReply{
				Term:        n.CurrentTerm,
				VoteGranted: false, // Non concede il voto
			}
			return nil
		}
		// Se non ha votato oppure ha già votato per il mittente, concede il voto
		n.VotedFor = args.node.ID
		*reply = RequestVoteReply{
			Term:        n.CurrentTerm,
			VoteGranted: true, // Concede il voto
		}
		n.becomeFollower(args.Term)
	}
	return nil
}

// HEARTBEAT gestisce la ricezione di un messaggio di HeartBeat da parte di un altro nodo
func (n *RaftNode) HEARTBEAT(args HeartBeatArgs, reply *HeartBeatReply) error {
	fmt.Printf("Node %d <-- HEARTBEAT Leader %d\n", n.ID, args.LeaderID)
	electionMutex.Lock()
	defer electionMutex.Unlock()
	resetElectionTimer() // Resetta il timer di elezione
	// Se il termine del messaggio è maggiore del termine corrente
	if args.Term > n.CurrentTerm {
		n.becomeFollower(args.Term) // Diventa follower e aggiorna il termine corrente
		*reply = HeartBeatReply{
			Term:    n.CurrentTerm,
			Success: true, // Accetta l'HeartBeat
		}
	} else {
		// Ignora il messaggio se il termine del messaggio è minore o uguale al termine corrente
		*reply = HeartBeatReply{
			Term:    n.CurrentTerm,
			Success: false, // Rifiuta l'HeartBeat
		}
	}
	return nil
}

// Metodo per avviare il nodo e registrarne l'indirizzo nel server di registrazione
// e recuperare la lista dei nodi in rete
func (n *RaftNode) start() {
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
	node := &Node{
		ID:        -1, // Per il nodeRegistry, che assegnerà un nuovo ID
		IPAddress: localAddress,
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
	// Visualizza l'ID e la porta assegnati al nodo
	fmt.Printf("-------- Registrato sulla rete. ID %d: Porta  %d --------\n", n.ID, n.Port)

	// ---------- RPC per ricevere la lista dei nodi nella rete  ----------
	err = client.Call("NodeRegistry.GetRegisteredNodes", n, &nodeList)
	if err != nil {
		fmt.Println("Errore durante la richiesta della lista dei nodi:", err)
		return
	}
	// ------------ Divento un candidato, per scoprire la situazione della rete ----------------
	go n.becomeCandidate()
	return
}

// Funzione InizioElezione(): Avvia un'elezione se non ricevo comunicazioni dal leader per un tempo superiore al timeout
func (n *RaftNode) startRaftElection() {
	fmt.Printf("Nodo %d Started RAFT-election\n", n.ID)
	electionMutex.Lock()
	if election {
		electionMutex.Unlock()
		return // Se sono già in corso elezioni, termina
	}
	electionMutex.Unlock()
	// --------------- Avvia elezione ----------------
	election = true

	// Incrementa il termine corrente e vota per se stesso
	n.CurrentTerm++
	n.VotedFor = n.ID

	// Contatore per i voti ricevuti
	votesReceived := 1 // Voto per me stesso

	// --------- Invia richieste di voto agli altri nodi --------------
	for _, node := range nodeList {
		if node.ID != n.ID {
			// Invia la richiesta di voto in una goroutine
			go n.sendRequestVoteMessage(node, &votesReceived)
		}
	}

	// Avvia un timeout di attesa per le risposte
	time.Sleep(electionTimeout)
	electionMutex.Lock()
	if election == false { // Elezione interrotta durante il periodo di timeout

	}
	if votesReceived < len(nodeList)/2 {
		election = false // Se non ha ricevuto la maggioranza dei voti, avvia una nuova elezione
		n.startRaftElection()
		return
	}
	// ------- Diventa Leader ----------
	n.becomeLeader()
}

// Funzione InviaHeartBeat(): Invia un messaggio di HeartBeat agli altri nodi
func (n *RaftNode) sendHeartBeatRoutine() {
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
		time.Sleep(time.Millisecond * 100)
	}
}

func (n *RaftNode) sendHeartBeatMessage(node Node) {
	// Connessione al nodo remoto
	client, err := rpc.Dial("tcp", node.IPAddress+":"+strconv.Itoa(node.Port))
	if err != nil {
		fmt.Printf("Errore durante la connessione al nodo %d: %v\n", node.ID, err)
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
		fmt.Printf("Errore durante l'invio di HeartBeat al nodo %d: %v\n", node.ID, err)
		return
	}
	// Aggiorna il termine corrente del nodo in base alla risposta ricevuta
	if reply.Term > n.CurrentTerm {
		// Se il termine nella risposta è maggiore del termine corrente,
		// aggiorniamo il termine corrente del nodo, ma non diventiamo follower
		n.CurrentTerm = reply.Term
	}
}

func (n *RaftNode) sendRequestVoteMessage(node Node, votesReceived *int) {
	// ---------------- Connessione al nodo remoto -----------------
	client, err := rpc.Dial("tcp", node.IPAddress+":"+strconv.Itoa(node.Port))
	if err != nil {
		fmt.Printf("Errore durante la connessione al nodo %d: %v\n", node.ID, err)
		return
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
		}
	}(client)
	// ------------------- Effettua la chiamata RPC per richiedere il voto -----------------
	senderNode := Node{
		IPAddress: n.IPAddress,
		Port:      n.Port,
		ID:        n.ID,
	}
	args := RequestVoteArgs{
		Term: n.CurrentTerm,
		node: senderNode,
	}
	var reply RequestVoteReply
	err = client.Call("RaftNode.REQUESTVOTE", args, &reply)
	if err != nil {
		fmt.Printf("Errore durante la richiesta di voto al nodo %d: %v\n", node.ID, err)
		return
	}
	// Aggiorna il termine corrente del nodo in base alla risposta ricevuta
	if reply.Term > n.CurrentTerm {
		n.becomeFollower(reply.Term)
	}

	// Controlla se il voto è stato concesso
	if reply.VoteGranted {
		// Incrementa il conteggio dei voti ricevuti
		*votesReceived++
		// Verifica se è stata ricevuta la maggioranza dei voti
		if *votesReceived > len(nodeList)/2 {
			// Diventa il nuovo leader
			n.becomeLeader()
		}
	}
}

// Funzione becomeFollower(): Avvia la routine di un nodo follower
func (n *RaftNode) becomeFollower(term int) {
	fmt.Printf("Node %d I'm a follower \n", n.ID)
	hbState = false
	electionMutex.Lock()
	election = false
	electionMutex.Unlock()
	n.CurrentTerm = term
	n.VotedFor = -1
}

// Funzione becomeLeader(): Avvia la routine di un nodo leader
func (n *RaftNode) becomeLeader() {
	fmt.Printf("Node %d I'm LEADER \n", n.ID)
	electionMutex.Lock()
	election = false
	electionMutex.Unlock()
	n.VotedFor = -1
	// Inizia a inviare messaggi di HeartBeat agli altri nodi
	go n.sendHeartBeatRoutine()
}

// Funzione becomeCandidate(): Avvia la routine di un nodo candidato
func (n *RaftNode) becomeCandidate() {
	fmt.Printf("Node %d I'm a Candidate \n", n.ID)
	// Incrementa il termine corrente e vota per se stesso
	n.CurrentTerm++
	n.VotedFor = n.ID
	// Avvia un'elezione
	n.startRaftElection()
}

// Funzione resetElectionTimer(): Resetta il timer di elezione
func resetElectionTimer() {
	fmt.Printf("Restart Election Timer: DO NOTHING AT THE MOMENT\n")
	// TODO
}
