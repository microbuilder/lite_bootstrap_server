# Linaro Certificate Authority

A basic, proof-of-concept certificate authority (CA) and utility written in Go.

This utility can be used to validate and sign certificate signing requests
(CSRs), list previously registered devices, and enables a basic HTTP server
that can be used to communicate with the CA and it's underlying device
registry.

Certificate requests are logged to a local SQLite database named `CADB.db`,
which can be used to extend the utility for blocklist/allowlist type features,
and certificate status checks based on the registered device's unique ID.

## Quick Setup 

### Key Generation for HTTP Server

The HTTP server requires a private key for TLS, which can be generated via:

```bash
$ openssl ecparam -name secp256r1 -genkey -out SERVER.key
```

You can then generate a self-signed X.509 public key via:

```bash
$ openssl req -new -x509 -sha256 -days 3650 -key SERVER.key -out SERVER.crt \
        -subj "/O=Linaro, LTD/CN=localhost"
```

This public key should be available on any devices connecting to the CA to
verify that we are communicating with the intended CA server.

The contents of the certificate can be verified via:

```bash
$ openssl x509 -in SERVER.crt -noout -text
```

### Generate a CA key

```bash
$ ./linaroca cakey generate
generate called
use cafile: CA.crt
```

## HTTPS Server

To initialise the HTTPS server on the default port (443), run:

> :information_source: Use the `-p <port>` flag to change the port number.

```bash 
$ linaroca server start
Starting HTTPS server on port https://localhost:443
```

This will serve web pages from root, and handle REST API requests from the
`/api/v1` sub-path.

### REST API Endpoints

API based loosely on [CMP (RFC4210)](https://tools.ietf.org/html/rfc4210).

#### Initialisation Request: `/api/v1/ir` **POST**

This endpoint is used to register new (previously unregistered) devices into
the management system. Initialisation must occur before certificates can be
requested.

A unique serial number must be provided for the device, and any certificates
issued for this device will be associated with this device serial number.

#### Certification Request: `/api/v1/cr` **POST**

This endpoint is used for certificate requests from existing devices who
wish to obtain new certificates.

The CA will assign and record a unique serial number for this certificate,
which can be used later to check the certificate status via the `cs` endpoint.

#### Certification Request from PKCS10: `/api/v1/p10cr` **POST**

This endpoint is used for certificate requests from existing devices who
wish to obtain new certificates, providing a PKCS#10 CSR file for the request.

The CA will assign and record a unique serial number for this certificate,
which can be used later to check the certificate status via the `cs` endpoint.

#### Certificate Status Request: `api/v1/cs/{serial}` **GET**

Requests the certificate status based on the supplied certificate serial number.

#### Key Update Request: `api/v1/kur` **POST**

Request an update to an existing (non-revoked and non-expired) certificate. An
update is a replacement certificate containing either a new subject public
key or the current subject public key.

#### Key Revocation Request: `api/v1/krr` **POST**

Requests the revocation of an existing certificate registration.

## Certificate Request Process

To simulate the generation of a certificate request from a device,
a simple test program has been added, `make_csr_json.go`.

In combination with the CA server, a certificate signing request can be
converted to a JSON file, which is then presented to linaroca for processing
via `wget`, eventually returning the generated certificate.

1. Generate a user CSR key with openssl:

> Each device requires its own private key, which is securely held on the
  embedded device, and should never be exposed. For simulation purposes,
  however, we generate a user key locally with `openssl`.

```bash
$ openssl ecparam -name prime256v1 -genkey -out USER.key
```

2. Next, generate a CSR based on this private key. 

> Note that the `CN` of the subject **MUST** be a unique identifier for the
  device being simulated. A UUID is a good choice for this, and is used in the
  example below.

> **IMPORTANT**: Be sure to change the UUID used here as an example!

> **IMPORTANT**: The `O` field should be set to `localhost` for local
  tests.

```bash
$ openssl req -new -key USER.key -out USER.csr \
    -subj "/O=Orgname/CN=396c7a48-a1a6-4682-ba36-70d13f3b8902"
```

3. Run `make_csr_json.go` to convert the CSR above into a .json file:

```bash
$ go run make_csr_json.go
```

4. Start `linaroca`:

> **NOTE**: You may need to generate keys for the CA and HTTPS server,
  as described earlier in this readme.

> **NOTE**: The port used may vary on your system, or you can manually
  specify a port using the `-p` or `--port` flag.

```bash
$ go build
$ ./linaroca server start
Starting HTTPS server on port https://localhost:443
```

5. Send the JSON CSR to linaroca using `wget`:

```bash
$ wget --ca-certificate=SERVER.crt \
    --post-file USER.json \
    https://localhost:443/api/v1/cr
--2020-10-05 13:30:17--  https://localhost/api/v1/cr
Resolving localhost (localhost)... ::1, 127.0.0.1
Connecting to localhost (localhost)|::1|:443... connected.
HTTP request sent, awaiting response... 200 OK
Length: 567 [application/json]
Saving to: ‘cr.1’

cr.1                  100%[========================>]     567  --.-KB/s    in 0s      

2020-10-05 13:30:17 (77.2 MB/s) - ‘cr.1’ saved [567/567]
```

6. The certificate can now be viewed as follows:

```bash
$ cat cr.1
{"Status":0,"Cert":"MIIBlDCCATqgAwIBAgIIFjsVNsN2hQgwCgYIKoZIzj0EAwIwOjEUMBIGA1UEChMLTGluYXJvLCBMVEQxIjAgBgNVBAMTGUxpbmFyb0NBIFJvb3QgQ2VydCAtIDIwMjAwHhcNMjAxMDA1MTEzMDE3WhcNMjExMDA1MTEzMDE3WjBBMRAwDgYDVQQKEwdPcmduYW1lMS0wKwYDVQQDEyQzOTZjN2E0OC1hMWE2LTQ2ODItYmEzNi03MGQxM2YzYjg5MDIwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATMdS7FSxBP9CJpZIZyzq9ocTy7HisS0EMS78paXAMogZNSHAc4VotitaB53IFvnom4j1qFAF4PF3m/YBuVYHN4oyMwITAfBgNVHSMEGDAWgBQuja3DxwDP0PrFiaNwjSFmVgXcgzAKBggqhkjOPQQDAgNIADBFAiEApX/N3shitI6Yx19iLhcTu31FURcQUI8ZDHWF6UoiyK4CIEkYLQ4gjFwZ3Y+3L2bgczjxqppjG2yuKaetQLHFWeH4"}
```
