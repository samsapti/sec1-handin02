package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	pb "github.com/samsapti/sec1-handin-02/grpc"
	"google.golang.org/grpc"
)

var (
	commitment    *pb.Commitment    = &pb.Commitment{}
	commitmentKey *pb.CommitmentKey = &pb.CommitmentKey{}
	name          *string           = flag.String("name", "Player_0", "Name of the player")
	ownPort       *int              = flag.Int("port", 50051, "gRPC port")
	peerAddr      *string           = flag.String("peer_addr", "localhost:50052", "Peer's gRPC port")
)

type server struct {
	pb.UnimplementedDiceGameServer
}

func (s *server) SendCommitment(ctx context.Context, in *pb.Commitment) (*pb.Empty, error) {
	log.Printf("%v receives commitment: %v", *name, in.GetC())
	commitment = &pb.Commitment{C: in.GetC()}
	return &pb.Empty{}, nil
}

func (s *server) SendCommitmentKey(ctx context.Context, in *pb.CommitmentKey) (*pb.Empty, error) {
	log.Printf("%v receives commitment key: (m: %v, r: %v)", *name, in.GetM(), in.GetR())
	commitmentKey = &pb.CommitmentKey{M: in.GetM(), R: in.GetR()}
	return &pb.Empty{}, nil
}

func main() {
	flag.Parse()

	// Setup secure channel
	//altsClient := alts.NewClientCreds(alts.DefaultClientOptions())
	conn, err := grpc.Dial(*peerAddr, grpc.WithInsecure() /*grpc.WithTransportCredentials(altsClient)*/)
	if err != nil {
		log.Fatalf("Cannot start connection: %v", err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *ownPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterDiceGameServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	go func() {
		if err := s.Serve(lis); err != nil {
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
	log.Printf("%v did receive commitment: %v\n", *name, commitment.C)
}
