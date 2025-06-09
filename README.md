# WStunnel - Web Sockets Tunnel

![CodeRabbit Pull Request Reviews](https://img.shields.io/coderabbit/prs/github/rshade/wstunnel?utm_source=oss&utm_medium=github&utm_campaign=rshade%2Fwstunnel&labelColor=171717&color=FF570A&link=https%3A%2F%2Fcoderabbit.ai&label=CodeRabbit+Reviews)

- Master:
[![Build Status](https://github.com/rshade/wstunnel/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/rshade/wstunnel/actions/workflows/go.yml)
[![Coverage](https://codecov.io/gh/rshade/wstunnel/branch/master/graph/badge.svg)](https://app.codecov.io/gh/rshade/wstunnel)
- 1.1.1:
[![Build Status](https://github.com/rshade/wstunnel/actions/workflows/go.yml/badge.svg?branch=1.1.1)](https://github.com/rshade/wstunnel/actions/workflows/go.yml)
[![Coverage](https://codecov.io/gh/rshade/wstunnel/branch/1.1.1/graph/badge.svg)](https://app.codecov.io/gh/rshade/wstunnel)

WStunnel creates an HTTPS tunnel that can connect servers sitting
behind an HTTP proxy and firewall to clients on the internet. It differs from many other projects
by handling many concurrent tunnels allowing a central client (or set of clients) to make requests
to many servers sitting behind firewalls. Each client/server pair are joined through a rendez-vous token.

At the application level the
situation is as follows, an HTTP client wants to make request to an HTTP server behind a
firewall and the ingress is blocked by a firewall:

```AsciiDoc
  HTTP-client ===> ||firewall|| ===> HTTP-server
```

The WStunnel app implements a tunnel through the firewall. The assumption is that the
WStunnel client app running on the HTTP-server box can make outbound HTTPS requests. In the end
there are 4 components running on 3 servers involved:

- the http-client application on a client box initiates HTTP requests
- the http-server application on a server box behind a firewall handles the HTTP requests
- the WStunnel server application on a 3rd box near the http-client intercepts the http-client's
  requests in order to tunnel them through (it acts as a surrogate "server" to the http-client)
- the WStunnel client application on the server box hands the http requests to the local
  http-server app (it acts as a "client" to the http-server)
The result looks something like this:

```AsciiDoc

HTTP-client ==>\                      /===> HTTP-server
                |                      |
                \----------------------/
            WStunsrv <===tunnel==== WStuncli
```

But this is not the full picture. Many WStunnel clients can connect to the same server and
many http-clients can make requests. The rendez-vous between these is made using secret
tokens that are registered by the WStunnel client. The steps are as follows:

- WStunnel client is initialized with a token, which typically is a sizeable random string,
  and the hostname of the WStunnel server to connect to
- WStunnel client connects to the WStunnel server using WSS or HTTPS and verifies the
  hostname-certificate match
- WStunnel client announces its token to the WStunnel server
- HTTP-client makes an HTTP request to WStunnel server with a std URI and a header
  containing the secret token
- WStunnel server forwards the request through the tunnel to WStunnel client
- WStunnel client receives the request and issues the request to the local server
- WStunnel client receives the HTTP response and forwards that back through the tunnel, where
  WStunnel server receives it and hands it back to HTTP-client on the still-open original
  HTTP request

In addition to the above functionality, wstunnel does some queuing in
order to handle situations where the tunnel is momentarily not open. However, during such
queing any HTTP connections to the HTTP-server/client remain open, i.e., they are not
made aware of the queueing happening.

The implementation of the actual tunnel is intended to support two methods (but only the
first is currently implemented).  
The preferred high performance method is websockets: the WStunnel client opens a secure
websockets connection to WStunnel server using the HTTP CONNECT proxy traversal connection
upgrade if necessary and the two ends use this connection as a persistent bi-directional
tunnel.  
The second (Not yet implemented!) lower performance method is to use HTTPS long-poll where the WStunnel client
makes requests to the server to shuffle data back and forth in the request and response
bodies of these requests.

## Getting Started

You will want to have 3 machines handy (although you could run everything on one machine to
try it out):

- `www.example.com` will be behind a firewall running a simple web site on port 80
- `wstun.example.com` will be outside the firewall running the tunnel server
- `client.example.com` will be outside the firewall wanting to make HTTP requests to
  `www.example.com` through the tunnel

### Download

Release branches are named '1.N.M' and a '1.N' package is created with each revision
as a form of 'latest'.  Download the latest [Linux binary](https://binaries.rightscale.com/rsbin/wstunnel/1.0/wstunnel-linux-amd64.tgz)
and extract the binary. To compile for OS-X or Linux ARM clone the github repo and run
`make depend; make` (this is not tested).

### Set-up tunnel server

On `wstun.example.com` start WStunnel server (I'll pick a port other than 80 for sake of example)

```bash
$ ./wstunnel srv -port 8080 &
2014/01/19 09:51:31 Listening on port 8080
$ curl http://localhost:8080/_health_check
WSTUNSRV RUNNING
$
```

When using a base path, the health check URL includes the base path:

```bash
$ ./wstunnel srv -port 8080 -base-path /wstunnel &
2014/01/19 09:51:31 Listening on port 8080
2014/01/19 09:51:31 Base path configured basePath=/wstunnel
$ curl http://localhost:8080/wstunnel/_health_check
WSTUNSRV RUNNING
$
```

#### Server Configuration Options

The WStunnel server supports several configuration options to control resource usage and security:

**Password Authentication:**
To require passwords for specific tokens, use the `-passwords` option:

```bash
$ ./wstunnel srv -port 8080 -passwords 'token1:password1,token2:password2' &
2024/01/19 09:51:31 Listening on port 8080
$ # Server is now running with password authentication
```

**Request Limiting:**
To control the maximum number of queued requests per tunnel (default: 20):

```bash
$ ./wstunnel srv -port 8080 -max-requests-per-tunnel 50 &
2024/01/19 09:51:31 Listening on port 8080
```

This prevents any single tunnel from consuming too many server resources by limiting how many requests can be queued for processing.

**Client Limiting:**
To limit the number of clients that can connect with the same token (default: unlimited):

```bash
$ ./wstunnel srv -port 8080 -max-clients-per-token 1 &
2024/01/19 09:51:31 Listening on port 8080
```

This is useful when you want to ensure only a single client instance per token is allowed, preventing unauthorized token sharing or connection conflicts.

**Base Path Configuration:**
When running behind a reverse proxy (like Envoy, Istio Ingress Gateway, or nginx) with path-based routing, use the `-base-path` option to specify the base path for all endpoints:

```bash
$ ./wstunnel srv -port 8080 -base-path /wstunnel &
2024/01/19 09:51:31 Listening on port 8080
2024/01/19 09:51:31 Base path configured basePath=/wstunnel
```

With a base path configured, all WStunnel endpoints become available under the specified path:
- Health check: `http://proxy.example.com/wstunnel/_health_check`
- Stats: `http://proxy.example.com/wstunnel/_stats`
- Tunnel endpoint: `ws://proxy.example.com/wstunnel/_tunnel`
- Token-based requests: `http://proxy.example.com/wstunnel/_token/your-token/path`

**Combined Configuration Example:**

```bash
$ ./wstunnel srv -port 8080 \
  -base-path /api/v1/tunnel \
  -passwords 'prod-token:secure-password,dev-token:dev-pass' \
  -max-requests-per-tunnel 30 \
  -max-clients-per-token 2 &
```

### Start tunnel

On `www.example.com` verify that you can access the local web site:

```bash
$ curl http://localhost/some/web/page
<html> .......
```

Now set-up the tunnel:

```bash
$ ./wstunnel cli -tunnel ws://wstun.example.com:8080 -server http://localhost -token 'my_b!g_$secret!!'
2014/01/19 09:54:51 Opening ws://wstun.example.com/_tunnel
```

If the server is running with a base path (e.g., `-base-path /wstunnel`), include it in the tunnel URL:

```bash
$ ./wstunnel cli -tunnel ws://wstun.example.com:8080/wstunnel -server http://localhost -token 'my_b!g_$secret!!'
2014/01/19 09:54:51 Opening ws://wstun.example.com/wstunnel/_tunnel
```

To use a token with a password for additional security:

```bash
$ ./wstunnel cli -tunnel ws://wstun.example.com:8080 -server http://localhost -token 'my_b!g_$secret!!:mypassword'
# Or with base path:
$ ./wstunnel cli -tunnel ws://wstun.example.com:8080/wstunnel -server http://localhost -token 'my_b!g_$secret!!:mypassword'
```

> **Security Warning**: Passing passwords via command-line arguments is not recommended as they can be exposed through:
> - Process listings (visible to other users on the system)
> - Shell history files
> - System logs and crash dumps
> - Command-line argument inspection tools
>
> Instead, consider these more secure alternatives:
> - Use environment variables: `WSTUNNEL_PASSWORD=mypassword ./wstunnel cli ...`
> - Store credentials in a configuration file with restricted permissions
> - Use interactive password prompts (not yet implemented)
> - Use a secrets management service
>
> The same warning applies to the `-passwords` option on the server side.

### Make a request through the tunnel

On `client.example.com` use curl to make a request to the web server running on `www.example.com`:

```bash
$ curl 'https://wstun.example.com:8080/_token/my_b!g_$secret!!/some/web/page'
<html> .......
$ curl '-HX-Token:my_b!g_$secret!!' https://wstun.example.com:8080/some/web/page
<html> .......
```

If the server is running with a base path, include it in the request URLs:

```bash
$ curl 'https://wstun.example.com:8080/wstunnel/_token/my_b!g_$secret!!/some/web/page'
<html> .......
$ curl '-HX-Token:my_b!g_$secret!!' https://wstun.example.com:8080/wstunnel/some/web/page
<html> .......
```

### Running on Android

WStunnel can be run on Android devices using terminal emulators like Termux. See the [Android documentation](docs/ANDROID.md) for detailed setup instructions.

### Targeting multiple web servers

The above example tells WStunnel client to only forward requests to `http://localhost`. It is possible to allow the wstunnel to target multiple hosts too. For this purpose the original HTTP client must pass an X-Host header to name the host and WStunnel client must be configured with a regexp that limits the destination web server hostnames it allows. For example, to allow access to `*.some.example.com` over https use:

- `wstunnel cli -regexp 'https://.*\.some\.example\.com' -server https://default.some.example.com ...`
- `curl '-HX-Host: https://www.some.example.com'`

Or to allow access to www.example.com and blog.example.com over http you might use:

- `wstunnel cli -regexp 'http://(www\.example\.com|blog\.example\.com)' -server http://www.example.com ...`
- `curl '-HX-Host: http://blog.example.com'`

Note the use of -server and -regexp, this is because the server named in -server is used when there is no X-Host header. The host in the -server option does not have to match the regexp but it is recommended for it match.

### Using a Proxy

WStunnel client may use a proxy as long as that proxy supports HTTPS CONNECT. Basic authentication may be used if the username and password are embedded in the url. For example, `-proxy http://myuser:mypass@proxy-server.com:3128`. In addition, the command line client will also respect the https_proxy/http_proxy environment variables if they're set. As websocket connections are very long lived, please set read timeouts on your proxy as high as possible.

### Using Secure Web Sockets (SSL)

WStunnel does not support SSL natively (although that would not be a big change). The recommended
approach for using WSS (web sockets through SSL) is to use nginx, which uses the well-hardened
openssl library, whereas WStunnel would be using the non-hardened Go SSL implementation.
In order to connect to a secure tunnel server from WStunnel client use the `wss` URL scheme, e.g.
`wss://wstun.example.com`.
Here is a sample nginx configuration:

````nginx

server {
  listen       443;
  server_name  wstunnel.test.rightscale.com;

  ssl_certificate        <path to crt>;
  ssl_certificate_key    <path to key>;

  # needed for HTTPS
  proxy_set_header X-Forwarded-Proto https;
  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  proxy_set_header Host $http_host;
  proxy_set_header X-Forwarded-Host $host;
  proxy_redirect off;
  proxy_max_temp_file_size 0;

  #configure ssl
  ssl on;
  ssl_protocols SSLv3 TLSv1;
  ssl_ciphers HIGH:!ADH;
  ssl_prefer_server_ciphers on; # don't trust the client
  # caches 10 MB of SSL sessions in memory, faster than OpenSSL's cache:
  ssl_session_cache shared:SSL:10m;
  # cache the SSL sessions for 5 minutes, just as long as today's browsers
  ssl_session_timeout 5m;

  location / {
    root /mnt/nginx;

    proxy_redirect     off;
    proxy_http_version 1.1;

    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
    proxy_set_header Host $http_host;
    proxy_set_header X-Forwarded-For    $proxy_add_x_forwarded_for;
    proxy_buffering off;

    proxy_pass http://127.0.0.1:8080;   # assume wstunsrv runs on port 8080
  }
}
````

### Monitoring and Status Endpoint

WStunnel server provides a `/_stats` endpoint that displays information about connected tunnels (or `/your-base-path/_stats` when using a base path). When accessed from localhost, it provides detailed information including:

- Number of active tunnels
- Server configuration limits
- Token information for each tunnel
- Pending requests per tunnel
- Client IP address and reverse DNS lookup
- Client version information
- Idle time for each tunnel
- Current client counts per token (when limits are configured)

Example output:

```text
tunnels=2
max_requests_per_tunnel=20
max_clients_per_token=1
token_clients_my_token=1
token_clients_another_=1
total_clients=2

tunnel00_token=my_token_...
tunnel00_req_pending=0
tunnel00_tun_addr=192.168.1.100:54321
tunnel00_tun_dns=client.example.com
tunnel00_client_version=wstunnel dev - 2025-05-27 18:59:20 - cli-version
tunnel00_idle_secs=5.2

tunnel01_token=another_t...
tunnel01_req_pending=1
tunnel01_tun_addr=10.0.0.5:12345
tunnel01_client_version=wstunnel v1.0.0
tunnel01_idle_secs=120.5
```

The configuration limits section shows:

- `max_requests_per_tunnel`: Maximum queued requests per tunnel
- `max_clients_per_token`: Maximum clients allowed per token (0 = unlimited)
- `token_clients_*`: Current number of clients for each token (when limits are configured)
- `total_clients`: Total number of connected clients across all tokens

Note: Full statistics are only available when the endpoint is accessed from localhost. Remote requests will only see the total number of tunnels.

### Admin API Endpoints

WStunnel server provides two JSON API endpoints for programmatic monitoring and auditing:

#### `/admin/monitoring` - Aggregate Statistics

Returns high-level statistics in JSON format, suitable for monitoring dashboards and alerting systems.

**Example Request:**
```bash
curl http://localhost:8080/admin/monitoring
# With base path:
curl http://localhost:8080/wstunnel/admin/monitoring
```

**Example Response:**
```json
{
  "timestamp": "2025-01-09T15:30:45Z",
  "unique_tunnels": 3,
  "tunnel_connections": 3,
  "pending_requests": 5,
  "completed_requests": 1247,
  "errored_requests": 23
}
```

**Fields:**
- `unique_tunnels`: Number of unique tunnel tokens registered
- `tunnel_connections`: Number of active tunnel connections
- `pending_requests`: Current number of requests waiting for response
- `completed_requests`: Total successful requests since server start
- `errored_requests`: Total failed requests since server start

#### `/admin/auditing` - Detailed Tunnel Information

Returns comprehensive details about all active tunnels and their connections, suitable for security auditing and detailed analysis.

**Example Request:**
```bash
curl http://localhost:8080/admin/auditing
# With base path:
curl http://localhost:8080/wstunnel/admin/auditing
```

**Example Response:**
```json
{
  "timestamp": "2025-01-09T15:30:45Z",
  "tunnels": {
    "my_secret_token": {
      "token": "my_secret_token",
      "remote_addr": "192.168.1.100:54321",
      "remote_name": "client.example.com",
      "remote_whois": "Example Corp",
      "client_version": "wstunnel v1.0.0",
      "last_activity": "2025-01-09T15:30:40Z",
      "active_connections": [
        {
          "request_id": 123,
          "method": "GET",
          "uri": "/api/data",
          "remote_addr": "10.0.0.5",
          "start_time": "2025-01-09T15:30:30Z"
        }
      ],
      "last_error_time": "2025-01-09T15:25:00Z",
      "last_error_addr": "10.0.0.3",
      "last_success_time": "2025-01-09T15:30:35Z",
      "last_success_addr": "10.0.0.5",
      "pending_requests": 1
    }
  }
}
```

**Tunnel Fields:**
- `token`: The tunnel token (first 8 characters shown in logs)
- `remote_addr`: IP address and port of the tunnel client
- `remote_name`: Reverse DNS lookup of the client IP
- `remote_whois`: WHOIS information for the client IP (if available)
- `client_version`: Version string reported by the tunnel client
- `last_activity`: Timestamp of last tunnel activity
- `active_connections`: Array of currently active HTTP requests
- `last_error_time`: Timestamp of most recent failed request (optional)
- `last_error_addr`: IP address of most recent failed request (optional)
- `last_success_time`: Timestamp of most recent successful request (optional)
- `last_success_addr`: IP address of most recent successful request (optional)
- `pending_requests`: Number of requests currently pending

**Active Connection Fields:**
- `request_id`: Unique identifier for the request
- `method`: HTTP method (GET, POST, etc.)
- `uri`: The requested URI path
- `remote_addr`: IP address of the client making the request
- `start_time`: When the request was initiated

**Use Cases:**
- **Monitoring**: Use `/admin/monitoring` for dashboards, alerting, and performance tracking
- **Security Auditing**: Use `/admin/auditing` to track which clients are connecting from where
- **Debugging**: Use `/admin/auditing` to see active requests and recent errors
- **Capacity Planning**: Monitor request volumes and tunnel usage patterns
- **Web UI Integration**: Both endpoints return JSON suitable for web-based admin interfaces

**Security Note:** These endpoints are accessible without authentication. In production environments, consider placing them behind a reverse proxy with appropriate access controls.

### Reading wstunnel server logs

Sample:

```text
Apr  4 17:40:29 srv1 wstunsrv[7808]: INFO HTTP RCV      pkg=WStunsrv token=tech_x... id=19 verb=GET url=/ addr="10.210.2.11, 53.5.22.247" x-host= try=
Apr  4 17:40:29 srv1 wstunsrv[7808]: INFO WS   SND      pkg=WStunsrv token=tech_x... id=19 info="GET /"
Apr  4 17:40:29 srv1 wstunsrv[7808]: INFO WS   RCV      token=tech_x... id=19 ws=0xc20a416d20 len=393
Apr  4 17:40:29 srv1 wstunsrv[7808]: INFO HTTP RET      pkg=WStunsrv token=tech_x... id=19 status=401
```

The first line says that wstunsrv received an HTTP request to be tunneled and assigned it id 19.
The second line says that wstunsrv sent the request onto the appropriate websocket ("WS") to wstuncli.
The third line says that it has received a response to request 19 over the websocket.
The fourth line says that wstunsrv sent an HTTP response back to request 19, and that the status is a 401. What may not be obvious is that because it went round-trip to wstuncli the status code comes from wstuncli.

## Release instructions

- Run the following bash commands.

```bash
make depend
git tag -a $VERSION
git push --tags
```

- Create a branch for the changelog.
- Create the changelog:

```bash
git-chglog -o CHANGELOG.md
git add CHANGELOG.md
```

- Update Readme to reflect new `$VERSION`.
- Commit and push README and CHANGLOG Changes.
