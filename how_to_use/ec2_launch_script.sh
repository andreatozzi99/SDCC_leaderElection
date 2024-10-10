#!/bin/bash
sudo yum update -y
sudo yum install -y docker
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
sudo service docker start
sudo yum install git -y
git clone https://github.com/andreatozzi99/SDCC_leaderElection
cd SDCC_leaderElection