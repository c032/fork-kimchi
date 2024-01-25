package main

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"git.sr.ht/~emersion/go-scfg"
)

func loadConfig(srv *Server, filename string) error {
	cfg, err := scfg.Load(filename)
	if err != nil {
		return err
	}

	cfg, err = resolveImports(cfg, filename)
	if err != nil {
		return err
	}

	return parseConfig(srv, cfg)
}

func parseConfig(srv *Server, cfg scfg.Block) error {
	for _, dir := range cfg {
		switch dir.Name {
		case "site":
			if err := parseSite(srv, dir); err != nil {
				return err
			}
		case "access-logs":
			if len(dir.Params) != 1 {
				return fmt.Errorf("invalid directive: need exactly one parameter: %v", dir.Name)
			}
			if srv.accessLogs != nil {
				return fmt.Errorf("invalid directive: only one directive of this kind is allowed: %v", dir.Name)
			}
			f, err := os.OpenFile(dir.Params[0], os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				return fmt.Errorf("opening access log file: %v", err)
			}
			// TODO: close this when we support closing the server without exiting in the future
			srv.accessLogs = f
		default:
			return fmt.Errorf("unknown directive %q", dir.Name)
		}
	}
	return nil
}

func parseSite(srv *Server, dir *scfg.Directive) error {
	for _, site := range dir.Params {
		uriStr := site
		if !strings.Contains(uriStr, "//") {
			uriStr = "//" + uriStr
		}

		u, err := url.Parse(uriStr)
		if err != nil {
			return fmt.Errorf("site %q: %v", site, err)
		}

		var ln *Listener
		var host, port string
		var insecure bool
		switch u.Scheme {
		case "", "http", "http+insecure":
			if host, port, err = net.SplitHostPort(u.Host); err != nil {
				host = u.Host
				port = "http"
			}
			ln = srv.AddListener("tcp", ":"+port)
			if u.Scheme == "http+insecure" {
				insecure = true
			}
		default:
			return fmt.Errorf("site %q: unknown URI scheme %q", site, u.Scheme)
		}

		path := u.Path
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("site %q: invalid path %q", site, path)
		}

		pattern := host + path

		// First process backend directives
		var backend http.Handler
		for _, child := range dir.Children {
			f, ok := backends[child.Name]
			if !ok {
				continue
			}

			if backend != nil {
				return fmt.Errorf("site %q: multiple HTTP backend directives provided", site)
			}

			backend, err = f(child)
			if err != nil {
				return fmt.Errorf("site %q: %v", site, err)
			}
		}
		if backend == nil {
			return fmt.Errorf("site %q: missing backend directive", site)
		}

		// Then process middleware directives
		handler := backend
		for _, child := range dir.Children {
			if _, ok := backends[child.Name]; ok {
				// Backend directive already processed above
				continue
			}

			handler, err = parseMiddleware(child, handler)
			if err != nil {
				return fmt.Errorf("site %q: directive %q: %v", site, child.Name, err)
			}
		}
		if !insecure {
			next := handler
			handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if redirectTLS(w, r) {
					return
				}
				next.ServeHTTP(w, r)
			})
		}

		handler = http.StripPrefix(path, handler)

		if srv.accessLogs != nil {
			next := handler
			handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				interceptWriter := interceptRW{
					ResponseWriter: w,
				}
				next.ServeHTTP(&interceptWriter, r)

				now := time.Now()

				scheme := "http"
				if contextTLSState(r.Context()) != nil {
					scheme = "https"
				}
				host := host
				if h := r.Header.Get("Host"); h != "" {
					host = h
				}
				referer := r.Header.Get("Referer")
				if referer == "" {
					referer = "-"
				}
				userAgent := r.Header.Get("User-Agent")
				if userAgent == "" {
					userAgent = "-"
				}
				line := fmt.Sprintf("%s - - [%s] \"%s %s://%s%s %s\" %d %d %q %q\n",
					r.RemoteAddr,
					now.Format("02/Jan/2006:15:04:05 -0700"),
					r.Method,
					scheme,
					host,
					r.RequestURI,
					r.Proto,
					interceptWriter.status,
					interceptWriter.size,
					referer,
					userAgent,
				)
				srv.accessLogs.WriteString(line)
			})
		}

		ln.Mux().Handle(pattern, handler)
	}
	return nil
}

type parseBackendFunc func(dir *scfg.Directive) (http.Handler, error)

