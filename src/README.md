# How to run

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

Alternatively, use the Docker Compose setup:

```sh
cd certs
bash gen_certs.sh
cd ..
docker-compose up --build
```
