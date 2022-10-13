package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	pb "github.com/samsapti/sec1-handin-02/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	recvCom    *pb.Commitment    = &pb.Commitment{}
	recvComKey *pb.CommitmentKey = &pb.CommitmentKey{}
	name       *string           = flag.String("name", "Player_0", "Name of the player")
	ownPort    *int              = flag.Int("port", 50051, "gRPC port")
	peerAddr   *string           = flag.String("peer_addr", "localhost:50052", "Peer's gRPC port")
)

type server struct {
	pb.UnimplementedDiceGameServer
}

func (s *server) SendCommitment(ctx context.Context, in *pb.Commitment) (*pb.Empty, error) {
	log.Printf("%v receives commitment: %v", *name, in.GetC())
	recvCom = &pb.Commitment{C: in.GetC()}
	return &pb.Empty{}, nil
}

func (s *server) SendCommitmentKey(ctx context.Context, in *pb.CommitmentKey) (*pb.Empty, error) {
	log.Printf("%v receives commitment key: (m: %v, r: %v)", *name, in.GetM(), in.GetR())
	recvComKey = &pb.CommitmentKey{M: in.GetM(), R: in.GetR()}
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

func main() {
	flag.Parse()

	// Setup secure channel
	tlsCreds := credentials.NewTLS(getTLSConfig())

	// Client connection info
	conn, err := grpc.Dial(*peerAddr, grpc.WithTransportCredentials(tlsCreds))
	if err != nil {
		log.Fatalf("Cannot start connection: %v\n", err)
	}

	// Server connection info
	srv := grpc.NewServer(grpc.Creds(tlsCreds))
	pb.RegisterDiceGameServer(srv, &server{})

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *ownPort))
	if err != nil {
		log.Fatalf("failed to listen: %v\n", err)
	}

	log.Printf("server listening at %v\n", lis.Addr())
	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Wait for peer to come online
	time.Sleep(2 * time.Second)

	// Connect to peer
	client := pb.NewDiceGameClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	log.Printf("%v sends commitment: %v", *name, *ownPort)
	_, e := client.SendCommitment(ctx, &pb.Commitment{C: uint32(*ownPort)})
	if e != nil {
		log.Fatalf("Error: %v", e)
	}

	time.Sleep(time.Second)
	log.Printf("%v did receive commitment: %v\n", *name, recvCom.C)
}