var backends = map[string]parseBackendFunc{
	"file_server": func(dir *scfg.Directive) (http.Handler, error) {
		var dirname string
		if err := dir.ParseParams(&dirname); err != nil {
			return nil, err
		}
		var fs http.FileSystem = http.Dir(dirname)
		if dir.Children.Get("browse") == nil {
			fs = noBrowseFileSystem{fs}
		}
		return http.FileServer(fs), nil
	},
	"reverse_proxy": func(dir *scfg.Directive) (http.Handler, error) {
		var urlStr string
		if err := dir.ParseParams(&urlStr); err != nil {
			return nil, err
		}
		target, err := url.Parse(urlStr)
		if err != nil {
			return nil, err
		}
		director := func(req *http.Request) {
			proto := "http"
			if contextTLSState(req.Context()) != nil {
				proto = "https"
			}

			forwarded := fmt.Sprintf("for=%q;host=%q;proto=%q", req.RemoteAddr, req.Host, proto)
			forwardedForHost, _, _ := net.SplitHostPort(req.RemoteAddr)

			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			if strings.HasSuffix(target.Path, "/") {
				p := path.Join("/", req.URL.Path)
				req.URL.Path = strings.TrimSuffix(target.Path, "/") + p
			} else {
				req.URL.Path = target.Path
			}
			if target.RawQuery == "" || req.URL.RawQuery == "" {
				req.URL.RawQuery = target.RawQuery + req.URL.RawQuery
			} else {
				req.URL.RawQuery = target.RawQuery + "&" + req.URL.RawQuery
			}

			// Override reverse proxy header fields: the incoming request's
			// header is not trusted
			req.Header.Set("Forwarded", forwarded)
			if forwardedForHost != "" {
				req.Header.Set("X-Forwarded-For", forwardedForHost)
			} else {
				req.Header.Del("X-Forwarded-For")
			}
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Forwarded-Proto", proto)
		}
		return &httputil.ReverseProxy{Director: director}, nil
	},
	"redirect": func(dir *scfg.Directive) (http.Handler, error) {
		var to string
		if err := dir.ParseParams(&to); err != nil {
			return nil, err
		}
		return http.RedirectHandler(to, http.StatusFound), nil
	},
}

func parseMiddleware(dir *scfg.Directive, next http.Handler) (http.Handler, error) {
	switch dir.Name {
	case "header":
		// TODO: allow adding and removing fields
		setFields := make(map[string]string)
		if len(dir.Params) > 0 {
			if len(dir.Params) != 2 {
				return nil, fmt.Errorf("expected exactly two parameters")
			}
			setFields[dir.Params[0]] = dir.Params[1]
		} else {
			for _, child := range dir.Children {
				if len(child.Params) != 1 {
					return nil, fmt.Errorf("expected exactly one parameter for child directive")
				}
				if _, ok := setFields[child.Name]; ok {
					return nil, fmt.Errorf("duplicate child directive %q", child.Name)
				}
				setFields[child.Name] = child.Params[0]
			}
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range setFields {
				w.Header().Set(k, v)
			}
			next.ServeHTTP(w, r)
		}), nil
	case "basic_auth":
		var username, password string
		if err := dir.ParseParams(&username, &password); err != nil {
			return nil, err
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(u))
			passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(p))
			if !ok || (usernameOK&passwordOK) != 1 {
				w.Header().Set("WWW-Authenticate", "Basic")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		}), nil
	default:
		return nil, fmt.Errorf("unknown directive")
	}
}

func resolveImports(input scfg.Block, filename string) (scfg.Block, error) {
	dirname := filepath.Dir(filename)

	output := make(scfg.Block, 0, len(input))
	for _, dir := range input {
		switch dir.Name {
		case "import":
			var pattern string
			if err := dir.ParseParams(&pattern); err != nil {
				return nil, err
			}
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(dirname, pattern)
			}

			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("failed to import %q: %v", pattern, err)
			}

			for _, filename := range matches {
				block, err := scfg.Load(filename)
				if err != nil {
					return nil, err
				}

				block, err = resolveImports(block, filename)
				if err != nil {
					return nil, err
				}

				output = append(output, block...)
			}
		default:
			if len(dir.Children) > 0 {
				children, err := resolveImports(dir.Children, filename)
				if err != nil {
					return nil, err
				}

				dirCopy := *dir
				dirCopy.Children = children
				dir = &dirCopy
			}

			output = append(output, dir)
			continue
		}
	}

	return output, nil
}

type noBrowseFileSystem struct {
	http.FileSystem
}

func (fs noBrowseFileSystem) Open(name string) (http.File, error) {
	f, err := fs.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	return noBrowseFile{f}, nil
}

type noBrowseFile struct {
	http.File
}

func (f noBrowseFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, os.ErrPermission
}
