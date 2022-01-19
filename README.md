# [kimchi]

A bare-bones HTTP server. Designed to be used together with [tlstunnel].

```
site example.org {
	root /srv/http
}

site example.com {
	reverse_proxy http://localhost:8080
}
```

## Contributing

Send patches to the [mailing list], report bugs on the [issue tracker].

## License

MIT

[kimchi]: https://sr.ht/~emersion/kimchi
[tlstunnel]: https://sr.ht/~emersion/tlstunnel
[mailing list]: https://lists.sr.ht/~emersion/public-inbox
[issue tracker]: https://todo.sr.ht/~emersion/kimchi
