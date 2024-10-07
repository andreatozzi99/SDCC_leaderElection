
# Comandi Docker

### 1. **Costruire e lanciare i container con Docker Compose**
Costruisce le immagini (se necessario) e avvia tutti i container in background.

```bash
docker-compose up -d --build
```

### 2. **Lista dei container in esecuzione**
Verifica quali container sono in esecuzione e ottieni i loro nomi.

```bash
docker ps
```

### 3. **Visualizzare i log di un singolo container replicato**
Visualizza i log di un container specifico.

```bash
docker logs -f <nome_container>
```
Per impostare un massimo di righe da visualizzare.

```bash
docker logs -f --tail <numero di righe> <nome_container>
```

### 4. **Visualizzare i log di un intero servizio**
Mostra i log di tutti i container di un servizio specifico.

```bash
docker-compose logs <nome_servizio>
```

### 5. **Entrare in un container**
Accedi al terminale di un container in esecuzione.

```bash
docker exec -it <nome_container> /bin/sh
```

### 6. **Controllare i processi in esecuzione**
Mostra i processi in esecuzione nel container.

```bash
ps aux
```

### 7. **Verificare l'accesso alla rete**
Controlla se il container ha accesso a internet.

```bash
ping google.com
```

### 8.1 **Interrompere un singolo container**
Ferma un container specifico.

```bash
docker stop <nome_container>
```

### 8.2 **Interrompere il gruppo compose**
Ferma tutti i container in esecuzione.

```bash
docker-compose stop
```

### 9. **Riavviare un container**
Riavvia un container specifico.

```bash
docker restart <nome_container>
```

### 10. **Eliminare un container (se necessario)**
Elimina un container fermato e libera risorse.

```bash
docker rm -f <nome_container>
```

### 11. **Esci dal container**
Quando hai finito di interagire con il container, puoi uscire.

```bash
exit
```

### 12. **Arresta l'istanza utilizzando il comando AWS CLI:**
```bash
aws ec2 stop-instances --instance-ids <instance_id>
```
