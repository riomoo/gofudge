package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Room struct {
	ID        string
	Users     map[*User]bool
	Usernames map[string]int // username -> connection count
	History   []RollResult
	mu        sync.RWMutex
}

type User struct {
	Username string
	Conn     *websocket.Conn
	Room     *Room
	writeMu  sync.Mutex
}

func (u *User) writeJSON(v interface{}) error {
	u.writeMu.Lock()
	defer u.writeMu.Unlock()
	return u.Conn.WriteJSON(v)
}

func (u *User) writeMessage(messageType int, data []byte) error {
	u.writeMu.Lock()
	defer u.writeMu.Unlock()
	return u.Conn.WriteMessage(messageType, data)
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
		dice[i] = (rand.Intn(3)) - 1 // -1, 0, or 1
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
		ID:        roomID,
		Users:     make(map[*User]bool),
		Usernames: make(map[string]int),
		History:   []RollResult{},
	}
	rooms[roomID] = room
	return room
}

func deleteRoomIfEmpty(roomID string) {
	// Grace period: wait briefly in case users are reconnecting
	time.Sleep(10 * time.Second)

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
		user.writeMessage(websocket.TextMessage, data)
	}
}

func (r *Room) broadcastUserList() {
	r.mu.RLock()
	usernames := make([]string, 0, len(r.Usernames))
	for name := range r.Usernames {
		usernames = append(usernames, name)
	}
	r.mu.RUnlock()

	r.broadcast(Message{Type: "users", Data: usernames})
}

func (r *Room) addUser(user *User) {
	r.mu.Lock()
	r.Users[user] = true
	r.Usernames[user.Username]++
	isNew := r.Usernames[user.Username] == 1
	r.mu.Unlock()

	// Send room history to new user
	r.mu.RLock()
	history := r.History
	r.mu.RUnlock()

	historyMsg := Message{Type: "history", Data: history}
	data, _ := json.Marshal(historyMsg)
	user.writeMessage(websocket.TextMessage, data)

	// Only announce if this username wasn't already in the room
	if isNew {
		r.broadcast(Message{
			Type:     "user_joined",
			Username: user.Username,
		})
	}
	r.broadcastUserList()
}

