# GoFudge

A Fudge Dice rolling room programmed in Go

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
- Upon visiting the URL you will be created with a username entry and Create room button. After that you will be in the room.
- (If you are hosting this publicly) You can copy the room link in the top right hand corner and share it to anyone. They will be prompted to also pick a username.
- From there you may increase/decrease the modifier as needed for the skill you are rolling for.


## If you want to use this with podman:
```bash
git clone https://codeberg.org/riomoo/gofudge.git
cd gofudge
./scripts-bash/run.sh
```

Then open http://localhost:12007 in your browser.

## Config for NGINX to use as a website:
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
