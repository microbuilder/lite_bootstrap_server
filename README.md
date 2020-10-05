# Linaro Certificate Authority

A basic, proof-of-concept certificate authority (CA) and utility written in Go.

This utility can be used to validate and sign certificate signing requests
(CSRs), list previously registered devices, and enables a basic HTTP server
and REST API that can be used to communicate with the CA and it's underlying
device registry.

Certificate requests are logged to a local SQLite database (`CADB.db`), which
can be used to extend the utility for blocklist/allowlist type features,
and certificate status checks based on the registered device's unique ID.

## Quick Setup 

### Building `linaroca`

`linaroca` can be built with the following command:

```bash
$ go build
```

Supported commands and flags are visible via `--help`:

```bash
$ ./linaroca --help
A proof-of-concept certificate authority (CA) and management tool.

Usage:
  linaroca [command]

Available Commands:
  cakey       CA key management
  help        Help about any command
  server      HTTPS server management

Flags:
      --config string   config file (default is $HOME/.linaroca.yaml)
  -h, --help            help for linaroca
  -t, --toggle          Help message for toggle

Use "linaroca [command] --help" for more information about a command.
```

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

### Key Generation for Certificate Authority

The certificate authority requires a key to sign certificates, which can be
generated by the app:

```bash
$ ./linaroca cakey generate
generate called
use cafile: CA.crt
```

### Starting the CA Server

To initialise the HTTPS server on the default port (443 or 1443), run:

> :information_source: Use the `-p <port>` flag to change the port number.

```bash 
$ ./linaroca server start
Starting HTTPS server on port https://localhost:443
```

This will serve web pages from root, and handle REST API requests from the
`/api/v1` sub-path.

## REST API Endpoints

The REST API is a **work in progress**, and not fully implemented at present!

API based loosely on [CMP (RFC4210)](https://tools.ietf.org/html/rfc4210).

- `/api/v1/ir` Initialisation Request: **POST**

This endpoint is used to register new (previously unregistered) devices into
the management system. Initialisation must occur before certificates can be
requested.

A unique serial number must be provided for the device, and any certificates
issued for this device will be associated with this device serial number.

- `/api/v1/cr` Certification Request: **POST**

This endpoint is used for certificate requests from existing devices who
wish to obtain new certificates.

The CA will assign and record a unique serial number for this certificate,
which can be used later to check the certificate status via the `cs` endpoint.

- `/api/v1/p10cr` Certification Request from PKCS10: **POST**

This endpoint is used for certificate requests from existing devices who
wish to obtain new certificates, providing a PKCS#10 CSR file for the request.

The CA will assign and record a unique serial number for this certificate,
which can be used later to check the certificate status via the `cs` endpoint.

- `api/v1/cs/{serial}` Certificate Status Request: **GET**

Requests the certificate status based on the supplied certificate serial number.

- `api/v1/kur` Key Update Request: **POST**

Request an update to an existing (non-revoked and non-expired) certificate. An
update is a replacement certificate containing either a new subject public
key or the current subject public key.

- `api/v1/krr` Key Revocation Request: **POST** 

Requests the revocation of an existing certificate registration.

## Certificate Generation Workflow

The steps below can be followed to simulate a certificate request from a
hardware device.

This test scenario calls `make_csr_json.go`, which takes a certificate
signing request (CSR) file, and converts it into a JSON file that can be sent
to the CA server using the REST API. The encoded CSR file can then be sent to
the CA server using `wget`, which will return the generated certificate as
a JSON payload.

#### 1. Generate a private user key with openssl

Each device requires its own private key, which is securely held on the
embedded device, and should never be exposed. For simulation purposes, however,
we generate a user key locally with `openssl`, storing is as `USER.key`.

```bash
$ openssl ecparam -name prime256v1 -genkey -out USER.key
```

#### 2. Generate a CSR based on the private user key. 

The `CN` field of the certificate signing request's subject **MUST** be a
unique identifier for the device being registered. A UUID is a good choice
for this, and we can generate a new UUID as follows:

> **IMPORTANT**: Be sure to change the UUID from the example used here! This
  field MUST be unique, and should never be reused across devices.

```bash
$ uuidgen
  396c7a48-a1a6-4682-ba36-70d13f3b8902
```

Next, generate a CSR using this UUID and the private key in `USER.key`:

> **IMPORTANT**: The `O` field should be set to `localhost` for local
  tests, to match the value set in the HTTP Server's `LTD/CN` field (see
  earlier in this guide). This should be changed to a different, unique value
  (`MyOrgname`, etc.) in production.

```bash
$ openssl req -new -key USER.key -out USER.csr \
    -subj "/O=localhost/CN=396c7a48-a1a6-4682-ba36-70d13f3b8902"
```

#### 3. Convert the CSR to JSON (`make_csr_json.go`):

```bash
$ go run make_csr_json.go
```

#### 4. Start `linaroca`

> **NOTE**: You first need to generate keys for the CA and HTTPS server,
  as described earlier in this readme.

```bash
$ ./linaroca server start
Starting HTTPS server on port https://localhost:443
```

