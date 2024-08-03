# fork-kimchi

Fork of [kimchi][].

## Changes compared to upstream

* Docker image.
* Short timeouts.
* Response compression.
* A bunch of headers.
* `/ping` endpoint for health check.
* `/robots.txt`.

## Usage

Create example config file `./kimchi.scfg`:

```scfg
# Using `http+insecure` in this specific example to disable automatic
# redirection to HTTPS.
site http+insecure://localhost:3000/ {
    file_server /srv/localhost
}
```

Listen on `127.0.0.1:3000`:

```sh
docker run --rm -it \
    -v './kimchi.scfg:/etc/kimchi/config:ro' \
    -p '127.0.0.1:3000:3000' \
    'ghcr.io/c032/fork-kimchi:main'
```

The container runs kimchi with UID and GID `100000`.

## License

MIT

[kimchi]: https://sr.ht/~emersion/kimchi
