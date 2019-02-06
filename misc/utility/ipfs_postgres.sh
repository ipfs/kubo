#!/bin/sh

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
  then
    echo "Env variables not provided"
    echo "Usage: ./ipfs_postgres.sh <IPFS_PGHOST> <IPFS_PGUSER> <IPFS_PGDATABASE>"
    exit 1
fi

export IPFS_PGHOST=$1
export IPFS_PGUSER=$2
export IPFS_PGDATABASE=$3
printf "${IPFS_PGUSER}@${IPFS_PGHOST} password:"
stty -echo
read IPFS_PGPASSWORD
stty echo
export IPFS_PGPASSWORD

ipfs init --profile=postgresds
