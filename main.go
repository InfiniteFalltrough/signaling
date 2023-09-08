package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clientsMutex sync.Mutex
var clients = make(map[*websocket.Conn]bool)
var roomsMutex sync.Mutex
var rooms = make(map[string]map[*websocket.Conn]bool)

func main() {
	http.HandleFunc("/", HandleWebSocket)
	fmt.Println("WebSocket server start")
	localIP := getIP() + ":8080"
	fmt.Println(localIP)
	http.ListenAndServe(":8080", nil)
}

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WS upgrading error: ", err)
		return
	}
	defer func() {
		conn.Close()
		clientsMutex.Lock()
		delete(clients, conn)
		clientsMutex.Unlock()
	}()

	// Read room ID from the request
	roomID := r.URL.Query().Get("room")

	if roomID == "" {
		fmt.Println("WS room ID not provided")
		return
	}

	clientsMutex.Lock()
	clients[conn] = true
	clientsMutex.Unlock()

	roomsMutex.Lock()
	if rooms[roomID] == nil {
		rooms[roomID] = make(map[*websocket.Conn]bool)
	}
	rooms[roomID][conn] = true
	roomsMutex.Unlock()

	fmt.Printf("WS connected to room %s\n", roomID)

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("WS reading error: ", err)
			break
		}
		fmt.Println("WS received message: ", message)

		// Handle received signaling message
		HandleSignalingMessage(roomID, conn, messageType, message)
	}
}

func HandleSignalingMessage(roomID string, sender *websocket.Conn, messageType int, message []byte) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	// Convert the message to a string
	messageStr := string(message)

	// Assuming the message is a JSON string with "type" field
	// For example: {"type": "offer", "sdp": "SDP_OFFER_DATA"}
	// You would parse the JSON and handle SDP offers and answers accordingly
	// For simplicity, let's just broadcast the message to all clients in the room
	roomsMutex.Lock()
	room := rooms[roomID]
	for client := range room {
		if isSame(sender, client) {
			fmt.Println("WS ignoring same sender")
		} else {
			err := client.WriteMessage(messageType, []byte(messageStr))
			if err != nil {
				fmt.Println("WS writing error:", err)
			}
		}
	}
	roomsMutex.Unlock()
}

func isSame(ws1, ws2 *websocket.Conn) bool {
	return ws1 == ws2
}

func getIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "localhost"
}
