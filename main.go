package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

var socketConn *net.Conn // Pointer to the connection

func main() {
	// Establish the connection to the socket server at startup
	err := connectToSocketServer()
	if err != nil {
		fmt.Println("Failed to connect to socket server:", err)
		return
	}
	defer closeSocketConn()

	// Initialize the Gin router
	router := gin.Default()

	// Define the endpoint
	router.POST("/send", func(c *gin.Context) {
		// Extract the message from the request body
		var request struct {
			Message string `json:"message"`
		}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request payload"})
			return
		}

		// Send the message to the socket server
		response, err := sendToSocketServer(request.Message)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		// Return the response from the socket server
		c.JSON(200, gin.H{"response": response})
	})

	// Start the REST API server
	router.Run(":8080")
}

// Establish the connection to the socket server
func connectToSocketServer() error {
	conn, err := net.Dial("tcp", "localhost:8888")
	if err != nil {
		return err
	}
	socketConn = &conn
	return nil
}

// Close the socket connection
func closeSocketConn() {
	if socketConn != nil && *socketConn != nil {
		(*socketConn).Close()
	}
}

func sendToSocketServer(requestString string) (string, error) {
	if socketConn == nil || *socketConn == nil {
		fmt.Println("Socket connection is closed. Attempting to reconnect...")
		if err := connectToSocketServer(); err != nil {
			return "", fmt.Errorf("failed to reconnect to socket server: %v", err)
		}
		fmt.Println("Reconnected to the socket server.")
	}

	// Prepare the message to send to the socket server
	requestString = strings.TrimSpace(requestString)
	requestLength := len(requestString) + 2

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, uint16(requestLength)); err != nil {
		return "", fmt.Errorf("error writing message length: %v", err)
	}

	/*
		// Send the length header and the message body
		if _, err := (*socketConn).Write(buf.Bytes()); err != nil {
			panic(err)
			//return "", fmt.Errorf("error sending message length: %v", err)
		}
		if _, err := (*socketConn).Write([]byte(requestString)); err != nil {
			return "", fmt.Errorf("error sending message body: %v", err)
		}
	*/
	// Send the message to the socket server
	err := writeToSocket(buf.Bytes(), requestString)
	if err != nil {
		// Handle reconnection on write failure
		fmt.Println("Write failed. Attempting to reconnect...")
		if err := connectToSocketServer(); err != nil {
			return "", fmt.Errorf("failed to reconnect to socket server: %v", err)
		}

		// Retry writing after reconnecting
		err = writeToSocket(buf.Bytes(), requestString)
		if err != nil {
			return "", fmt.Errorf("error sending message after reconnect: %v", err)
		}
	}

	// Read the response from the socket server
	socketReader := bufio.NewReader(*socketConn)

	// Read the 2-byte header
	header := make([]byte, 2)
	if _, err := socketReader.Read(header); err != nil {
		return "", fmt.Errorf("error reading response header: %v", err)
	}

	fmt.Printf("Header received: %v\n", header)

	// Parse the header to determine the response body length
	bodyLength := int(header[0])<<8 | int(header[1])

	fmt.Printf("Body received: %v\n", bodyLength)

	// Read the response body
	/*
		body := make([]byte, bodyLength)
		if _, err := io.ReadFull(socketReader, body); err != nil {
			return "", fmt.Errorf("error reading response body: %v", err)
		}
	*/

	body := make([]byte, bodyLength)
	_, err = socketReader.Read(body)
	if err != nil {
		//fmt.Println("Error reading body:", err)
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	response := strings.ReplaceAll(string(body), "\x00", "")

	fmt.Printf("Body received: %v\n", response)
	return response, nil
}

func writeToSocket(header []byte, body string) error {
	if _, err := (*socketConn).Write(header); err != nil {
		return err
	}
	if _, err := (*socketConn).Write([]byte(body)); err != nil {
		return err
	}
	return nil
}
