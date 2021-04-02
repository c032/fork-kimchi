kimchi(1)

# NAME

kimchi - HTTP server

# SYNOPSIS

*kimchi* [options...]

# DESCRIPTION

kimchi is a simple HTTP server designed to be used behind a TLS reverse proxy.

# OPTIONS

*-h*, *-help*
	Show help message and quit.

*-config* <path>
	Path to the configuration file.

# CONFIG FILE

The config file has one directive per line. Directives have a name, followed
by parameters separated by space characters. Directives may have children in
blocks delimited by "{" and "}". Lines beginning with "#" are comments.

Example:

```
frontend example.org:443 {
    backend localhost:8080
}
```

The following directives are supported:

*site* <uri>... { ... }
	URI(s) to listen on for incoming HTTP requests.

	The following URIs are supported:

	- _[http://][host][:port][/path]_ sets up an HTTP listener with an
	  automatic HTTPS redirection. This is useful when used together with a
	  TLS reverse proxy.
	- _http+insecure://[host][:port][/path]_ sets up an HTTP listener without
	  HTTPS redirection.

	If the host is omitted, requests for all hosts will be handled. If the port
	is omitted, the default HTTP port (80) is assumed. If the path is omitted,
	requests for all paths will be handled.

	URI examples:

	- _:8080_ listens on port 8080
	- _example.org_ listens on port 80 and handles requests for host
	  "example.org"
	- _http+insecure://localhost_ listens on port 80 without redirecting to
	  HTTPS
	- _example.org/foo/_ listens on port 80 and handles requests for host
	  "example.org" and path begining with "/foo/"
	- _http://_ listens on port 80

	The site directive supports the following sub-directives:

	*reverse_proxy* <uri>
		Forward incoming requests to another HTTP server.

		The header fields are passed through, except _Host_ which is set to the
		target URI's. The _Forwarded_ header is set with the original remote
		IP address, port, host and protocol (see RFC 7239), as well as the X-Forwarded-For,
		X-Forwarded-Host and X-Forwarded-Proto headers.

	*file_server* <path> { ++
	*browse* ++
}
		Serve static files at the specified path.

		_browse_ enables file listings for directories that do not have an
		index file.

	*header* <key> <value> ++
*header* { ++
	<key> <value> ++
	... ++
}
		Set an HTTP header field.

	*basic_auth* <username> <password>
		Sets up HTTP basic authentication.

	*redirect* <to>
		Replies with an HTTP redirection.

*import* <pattern>
	Include external files.

	_pattern_ can be a filename or a glob pattern. Its contents will replace
	this directive. A glob that doesn't match any file is not an error. Paths
	are resolved relative to the file the *import* directive appears in.

	This directive is a special case: it is evaluated before the configuration
	is parsed, and it can appear anywhere.

# FILES

_/etc/kimchi/config_
	Default configuration file location.

# AUTHORS

Maintained by Simon Ser <contact@emersion.fr>, who is assisted by other
open-source contributors. For more information about kimchi development, see
https://sr.ht/~emersion/kimchi.
