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
	recvCom      *pb.Commitment    = &pb.Commitment{}
	recvComKey   *pb.CommitmentKey = &pb.CommitmentKey{}
	recvDieThrow *pb.DieThrow      = &pb.DieThrow{}
	name         *string           = flag.String("name", "Alice", "Name of the player")
	ownAddr      *string           = flag.String("addr", "localhost:50051", "gRPC listen address. Format: [host]:port")
	peerAddr     *string           = flag.String("peer_addr", "localhost:50052", "Peer's gRPC listen address. Format: [host]:port")
)

type server struct {
	pb.UnimplementedDiceGameServer
}

func (s *server) SendCommitment(ctx context.Context, in *pb.Commitment) (*pb.Empty, error) {
	recvCom = in
	log.Printf("%v receives commitment: %d\n", *name, in.GetC())
	return &pb.Empty{}, nil
}

func (s *server) SendCommitmentKey(ctx context.Context, in *pb.CommitmentKey) (*pb.Empty, error) {
	recvComKey = in
	log.Printf("%v receives commitment key: (m: %d, r: %d)\n", *name, in.GetM(), in.GetR())
	return &pb.Empty{}, nil
}

func (s *server) SendDieThrow(ctx context.Context, in *pb.DieThrow) (*pb.Empty, error) {
	recvDieThrow = in
	log.Printf("%v receives die throw: %d\n", *name, in.GetVal())
	return &pb.Empty{}, nil
}

func getTLSConfig() *tls.Config {
	certPool := x509.NewCertPool()
	certs := []tls.Certificate{}

	for _, v := range []string{"alice", "bob"} {
		// Read certificate files
		srvPemBytes, err := os.ReadFile(fmt.Sprintf("certs/%v.cert.pem", v))
		if err != nil {
			log.Fatalf("%v\n", err)
		}

		// Decode and parse certs
		srvPemBlock, _ := pem.Decode(srvPemBytes)
		clientCert, err := x509.ParseCertificate(srvPemBlock.Bytes)
		if err != nil {
			log.Fatalf("%v\n", err)
		}

		// Enforce client authentication and allow self-signed certs
		clientCert.BasicConstraintsValid = true
		clientCert.IsCA = true
		clientCert.KeyUsage = x509.KeyUsageCertSign
		certPool.AppendCertsFromPEM(srvPemBytes)

		// Load server certificates (essentially the same as the client certs)
		srvCert, err := tls.LoadX509KeyPair(fmt.Sprintf("certs/%v.cert.pem", v), fmt.Sprintf("certs/%v.key.pem", v))
		if err != nil {
			log.Fatalf("%v\n", err)
		}
		certs = append(certs, srvCert)
	}

	return &tls.Config{
		Certificates: certs, // Server certs
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
		log.Fatalf("Cannot start connection: %v\n", err)
	}
	*client = pb.NewDiceGameClient(conn)

	// Server connection info
	srv := grpc.NewServer(grpc.Creds(tlsCreds))
	pb.RegisterDiceGameServer(srv, &server{})

	// Initialize listener
	lis, err := net.Listen("tcp", *ownAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v\n", err)
	}

	// Server
	log.Printf("server listening at %v\n", lis.Addr())
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v\n", err)
	}
}

func main() {
	// Prepare
	flag.Parse()
	starts := false

	if *name == "Alice" {
		starts = true
	}

	// Setup connection
	var client pb.DiceGameClient
	go initPeer(&client)

	// Wait for peer to come online
	time.Sleep(2 * time.Second)

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	/*
		Game loop starts here
	*/

	for i := 0; i < rounds; i++ {
		log.Printf("Round %d\n", i+1)

		if starts {
			log.Printf("%v starts this round", *name)
		}

		throw, err := rand.Int(rand.Reader, big.NewInt(5))
		if err != nil {
			log.Fatalf("Error: %v\n", err)
		}

		m := throw.Uint64() + 1
		log.Printf("%v throws their die and gets: %d\n", *name, m)
		time.Sleep(time.Second)

		if starts {
			// Create commitment
			r := pedersen.GetR()
			c := pedersen.GetCommitment(m, r)

			// Send commitment to peer
			log.Printf("%v sends commitment: %d\n", *name, c)
			if _, err := client.SendCommitment(ctx, &pb.Commitment{C: c}); err != nil {
				log.Fatalf("Error: %v\n", err)
			}

			// Wait for die throw from peer
			time.Sleep(2 * time.Second)

			// Send commitment key to peer
			log.Printf("%v sends commitment key: (m: %v, r: %v)\n", *name, m, r)
			if _, err := client.SendCommitmentKey(ctx, &pb.CommitmentKey{M: m, R: r}); err != nil {
				log.Fatalf("Error: %v\n", err)
			}

			// Done
			time.Sleep(2 * time.Second)

			// Check winner
			if m > recvDieThrow.Val {
				log.Printf("%v won this round!\n", *name)
			} else if m < recvDieThrow.Val {
				log.Printf("%v lost this round!\n", *name)
			} else {
				log.Println("It's a tie!")
			}
		} else {
			// Wait for commitment from peer
			time.Sleep(2 * time.Second)

			// Send die throw to peer
			log.Printf("%v sends die throw: %d\n", *name, m)
			if _, err := client.SendDieThrow(ctx, &pb.DieThrow{Val: m}); err != nil {
				log.Fatalf("Error: %v\n", err)
			}

			// Wait for commitment key from peer
			time.Sleep(2 * time.Second)

			// Validate commitment from peer
			if pedersen.ValidateCommitment(recvCom.C, recvComKey.M, recvComKey.R) {
				log.Printf("%v confirms commitment is valid\n", *name)
			} else {
				log.Printf("%v's opponent is cheating! (c: %d, m: %d, r: %d)\n", *name, recvCom.C, recvComKey.M, recvComKey.R)
				os.Exit(0)
			}

			// Check winner
			if m > recvComKey.M {
				log.Printf("%v won this round!\n", *name)
			} else if m < recvComKey.M {
				log.Printf("%v lost this round!\n", *name)
			} else {
				log.Println("It's a tie!")
			}
		}

		// Switch turns
		starts = !starts
	}
}
