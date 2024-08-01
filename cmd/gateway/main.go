package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	pb "github.com/ratheeshkumar/SampleUserGateWAy/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Handler struct {
	grpcClient pb.UserServiceClient
}

func NewHandler(grpcClient pb.UserServiceClient) *Handler {
	return &Handler{grpcClient: grpcClient}
}

func (h *Handler) UserSignup(c *gin.Context) {
	var req pb.UserCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.grpcClient.UserSignup(context.Background(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListUsers(c *gin.Context) {
	req := &pb.FetchAll{}
	stream, err := h.grpcClient.ListUsers(context.Background(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var users []*pb.UserDetails
	for {
		user, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		users = append(users, user)
	}

	c.JSON(http.StatusOK, users)
}

func (h *Handler) UploadUsers(c *gin.Context) {
	stream, err := h.grpcClient.UploadUsers(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var users []*pb.UserCreate
	if err := c.ShouldBindJSON(&users); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, user := range users {
		if err := stream.Send(user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) Chat(c *gin.Context) {
	// Create a new stream for the bidirectional RPC
	stream, err := h.grpcClient.Chat(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Use a goroutine to handle sending messages
	go func() {
		defer stream.CloseSend()
		for {
			var req pb.Message
			if err := c.ShouldBindJSON(&req); err != nil {
				log.Printf("Failed to bind request: %v", err)
				return
			}

			if err := stream.Send(&req); err != nil {
				log.Printf("Failed to send message: %v", err)
				return
			}
		}
	}()

	// Handle receiving messages
	for {
		res, err := stream.Recv()
		if err == io.EOF {
			// Stream closed
			log.Println("End of stream")
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Send the received message back to the HTTP client
		c.JSON(http.StatusOK, res)
	}
}

func main() {
	conn, err := grpc.NewClient("localhost:3000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	grpcClient := pb.NewUserServiceClient(conn)
	handler := NewHandler(grpcClient)

	router := gin.Default()
	router.POST("/v1/usersignup", handler.UserSignup)
	router.GET("/v1/users", handler.ListUsers)
	router.POST("/v1/uploadusers", handler.UploadUsers)
	router.POST("/v1/chat", handler.Chat)

	log.Println("Serving Gin HTTP server on http://localhost:8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
