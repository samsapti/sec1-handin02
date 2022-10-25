package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"time"

	pb "github.com/samsapti/sec1-handin-02/grpc"
	"github.com/samsapti/sec1-handin-02/pedersen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const rounds int = 3

var (
	commChan    chan *pb.Commitment      = make(chan *pb.Commitment, 1)
	openingChan chan *pb.Opening         = make(chan *pb.Opening, 1)
	throwChan   chan *pb.DieThrow        = make(chan *pb.DieThrow, 1)
	ackChan     chan *pb.Acknowledgement = make(chan *pb.Acknowledgement, 1)

	name     *string = flag.String("name", "Alice", "Name of the player")
	ownAddr  *string = flag.String("addr", "localhost:50051", "gRPC listen address. Format: [host]:port")
	peerAddr *string = flag.String("peer_addr", "localhost:50052", "Peer's gRPC listen address. Format: [host]:port")
)

type server struct {
	pb.UnimplementedDiceGameServer
}

func (s *server) SendCommitment(ctx context.Context, in *pb.Commitment) (*pb.DieThrow, error) {
	commChan <- in
	return <-throwChan, nil
}

func (s *server) SendOpening(ctx context.Context, in *pb.Opening) (*pb.Acknowledgement, error) {
	openingChan <- in
	return <-ackChan, nil
}

func getTLSConfig() *tls.Config {
	certPool := x509.NewCertPool()
	certs := []tls.Certificate{}

	for _, v := range []string{"alice", "bob"} {
		// Read certificate files
		clientPemBytes, err := os.ReadFile(fmt.Sprintf("certs/%s.cert.pem", v))
		if err != nil {
			log.Fatalf("%s\n", err)
		}

		// Decode and parse certs
		clientPemBlock, _ := pem.Decode(clientPemBytes)
		clientCert, err := x509.ParseCertificate(clientPemBlock.Bytes)
		if err != nil {
			log.Fatalf("%s\n", err)
		}

		// Enforce client authentication and allow self-signed certs
		clientCert.BasicConstraintsValid = true
		clientCert.IsCA = true
		clientCert.KeyUsage = x509.KeyUsageCertSign

		// Add certs as client certs
		certPool.AppendCertsFromPEM(clientPemBytes)

		// Load certs as server certs
		srvCert, err := tls.LoadX509KeyPair(fmt.Sprintf("certs/%s.cert.pem", v), fmt.Sprintf("certs/%s.key.pem", v))
		if err != nil {
			log.Fatalf("%s\n", err)
		}
		certs = append(certs, srvCert)
	}

	return &tls.Config{
		Certificates: certs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		RootCAs:      certPool,
	}
}

func initPeer(client *pb.DiceGameClient) {
	// Setup TLS tunnel
	tlsCreds := credentials.NewTLS(getTLSConfig())

	// Instantiate client
	conn, err := grpc.Dial(*peerAddr, grpc.WithTransportCredentials(tlsCreds))
	if err != nil {
		log.Fatalf("Cannot start connection: %s\n", err)
	}
	*client = pb.NewDiceGameClient(conn)

	// Server connection info
	srv := grpc.NewServer(grpc.Creds(tlsCreds))
	pb.RegisterDiceGameServer(srv, &server{})

	// Initialize listener
	lis, err := net.Listen("tcp", *ownAddr)
	if err != nil {
		log.Fatalf("%s failed to listen: %s\n", *name, err)
	}

	// Server
	log.Printf("%s listening at %s\n", *name, lis.Addr())
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("%s failed to serve: %s\n", *name, err)
	}
}

func main() {
	// Prepare
	flag.Parse()
	starts := false
	ctx := context.Background()

	if *name == "Alice" {
		starts = true
	}

	// Setup connection
	var client pb.DiceGameClient
	go initPeer(&client)

	// Wait for peer to come online
	time.Sleep(2 * time.Second)

	/*
		Game loop starts here
	*/

	for i := 0; i < rounds; i++ {
		log.Printf("Round %d\n", i+1)

		if starts {
			log.Printf("%s starts this round", *name)
		}

		throw, err := rand.Int(rand.Reader, big.NewInt(5))
		if err != nil {
			log.Fatalf("Error: %s\n", err)
		}

		m := throw.Uint64() + 1

		if starts {
			// Create commitment
			r := pedersen.GetR()
			c := pedersen.GetCommitment(m, r)

			// Send commitment to peer and wait for die throw
			log.Printf("%s sends commitment: %d\n", *name, c)
			peerThrow, err := client.SendCommitment(ctx, &pb.Commitment{C: c})
			if err != nil {
				log.Fatalf("Error: %s\n", err)
			}
			log.Printf("%s receives die throw: %d\n", *name, peerThrow.Val)

			// Send opening to peer
			log.Printf("%s sends opening: (m: %d, r: %d)\n", *name, m, r)
			peerAck, err := client.SendOpening(ctx, &pb.Opening{M: m, R: r})
			if err != nil {
				log.Fatalf("Error: %s\n", err)
			}
			log.Printf("%s receives acknowledgement: %t\n", *name, peerAck.Ack)

			// Check peer's acknowledgement
			if !peerAck.Ack {
				log.Printf("%s got caught cheating, run!\n", *name)
				time.Sleep(time.Second)
				os.Exit(1)
			}

			// Check winner
			if m > peerThrow.Val {
				log.Printf("%s won this round!\n", *name)
			} else if m < peerThrow.Val {
				log.Printf("%s lost this round!\n", *name)
			} else {
				log.Println("It's a tie!")
			}
		} else {
			// Wait for commitment from peer
			commitment := <-commChan
			log.Printf("%s receives commitment: %d\n", *name, commitment.C)

			throwChan <- &pb.DieThrow{Val: m}
			log.Printf("%s sends die throw: %d", *name, m)

			opening := <-openingChan
			log.Printf("%s receives opening: (m: %d, r: %d)\n", *name, opening.M, opening.R)

			// Validate commitment from peer
			if pedersen.ValidateCommitment(commitment.C, opening.M, opening.R) {
				ackChan <- &pb.Acknowledgement{Ack: true}
				log.Printf("%s confirms commitment is valid\n", *name)
			} else {
				ackChan <- &pb.Acknowledgement{Ack: false}
				log.Printf("%s's opponent is cheating!\n", *name)
				time.Sleep(time.Second)
				os.Exit(1)
			}

			// Check winner
			if m > opening.M {
				log.Printf("%s won this round!\n", *name)
			} else if m < opening.M {
				log.Printf("%s lost this round!\n", *name)
			} else {
				log.Println("It's a tie!")
			}
		}

		// Switch turns
		starts = !starts
	}

	time.Sleep(2 * time.Second)
}