#### 5. Send `USER.json` to `api/v1/cr` using `wget`:

> Note the use of the `SERVER.crt` certificate to verify that we are talking
  to the server that we believe we are communicating with.

```bash
$ wget --ca-certificate=SERVER.crt \
    --post-file USER.json \
    https://localhost:443/api/v1/cr
```

This should produce the following output:

```
--2020-10-05 13:30:17--  https://localhost/api/v1/cr
Resolving localhost (localhost)... ::1, 127.0.0.1
Connecting to localhost (localhost)|::1|:443... connected.
HTTP request sent, awaiting response... 200 OK
Length: 567 [application/json]
Saving to: ‘cr’

cr                  100%[========================>]     567  --.-KB/s    in 0s      

2020-10-05 13:30:17 (77.2 MB/s) - ‘cr’ saved [567/567]
```

#### 6. View the JSON-encoded certificate response

If the CSR was accepted and successfully processed, the response will be a
certificate enclosed in a JSON wrapper, with the certificate payload BASE64
encoded in the `Cert` field:

```
{"Status":0,
"Cert":"MIIBlDCCATqgAwIBAgIIFjsVNsN2hQgwCgYIKoZIzj0EAwIwOjEUMBIGA1UEChMLTGluYXJ
vLCBMVEQxIjAgBgNVBAMTGUxpbmFyb0NBIFJvb3QgQ2VydCAtIDIwMjAwHhcNMjAxMDA1MTEzMDE3Wh
cNMjExMDA1MTEzMDE3WjBBMRAwDgYDVQQKEwdPcmduYW1lMS0wKwYDVQQDEyQzOTZjN2E0OC1hMWE2L
TQ2ODItYmEzNi03MGQxM2YzYjg5MDIwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATMdS7FSxBP9CJp
ZIZyzq9ocTy7HisS0EMS78paXAMogZNSHAc4VotitaB53IFvnom4j1qFAF4PF3m/
YBuVYHN4oyMwITAfBgNVHSMEGDAWgBQuja3DxwDP0PrFiaNwjSFmVgXcgzAKBggqhkjOPQQDAgNIADB
FAiEApX/N3shitI6Yx19iLhcTu31FURcQUI8ZDHWF6UoiyK4CIEkYLQ4gjFwZ3Y
+3L2bgczjxqppjG2yuKaetQLHFWeH4"}
```

This response packet will need to be parsed, and the certificate payload stored
on the embedded device.

As a test, you can parse the JSON packet and convert from BASE64 to binary via:

```bash
$ jq -r '.Cert' < cr | base64 --decode > USER.crt
```

The certificate can then be parsed as follows:

```bash
$ openssl x509 -inform der -in USER.crt -noout -text
```

Which should yield something resembling the following:

```
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 1601897417412085000 (0x163b1536c3768508)
    Signature Algorithm: ecdsa-with-SHA256
        Issuer: O=Linaro, LTD, CN=LinaroCA Root Cert - 2020
        Validity
            Not Before: Oct  5 11:30:17 2020 GMT
            Not After : Oct  5 11:30:17 2021 GMT
        Subject: O=localhost, CN=396c7a48-a1a6-4682-ba36-70d13f3b8902
        Subject Public Key Info:
            Public Key Algorithm: id-ecPublicKey
                Public-Key: (256 bit)
                pub: 
                    04:cc:75:2e:c5:4b:10:4f:f4:22:69:64:86:72:ce:
                    af:68:71:3c:bb:1e:2b:12:d0:43:12:ef:ca:5a:5c:
                    03:28:81:93:52:1c:07:38:56:8b:62:b5:a0:79:dc:
                    81:6f:9e:89:b8:8f:5a:85:00:5e:0f:17:79:bf:60:
                    1b:95:60:73:78
                ASN1 OID: prime256v1
                NIST CURVE: P-256
        X509v3 extensions:
            X509v3 Authority Key Identifier: 
                keyid:2E:8D:AD:C3:C7:00:CF:D0:FA:C5:89:A3:70:8D:21:66:56:05:DC:83

    Signature Algorithm: ecdsa-with-SHA256
         30:45:02:21:00:a5:7f:cd:de:c8:62:b4:8e:98:c7:5f:62:2e:
         17:13:bb:7d:45:51:17:10:50:8f:19:0c:75:85:e9:4a:22:c8:
         ae:02:20:49:18:2d:0e:20:8c:5c:19:dd:8f:b7:2f:66:e0:73:
         38:f1:aa:9a:63:1b:6c:ae:29:a7:ad:40:b1:c5:59:e1:f8
```

#### 7. Optional: Verify the Certificate

You can verify that the generated certificate was signed by the CA using the
`CA.crt` file generated earlier in this guide.

First, convert `USER.crt` (from step 6 above) from DER format to PEM via:

```bash
$ openssl x509 -in USER.crt -inform DER -out USER.pem -outform PEM
```

Then verify `USER.pem` against `CA.crt`:

```bash
$ openssl verify -CAfile CA.crt USER.pem 
USER.pem: OK
```