func (r *Room) removeUser(user *User) {
	r.mu.Lock()
	delete(r.Users, user)
	r.Usernames[user.Username]--
	isGone := r.Usernames[user.Username] <= 0
	if isGone {
		delete(r.Usernames, user.Username)
	}
	r.mu.Unlock()

	if isGone {
		r.broadcast(Message{
			Type:     "user_left",
			Username: user.Username,
		})
	}
	r.broadcastUserList()
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Go Fudge Dice Roller</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
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
    <title>Join Room - Go Fudge Dice Roller</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
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
    <meta name="viewport" content="width=device-width, initial-scale=1">
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
            padding: 12px;
        }

        /* ── Header ── */
        .header {
            background: #2a2a2a;
            padding: 14px 16px;
            border-radius: 8px;
            margin-bottom: 12px;
            display: flex;
            flex-wrap: wrap;
            gap: 10px;
            justify-content: space-between;
            align-items: center;
        }
        h1 { color: #4a9eff; font-size: clamp(16px, 4vw, 22px); }
        .invite-link {
            background: #1a1a1a;
            padding: 8px 12px;
            border-radius: 4px;
            font-size: 13px;
            display: flex;
            gap: 8px;
            align-items: center;
            min-width: 0;
        }
        .invite-link span {
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            max-width: 180px;
        }
        .copy-btn {
            padding: 5px 12px;
            background: #4a9eff;
            border: none;
            border-radius: 4px;
            color: white;
            cursor: pointer;
            white-space: nowrap;
            font-size: 13px;
        }

        /* ── Two-column layout (desktop) ── */
        .main {
            display: flex;
            gap: 12px;
            align-items: flex-start;
        }
        .sidebar {
            flex: 0 0 160px;
            background: #2a2a2a;
            border-radius: 8px;
            padding: 12px;
            position: sticky;
            top: 12px;
        }
        .sidebar h2 {
            color: #4a9eff;
            font-size: 11px;
            text-transform: uppercase;
            letter-spacing: 0.07em;
            margin-bottom: 10px;
        }
        .user-entry {
            display: flex;
            align-items: center;
            gap: 7px;
            padding: 5px 0;
            font-size: 13px;
            border-bottom: 1px solid #3a3a3a;
        }
        .user-entry:last-child { border-bottom: none; }
        .user-dot {
            width: 7px;
            height: 7px;
            border-radius: 50%;
            background: #4ade80;
            flex-shrink: 0;
        }
        .user-name {
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .user-name.is-you { color: #4a9eff; }
        .content { flex: 1; min-width: 0; }

        /* ── Roll section ── */
        .roll-section {
            background: #2a2a2a;
            padding: 16px;
            border-radius: 8px;
            margin-bottom: 12px;
        }
        .roll-section h2 { margin-bottom: 12px; font-size: 15px; }
        .roll-controls {
            display: flex;
            gap: 8px;
            align-items: center;
            flex-wrap: wrap;
        }
        .modifier-row {
            display: flex;
            gap: 5px;
            align-items: center;
        }
        .modifier-row button {
            width: 40px;
            height: 40px;
            padding: 0;
            background: #3a3a3a;
            color: #e0e0e0;
            border: none;
            border-radius: 4px;
            font-size: 18px;
            cursor: pointer;
            flex-shrink: 0;
        }
        .modifier-row button:hover { background: #4a4a4a; }
        .modifier-row input {
            width: 70px;
            padding: 8px;
            background: #1a1a1a;
            border: 2px solid #3a3a3a;
            border-radius: 4px;
            color: #e0e0e0;
            font-size: 15px;
            text-align: center;
        }
        .roll-btn {
            flex: 1;
            min-width: 130px;
            padding: 12px 16px;
            background: #4a9eff;
            color: white;
            border: none;
            border-radius: 4px;
            font-size: 16px;
            cursor: pointer;
            touch-action: manipulation;
        }
        .roll-btn:hover { background: #3a8eef; }
        .roll-btn:active { background: #2a7edf; }

        /* ── History ── */
        .history {
            background: #2a2a2a;
            border-radius: 8px;
            padding: 16px;
        }
        .history h2 { margin-bottom: 12px; font-size: 15px; }
        .roll-item {
            background: #1a1a1a;
            padding: 12px;
            border-radius: 4px;
            margin-bottom: 8px;
        }
        .roll-header {
            display: flex;
            justify-content: space-between;
            margin-bottom: 8px;
            color: #4a9eff;
            font-size: 14px;
            flex-wrap: wrap;
            gap: 4px;
        }
        .dice {
            display: flex;
            gap: 8px;
            margin: 8px 0;
            flex-wrap: wrap;
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
            font-size: 22px;
            font-weight: bold;
            color: #4a9eff;
            margin-top: 8px;
        }

        /* ── Footer ── */
        .footer {
            text-align: center;
            padding: 16px;
            color: #94a3b8;
            font-size: 13px;
            margin-top: 12px;
        }
        .footer a { color: #4a9eff; text-decoration: none; }
        .footer a:hover { text-decoration: underline; }

        /* ── Mobile: stack sidebar above content ── */
        @media (max-width: 600px) {
            .container { padding: 8px; }
            .header { padding: 10px 12px; }
            .invite-link span { max-width: 120px; }
            .main { flex-direction: column; }
            .sidebar {
                flex: none;
                width: 100%;
                position: static;
                padding: 10px 12px;
            }
            .sidebar h2 { margin-bottom: 6px; }
            /* Show users in a horizontal pill row on mobile */
            #userList {
                display: flex;
                flex-wrap: wrap;
                gap: 6px;
            }
            .user-entry {
                border-bottom: none;
                background: #1a1a1a;
                border-radius: 20px;
                padding: 4px 10px;
                font-size: 12px;
            }
            .roll-controls { flex-direction: column; align-items: stretch; }
            .modifier-row { justify-content: center; }
            .roll-btn { width: 100%; font-size: 18px; padding: 14px; }
            .die { width: 36px; height: 36px; font-size: 16px; }
            .total { font-size: 20px; }
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

        <div class="main">
            <div class="sidebar">
                <h2>Players</h2>
                <div id="userList"></div>
            </div>
            <div class="content">
        <div class="roll-section">
            <h2>Roll Dice</h2>
            <div class="roll-controls">
                <div class="modifier-row">
                    <button onclick="adjustModifier(-1)">−</button>
                    <input type="text" id="modifier" class="modifier-input" placeholder="±0" value="0">
                    <button onclick="adjustModifier(1)">+</button>
                </div>
                <button class="roll-btn" onclick="rollDice()">Roll Fudge Dice</button>
            </div>
        </div>

        <div class="history">
            <h2>Roll History</h2>
            <div id="rollHistory"></div>
        </div>
            </div> <!-- end .content -->
        </div> <!-- end .main -->
    </div>

    <div class="footer">
        Licensed under the <a href="https://pil.jester-designs.com/" target="_blank">Prism Information License</a>
    </div>

    <script>
	const roomId = '{{.RoomID}}';
	const username = '{{.Username}}';

	let ws;
	let pendingRoll = null;
	let reconnectTimer = null;

	function connect() {
		if (ws && ws.readyState === WebSocket.CONNECTING) return; // already trying
		clearTimeout(reconnectTimer);

		ws = new WebSocket((window.location.protocol === 'https:' ? 'wss://' : 'ws://') + window.location.host + '/ws?room=' + roomId + '&username=' + encodeURIComponent(username));

		ws.onopen = function() {
			if (pendingRoll !== null) {
				ws.send(JSON.stringify({ type: 'roll', data: { modifier: pendingRoll } }));
				pendingRoll = null;
			}
		};

		ws.onclose = function() {
			showToast('Connection lost. Reconnecting...');
			reconnectTimer = setTimeout(connect, 1000);
		};

		ws.onerror = function() {
			ws.close();
		};

		ws.onmessage = handleMessage;
	}

	// Reconnect immediately when user comes back to the tab
	document.addEventListener('visibilitychange', function() {
		if (!document.hidden && (!ws || ws.readyState === WebSocket.CLOSED)) {
			clearTimeout(reconnectTimer);
			connect();
		}
	});

	connect();

	document.getElementById('inviteLink').textContent = window.location.origin + '/room/' + roomId;



	function showToast(message) {
	    const toast = document.createElement('div');
	    toast.textContent = message;

	    // Apply all styles inline to guarantee they work
	    Object.assign(toast.style, {
		position: 'fixed',
		top: '20px',
		left: '50%',
		transform: 'translateX(-50%) translateY(-20px)',
		background: '#4a9eff',
		color: 'white',
		padding: '15px 25px',
		borderRadius: '8px',
		boxShadow: '0 4px 12px rgba(0,0,0,0.4)',
		fontSize: '16px',
		fontWeight: '500',
		opacity: '0',
		transition: 'all 0.3s ease',
		pointerEvents: 'none',
		zIndex: '9999',
		whiteSpace: 'nowrap'
	    });

	    document.body.appendChild(toast);

	    setTimeout(function() {
		toast.style.opacity = '1';
		toast.style.transform = 'translateX(-50%) translateY(0)';
	    }, 10);

	    setTimeout(function() {
		toast.style.opacity = '0';
		toast.style.transform = 'translateX(-50%) translateY(-20px)';
		setTimeout(function() {
		    if (toast.parentNode) {
			document.body.removeChild(toast);
		    }
		}, 300);
	    }, 2000);
	}

	function copyInvite() {
	    const inviteUrl = window.location.origin + '/room/' + roomId;

	    // Try modern clipboard API first
	    if (navigator.clipboard && navigator.clipboard.writeText) {
		navigator.clipboard.writeText(inviteUrl).then(function() {
		    showToast('✓ Invite link copied!');
		}).catch(function() {
		    fallbackCopy(inviteUrl);
		});
	    } else {
		fallbackCopy(inviteUrl);
	    }
	}

	function fallbackCopy(text) {
	    const textarea = document.createElement('textarea');
	    textarea.value = text;
	    textarea.style.position = 'fixed';
	    textarea.style.opacity = '0';
	    document.body.appendChild(textarea);
	    textarea.select();
	    textarea.setSelectionRange(0, 99999);

	    try {
		document.execCommand('copy');
		showToast('✓ Invite link copied!');
	    } catch (err) {
		showToast('✗ Copy failed');
	    }

	    document.body.removeChild(textarea);
	}


	function adjustModifier(amount) {
	    const input = document.getElementById('modifier');
	    const currentValue = parseInt(input.value) || 0;
	    input.value = currentValue + amount;
	}

	document.addEventListener('DOMContentLoaded', function() {
	    document.getElementById('modifier').addEventListener('input', function(e) {
		if (e.target.value !== '' && e.target.value !== '-' && isNaN(parseInt(e.target.value))) {
		    e.target.value = e.target.value.slice(0, -1);
		}
	    });
	});

function handleMessage(event) {
	    const msg = JSON.parse(event.data);

	    if (msg.type === 'history') {
		displayHistory(msg.data);
	    } else if (msg.type === 'roll') {
		addRoll(msg.data);
	    } else if (msg.type === 'users') {
		updateUserList(msg.data);
	    } else if (msg.type === 'user_joined') {
		showToast(msg.username + ' joined');
	    } else if (msg.type === 'user_left') {
		showToast(msg.username + ' left');
	    }
	};

	function updateUserList(users) {
	    const container = document.getElementById('userList');
	    container.innerHTML = '';
	    (users || []).slice().sort().forEach(function(name) {
		const entry = document.createElement('div');
		entry.className = 'user-entry';
		entry.innerHTML =
		    '<div class="user-dot"></div>' +
		    '<div class="user-name' + (name === username ? ' is-you' : '') + '">' +
		    name + (name === username ? ' (you)' : '') +
		    '</div>';
		container.appendChild(entry);
	    });
	}

	function rollDice() {
		const modifier = parseInt(document.getElementById('modifier').value) || 0;
		if (!ws || ws.readyState !== WebSocket.OPEN) {
			pendingRoll = modifier;
			if (!ws || ws.readyState === WebSocket.CLOSED) {
				connect();
			}
			showToast('Reconnecting...');
			return;
		}
		ws.send(JSON.stringify({ type: 'roll', data: { modifier: modifier } }));
	}

	function displayHistory(rolls) {
	    const container = document.getElementById('rollHistory');
	    const fragment = document.createDocumentFragment();

	    for (let i = rolls.length - 1; i >= 0; i--) {
		fragment.appendChild(createRollElement(rolls[i]));
	    }

	    container.innerHTML = '';
	    container.appendChild(fragment);
	}

	function createRollElement(roll) {
	    const item = document.createElement('div');
	    item.className = 'roll-item';

	    const diceSum = roll.dice.reduce((a, b) => a + b, 0);

	    // Pre-compute all strings to avoid layout thrashing
	    const diceHTML = roll.dice.map(function(d) {
		var className = d > 0 ? 'plus' : d < 0 ? 'minus' : 'zero';
		var symbol = d > 0 ? '+' : d < 0 ? '−' : '0';
		return '<div class="die ' + className + '">' + symbol + '</div>';
	    }).join('');

	    const modifierText = roll.modifier !== 0 ? ' ' + (roll.modifier > 0 ? '+' : '') + roll.modifier : '';
	    const timestamp = new Date(roll.timestamp).toLocaleTimeString();

	    // Single innerHTML assignment to minimize reflows
	    item.innerHTML =
		'<div class="roll-header">' +
		    '<strong>' + roll.username + '</strong>' +
		    '<span>' + timestamp + '</span>' +
		'</div>' +
		'<div class="dice">' + diceHTML + '</div>' +
		'<div>Result: ' + diceSum + modifierText + '</div>' +
		'<div class="total">Total: ' + roll.total + '</div>';

	    return item;
	}

	function addRoll(roll) {
	    const container = document.getElementById('rollHistory');
	    const item = createRollElement(roll);

	    // Use prepend if available (faster than insertBefore)
	    if (container.prepend) {
		container.prepend(item);
	    } else {
		container.insertBefore(item, container.firstChild);
	    }
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

	// Ping every 30s to detect dead connections
	const pingInterval = 30 * time.Second

	conn.SetPongHandler(func(string) error { return nil })

	ticker := time.NewTicker(pingInterval)
	done := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				user.writeMu.Lock()
				err := conn.WriteMessage(websocket.PingMessage, nil)
				user.writeMu.Unlock()
				if err != nil {
					conn.Close()
					return
				}
			}
		}
	}()

	room.addUser(user)
	defer func() {
		close(done)
		room.removeUser(user)
		conn.Close()
		go deleteRoomIfEmpty(roomID)
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
	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/room/", handleRoom)
	http.HandleFunc("/api/create-room", handleCreateRoom)
	http.HandleFunc("/ws", handleWebSocket)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
