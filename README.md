# jaker
Simple CDN image uploader and host written in golang
Named after my best friend.


USAGE

git clone https://github.com/phluxx/jaker
cd jaker
go mod init jaker
go build -o jaker
./jaker -port 8675 -storage-dir /path/to/storage/dir -cert-file /path/to/ssl/cert -key-file /path/to/ssl/key -daemon

This will only listen and server over HTTPS, so a certificate and keyfile must exist.

I recommend using Let's Encrypt and certbot to generate your certificates.