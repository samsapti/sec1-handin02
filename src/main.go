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
	ownPort       *int              = flag.Int("port", 50051, "gRPC port")
	peerAddr      *string           = flag.String("peer_addr", "localhost:50052", "Peer's gRPC port")
)

type server struct {
	pb.UnimplementedDiceGameServer
}

func (s *server) SendCommitment(ctx context.Context, in *pb.Commitment) (*pb.Empty, error) {
	log.Printf("Sending commitment: %v", in.GetC())
	commitment = &pb.Commitment{C: in.GetC()}
	return &pb.Empty{}, nil
}

func (s *server) SendCommitmentKey(ctx context.Context, in *pb.CommitmentKey) (*pb.Empty, error) {
	log.Printf("Sending commitment key: (m: %v, r: %v)", in.GetM(), in.GetR())
	commitmentKey = &pb.CommitmentKey{M: in.GetM(), R: in.GetR()}
	return &pb.Empty{}, nil
}

func main() {
	flag.Parse()

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

	client := pb.NewDiceGameClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Alice sends commitment")
	_, e := client.SendCommitment(ctx, &pb.Commitment{C: 235})
	if e != nil {
		log.Fatalf("Error: %v", e)
	}

	time.Sleep(time.Second)
	log.Printf("Bob receives commitment from Alice: %v\n", commitment.C)
}
