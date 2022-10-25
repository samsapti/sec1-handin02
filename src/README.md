# How to run

## Manually

First, generate the certificates:

```sh
cd certs
bash gen_certs.sh
cd ..
```

Then, in one terminal, run the following command to start Alice:

```sh
go run . -addr "localhost:50051" -peer_addr "localhost:50052" -name "Alice"
```

In another terminal, run the following command to start Bob:

```sh
go run . -addr "localhost:50052" -peer_addr "localhost:50051" -name "Bob"
```

## With Docker

Running the `run.sh` script will handle everything. Commandline arguments are
supported and will be forwarded to the `docker-compose up` command (e.g.
`--build`).

To run it, issue the following command:

```sh
bash run.sh
```
