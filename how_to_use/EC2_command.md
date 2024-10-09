### Esecuzione con istanze EC2

#### 1. Installare Docker

Aggiorna i pacchetti e installa Docker.

```bash
sudo yum update -y
sudo yum install -y docker
```

#### 2. Installare Docker Compose e Avviare il Docker Daemon

Scarica e installa Docker Compose.

```bash
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
sudo service docker start
```

#### 3. Installare Git e clonare il repository

Installa Git e clona il repository.

```bash
sudo yum install git -y
git clone https://github.com/andreatozzi99/SDCC_leaderElection
```

#### 4. Eseguire Docker Compose per avviare il progetto in detached mode

Naviga nella directory del progetto e avvia Docker Compose.

```bash
cd SDCC_leaderElection
sudo docker-compose up -d --build
```
```