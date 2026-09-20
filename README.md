<p align="center">
  <img src="images/favicon-border-name.svg" width="256" height="256" alt="GoFudge logo">
</p>

A real-time Fudge Dice rolling room, built in Go.

Spin up a lightweight room, share the link, and roll 4dF together over a live WebSocket connection. No accounts, no database, just you, your friends, and the dice.

## Features

- **Real-time rooms** - every roll, join, and departure is pushed to the whole table instantly over WebSockets.
- **Shareable invite links** - create a room, copy the link, and send it to your group. Anyone who opens it just picks a username and they're in.
- **4dF rolls** - rolls four Fudge dice (+, −, blank) and adds your modifier for the total, exactly like a tabletop session.
- **Synced roll history** - the whole room's history replays for anyone who joins late or reconnects, so nobody misses a beat.
- **Live player list** - see who's at the table in real time, with join and leave notifications as people come and go.
- **Automatic reconnect** - dropped connections retry on their own and pick back up the moment your tab regains focus.
- **Minimal footprint** - a single static Go binary, UPX compressed, that runs comfortably in a 75 MB container.
- **Self-cleaning rooms** - empty rooms are torn down automatically after a short grace period, so nothing lingers.

## Where the randomness comes from

Every roll is generated with Go's standard `math/rand` package: a fast, deterministic pseudo-random number generator (PRNG) that's seeded once when the server starts. Each Fudge die just asks that generator for a number between 0 and 2 and maps it to −, blank, or +.

That's intentionally lightweight, and on purpose. A Fudge roll isn't a security problem the way a lottery draw or a casino RNG is; there's no adversary trying to predict your next roll to steal something. Reaching for `crypto/rand` or a memory-hard algorithm like RandomX would add real overhead for a guarantee nobody at the table actually needs.

If you wanted to raise the trust bar anyway, say for streamed or competitive play where people might get suspicious of the server rolling this, `crypto/rand` is the natural next step. It's cryptographically secure, pulls from the OS's entropy pool, and is still part of the standard library, so it's a small swap.

## License

[![Custom badge](https://img.shields.io/endpoint?style=for-the-badge&url=https%3A%2F%2Fshare.jester-designs.com%2Fview%2Fpil.json)](LICENSE)

## Prerequisites

[![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)](https://golang.org/dl/)
- Go 1.25.3+ (only needed to build)
- Or just download a pre-built binary from Releases (when available)

### Quick start (from source)
```bash
git clone https://codeberg.org/riomoo/gofudge.git
cd gofudge
go build -o gofudge app/gofudge/main.go
./gofudge
```
- Visit http://localhost:8080 in your browser.
- You'll be greeted with a username field and a Create Room button. Fill it in and you're dropped straight into the room.
- (If you're hosting this publicly) Copy the room link from the top right corner and share it with anyone. They'll be prompted to pick a username too before joining.
- From there, adjust the modifier up or down for whatever skill you're rolling against, and hit Roll.


## Running with Podman

```bash
git clone https://codeberg.org/riomoo/gofudge.git
cd gofudge
make build-labeled-image
make start-container
```

Then open http://localhost:12007 in your browser.

## NGINX config, if you're hosting this as a website
```
upstream gofudge {
        server 127.0.0.1:8080;
        #server 127.0.0.1:12007; #For Podman instead
        server [::1]:8080;
        #server [::1]:12007; #For Podman instead
}
server {
        listen 80;
        listen [::1]:80;
        server_name fudge.example.com;
        location /ws {
                proxy_pass http://gofudge;
                proxy_http_version 1.1;
                
                # WebSocket upgrade headers
                proxy_set_header Upgrade $http_upgrade;
                proxy_set_header Connection "upgrade";
                
                # Standard proxy headers
                proxy_set_header Host $host;
                proxy_set_header X-Real-IP $remote_addr;
                proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
                proxy_set_header X-Forwarded-Proto $scheme;
                
                # WebSocket timeout settings (increase for long-lived connections)
                proxy_connect_timeout 7d;
                proxy_send_timeout 7d;
                proxy_read_timeout 7d;
                
                # Disable buffering for WebSocket
                proxy_buffering off;
                
                # Security headers
                add_header X-Content-Type-Options nosniff;
                add_header X-Frame-Options DENY;
                add_header X-XSS-Protection "1; mode=block";
                add_header Referrer-Policy "strict-origin-when-cross-origin";
        }
        location / {
                proxy_pass http://gofudge;
                proxy_set_header Host $host;
                proxy_set_header X-Real-IP $remote_addr;
                proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
                proxy_set_header X-Forwarded-Proto $scheme;

                # Connection keep-alive for better performance
                proxy_http_version 1.1;
                proxy_set_header Connection "";

                # Timeouts optimized for your simple site
                proxy_connect_timeout 5s;
                proxy_send_timeout 10s;
                proxy_read_timeout 10s;

                # Enable buffering for better compression
                proxy_buffering on;
                proxy_buffer_size 4k;
                proxy_buffers 8 4k;

                # Security headers
                add_header X-Content-Type-Options nosniff;
                add_header X-Frame-Options DENY;
                add_header X-XSS-Protection "1; mode=block";
                add_header Referrer-Policy "strict-origin-when-cross-origin";
        }
}
```

<div align="center">

## Software Used but not included

![Arch](https://img.shields.io/badge/Arch%20Linux-1793D1?logo=arch-linux&logoColor=fff&style=for-the-badge)
![Podman](https://img.shields.io/badge/-Podman-892CA0?style=flat-square&logo=podman&logoColor=white)
![Vim](https://img.shields.io/badge/VIM-%2311AB00.svg?style=for-the-badge&logo=vim&logoColor=white)
![Git](https://img.shields.io/badge/git-%23F05033.svg?style=for-the-badge&logo=git&logoColor=white)
![Forgejo](https://img.shields.io/badge/forgejo-%23FB923C.svg?style=for-the-badge&logo=forgejo&logoColor=white)

</div>
