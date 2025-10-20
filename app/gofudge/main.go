package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Room struct {
	ID      string
	Users   map[*User]bool
	History []RollResult
	mu      sync.RWMutex
}

type User struct {
	Username string
	Conn     *websocket.Conn
	Room     *Room
}

type RollResult struct {
	Username  string    `json:"username"`
	Dice      []int     `json:"dice"`
	Modifier  int       `json:"modifier"`
	Total     int       `json:"total"`
	Timestamp time.Time `json:"timestamp"`
}

type Message struct {
	Type     string      `json:"type"`
	Data     interface{} `json:"data"`
	Username string      `json:"username,omitempty"`
}

var (
	rooms    = make(map[string]*Room)
	roomsMu  sync.RWMutex
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func generateRoomID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func rollFateDice() []int {
	dice := make([]int, 4)
	for i := 0; i < 4; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(3))
		dice[i] = int(n.Int64()) - 1 // -1, 0, or 1
	}
	return dice
}

func getOrCreateRoom(roomID string) *Room {
	roomsMu.Lock()
	defer roomsMu.Unlock()

	if room, exists := rooms[roomID]; exists {
		return room
	}

	room := &Room{
		ID:      roomID,
		Users:   make(map[*User]bool),
		History: []RollResult{},
	}
	rooms[roomID] = room
	return room
}

func deleteRoomIfEmpty(roomID string) {
	roomsMu.Lock()
	defer roomsMu.Unlock()

	if room, exists := rooms[roomID]; exists {
		room.mu.RLock()
		isEmpty := len(room.Users) == 0
		room.mu.RUnlock()

		if isEmpty {
			delete(rooms, roomID)
			log.Printf("Room %s deleted", roomID)
		}
	}
}

func (r *Room) broadcast(msg Message) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, _ := json.Marshal(msg)
	for user := range r.Users {
		user.Conn.WriteMessage(websocket.TextMessage, data)
	}
}

func (r *Room) addUser(user *User) {
	r.mu.Lock()
	r.Users[user] = true
	r.mu.Unlock()

	// Send room history to new user
	r.mu.RLock()
	history := r.History
	r.mu.RUnlock()

	historyMsg := Message{Type: "history", Data: history}
	data, _ := json.Marshal(historyMsg)
	user.Conn.WriteMessage(websocket.TextMessage, data)

	// Notify others
	r.broadcast(Message{
		Type:     "user_joined",
		Username: user.Username,
	})
}

func (r *Room) removeUser(user *User) {
	r.mu.Lock()
	delete(r.Users, user)
	r.mu.Unlock()

	r.broadcast(Message{
		Type:     "user_left",
		Username: user.Username,
	})
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Go Fudge Dice Roller</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: #1a1a1a;
            color: #e0e0e0;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            flex: 1;
        }
        h1 {
            color: #4a9eff;
            margin-bottom: 30px;
            text-align: center;
        }
        .section {
            background: #2a2a2a;
            border-radius: 8px;
            padding: 30px;
            margin-bottom: 20px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
        }
        input {
            width: 100%;
            padding: 12px;
            background: #1a1a1a;
            border: 2px solid #3a3a3a;
            border-radius: 4px;
            color: #e0e0e0;
            font-size: 16px;
            margin-bottom: 15px;
        }
        input:focus {
            outline: none;
            border-color: #4a9eff;
        }
        button {
            width: 100%;
            padding: 12px 24px;
            background: #4a9eff;
            color: white;
            border: none;
            border-radius: 4px;
            font-size: 16px;
            cursor: pointer;
            transition: background 0.3s;
        }
        button:hover {
            background: #3a8eef;
        }
        .invite-info {
            background: #1a1a1a;
            padding: 15px;
            border-radius: 4px;
            margin-top: 15px;
            word-break: break-all;
        }
        .label {
            font-weight: bold;
            color: #4a9eff;
            margin-bottom: 8px;
        }
        .footer {
            text-align: center;
            padding: 20px;
            color: #94a3b8;
            font-size: 14px;
        }
        .footer a {
            color: #4a9eff;
            text-decoration: none;
        }
        .footer a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Go Fudge Dice Roller</h1>
        <div class="section">
            <h2 style="color: #4a9eff; margin-bottom: 20px;">Create New Room</h2>
            <input type="text" id="username" placeholder="Enter your username">
            <button onclick="createRoom()">Create Room</button>
        </div>
    </div>
    <div class="footer">
        Licensed under the <a href="https://pil.jester-designs.com/" target="_blank">Prism Information License</a>
    </div>
    <script>
        function createRoom() {
            const username = document.getElementById('username').value.trim();
            if (!username) {
                alert('Please enter a username');
                return;
            }
            fetch('/api/create-room', { method: 'POST' })
                .then(r => r.json())
                .then(data => {
                    window.location.href = '/room/' + data.roomId + '?username=' + encodeURIComponent(username);
                });
        }
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

