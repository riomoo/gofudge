# GoFudge

A Fudge Dice rolling room programmed in Go

## License

[![Custom badge](https://img.shields.io/endpoint?style=for-the-badge&url=https%3A%2F%2Fshare.jester-designs.com%2Fview%2Fpil.json)](LICENSE)

## Prerequisites

[![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)](https://golang.org/dl/)
- The version of **Go** used to test the code in this repository is **1.25.3**.

## Get started

- run the following commands: `go mod tidy; go build -o bin/main ./app/gofudge/main.go` then `./bin/main` to start the server on port 8080.
- Visit http://localhost:8080 in your browser.

- Upon visiting the URL you will be created with a username entry and Create room button. After that you will be in the room.
- (If you are hosting this publicly) You can copy the room link in the top right hand corner and share it to anyone. They will be prompted to also pick a username.
- From there you may increase/decrease the modifier as needed for the skill you are rolling for.

### Podman/Docker

- If you want to use this with Docker, replace all `podman` commands in `run.sh` instances with `docker`
- Use `run.sh` script which will start the site on port `12007` and to make it public change the `8080` port in the NGINX config to `12007` or change the port in the `run.sh` script how you like.
- To use it locally, same as above but visit http://localhost:12007 instead.

## Config for NGINX to use as a website:
```
upstream gofudge {
        server 127.0.0.1:8080;
        server [::1]:8080;
}
server {
        listen 80;
        listen [::1]:80;
        server_name fudge.example.com;
        location / {
                proxy_pass http://gofudge;
                proxy_set_header Host $host;
                proxy_set_header X-Real-IP $remote_addr;
                proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
                proxy_set_header X-Forwarded-Proto $scheme;
        }
}
```

<div align="center">

## Software Used but not included

![Arch](https://img.shields.io/badge/Arch%20Linux-1793D1?logo=arch-linux&logoColor=fff&style=for-the-badge)
![Vim](https://img.shields.io/badge/VIM-%2311AB00.svg?style=for-the-badge&logo=vim&logoColor=white)
![Git](https://img.shields.io/badge/git-%23F05033.svg?style=for-the-badge&logo=git&logoColor=white)
![Forgejo](https://img.shields.io/badge/forgejo-%23FB923C.svg?style=for-the-badge&logo=forgejo&logoColor=white)

</div>