func handleRoom(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Path[len("/room/"):]
	username := r.URL.Query().Get("username")

	if username == "" {
		// Show username prompt
		tmpl := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Join Room - FATE</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: #1a1a1a;
            color: #e0e0e0;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .section {
            background: #2a2a2a;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
            max-width: 400px;
            width: 100%%;
        }
        h1 {
            color: #4a9eff;
            margin-bottom: 30px;
            text-align: center;
        }
        input {
            width: 100%%;
            padding: 12px;
            background: #1a1a1a;
            border: 2px solid #3a3a3a;
            border-radius: 4px;
            color: #e0e0e0;
            font-size: 16px;
            margin-bottom: 15px;
        }
        button {
            width: 100%%;
            padding: 12px;
            background: #4a9eff;
            color: white;
            border: none;
            border-radius: 4px;
            font-size: 16px;
            cursor: pointer;
        }
        .footer {
            text-align: center;
            padding: 20px;
            color: #94a3b8;
            font-size: 14px;
        }
        .footer a {
            color: #4a9eff;
            text-decoration: none;
        }
        .footer a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="section">
        <h1>Join Room</h1>
        <input type="text" id="username" placeholder="Enter your username">
        <button onclick="joinRoom()">Join</button>
    </div>
    <div class="footer">
        Licensed under the <a href="https://pil.jester-designs.com/" target="_blank">Prism Information License</a>
    </div>
    <script>
        function joinRoom() {
            const username = document.getElementById('username').value.trim();
            if (!username) {
                alert('Please enter a username');
                return;
            }
            window.location.href = '/room/%s?username=' + encodeURIComponent(username);
        }
    </script>
</body>
</html>`, roomID)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(tmpl))
		return
	}

	tmpl := template.Must(template.New("room").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>FATE Room</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: #1a1a1a;
            color: #e0e0e0;
            min-height: 100vh;
        }
        .container {
            max-width: 1000px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background: #2a2a2a;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        h1 { color: #4a9eff; }
        .invite-link {
            background: #1a1a1a;
            padding: 10px 15px;
            border-radius: 4px;
            font-size: 14px;
            display: flex;
            gap: 10px;
            align-items: center;
        }
        .copy-btn {
            padding: 5px 15px;
            background: #4a9eff;
            border: none;
            border-radius: 4px;
            color: white;
            cursor: pointer;
        }
        .roll-section {
            background: #2a2a2a;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }
        .roll-controls {
            display: flex;
            gap: 10px;
            align-items: center;
        }
        .roll-controls input {
            flex: 0 0 150px;
            padding: 10px;
            background: #1a1a1a;
            border: 2px solid #3a3a3a;
            border-radius: 4px;
            color: #e0e0e0;
        }
        .roll-controls button {
            flex: 1;
            padding: 10px 20px;
            background: #4a9eff;
            color: white;
            border: none;
            border-radius: 4px;
            font-size: 16px;
            cursor: pointer;
        }
        .history {
            background: #2a2a2a;
            border-radius: 8px;
            padding: 20px;
        }
        .roll-item {
            background: #1a1a1a;
            padding: 15px;
            border-radius: 4px;
            margin-bottom: 10px;
        }
        .roll-header {
            display: flex;
            justify-content: space-between;
            margin-bottom: 10px;
            color: #4a9eff;
        }
        .dice {
            display: flex;
            gap: 10px;
            margin: 10px 0;
        }
        .die {
            width: 40px;
            height: 40px;
            background: #2a2a2a;
            border-radius: 4px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: bold;
            font-size: 18px;
        }
        .die.plus { color: #4ade80; }
        .die.minus { color: #f87171; }
        .die.zero { color: #94a3b8; }
        .total {
            font-size: 24px;
            font-weight: bold;
            color: #4a9eff;
            margin-top: 10px;
        }
        .notification {
            background: #1a1a1a;
            padding: 10px;
            border-radius: 4px;
            margin-bottom: 10px;
            color: #94a3b8;
            font-style: italic;
        }
        .footer {
            text-align: center;
            padding: 20px;
            color: #94a3b8;
            font-size: 14px;
            margin-top: 20px;
        }
        .footer a {
            color: #4a9eff;
            text-decoration: none;
        }
        .footer a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Go Fudge Dice Roller</h1>
            <div class="invite-link">
                <span id="inviteLink"></span>
                <button class="copy-btn" onclick="copyInvite()">Copy</button>
            </div>
        </div>

        <div class="roll-section">
            <h2 style="margin-bottom: 15px;">Roll Dice</h2>
            <div class="roll-controls">
                <input type="number" id="modifier" placeholder="Modifier (±)" value="0">
                <button onclick="rollDice()">Roll Fuge Dice</button>
            </div>
        </div>

        <div class="history">
            <h2 style="margin-bottom: 15px;">Roll History</h2>
            <div id="rollHistory"></div>
        </div>
    </div>

    <div class="footer">
        Licensed under the <a href="https://pil.jester-designs.com/" target="_blank">Prism Information License</a>
    </div>

    <script>
        const roomId = '{{.RoomID}}';
        const username = '{{.Username}}';
        const ws = new WebSocket('ws://' + window.location.host + '/ws?room=' + roomId + '&username=' + encodeURIComponent(username));

        document.getElementById('inviteLink').textContent = window.location.origin + '/room/' + roomId;

        function copyInvite() {
            navigator.clipboard.writeText(window.location.origin + '/room/' + roomId);
            alert('Invite link copied!');
        }

        ws.onmessage = function(event) {
            const msg = JSON.parse(event.data);

            if (msg.type === 'history') {
                displayHistory(msg.data);
            } else if (msg.type === 'roll') {
                addRoll(msg.data);
            } else if (msg.type === 'user_joined') {
                addNotification(msg.username + ' joined the room');
            } else if (msg.type === 'user_left') {
                addNotification(msg.username + ' left the room');
            }
        };

        function rollDice() {
            const modifier = parseInt(document.getElementById('modifier').value) || 0;
            ws.send(JSON.stringify({ type: 'roll', data: { modifier: modifier } }));
        }

        function displayHistory(rolls) {
            const container = document.getElementById('rollHistory');
            container.innerHTML = '';
            rolls.forEach(roll => addRoll(roll));
        }

        function addRoll(roll) {
            const container = document.getElementById('rollHistory');
            const item = document.createElement('div');
            item.className = 'roll-item';

            const diceSum = roll.dice.reduce((a, b) => a + b, 0);
            const diceHTML = roll.dice.map(d =>
                '<div class="die ' + (d > 0 ? 'plus' : d < 0 ? 'minus' : 'zero') + '">' +
                (d > 0 ? '+' : d < 0 ? '−' : '0') + '</div>'
            ).join('');

            const modifierText = roll.modifier !== 0 ?
                ' ' + (roll.modifier > 0 ? '+' : '') + roll.modifier : '';

            item.innerHTML =
                '<div class="roll-header">' +
                    '<strong>' + roll.username + '</strong>' +
                    '<span>' + new Date(roll.timestamp).toLocaleTimeString() + '</span>' +
                '</div>' +
                '<div class="dice">' + diceHTML + '</div>' +
                '<div>Result: ' + diceSum + modifierText + '</div>' +
                '<div class="total">Total: ' + roll.total + '</div>';

            container.insertBefore(item, container.firstChild);
        }

        function addNotification(text) {
            const container = document.getElementById('rollHistory');
            const item = document.createElement('div');
            item.className = 'notification';
            item.textContent = text;
            container.insertBefore(item, container.firstChild);
        }
    </script>
</body>
</html>`))

	data := struct {
		RoomID   string
		Username string
	}{
		RoomID:   roomID,
		Username: username,
	}

	tmpl.Execute(w, data)
}

func handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	roomID := generateRoomID()
	json.NewEncoder(w).Encode(map[string]string{"roomId": roomID})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	username := r.URL.Query().Get("username")

	if roomID == "" || username == "" {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	room := getOrCreateRoom(roomID)
	user := &User{
		Username: username,
		Conn:     conn,
		Room:     room,
	}

	room.addUser(user)
	defer func() {
		room.removeUser(user)
		conn.Close()
		deleteRoomIfEmpty(roomID)
	}()

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		if msg.Type == "roll" {
			modifier := 0
			if m, ok := msg.Data.(map[string]interface{}); ok {
				if mod, ok := m["modifier"].(float64); ok {
					modifier = int(mod)
				}
			}

			dice := rollFateDice()
			sum := 0
			for _, d := range dice {
				sum += d
			}

			result := RollResult{
				Username:  username,
				Dice:      dice,
				Modifier:  modifier,
				Total:     sum + modifier,
				Timestamp: time.Now(),
			}

			room.mu.Lock()
			room.History = append(room.History, result)
			room.mu.Unlock()

			room.broadcast(Message{
				Type: "roll",
				Data: result,
			})
		}
	}
}

func main() {
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/room/", handleRoom)
	http.HandleFunc("/api/create-room", handleCreateRoom)
	http.HandleFunc("/ws", handleWebSocket)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
