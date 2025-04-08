# OpenSSL generated keys for import/export tests

Created with commands:

```bash
openssl genpkey -algorithm ED25519 > openssl_ed25519.pem
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 > openssl_rsa.pem
```

secp key used in the 'restrict import key' test.
From: https://docs.openssl.org/1.1.1/man1/genpkey/
```bash
openssl genpkey -genparam -algorithm EC -out ecp.pem \
        -pkeyopt ec_paramgen_curve:secp384r1 \
        -pkeyopt ec_param_enc:named_curve
openssl genpkey -paramfile ecp.pem -out openssl_secp384r1.pem
rm ecp.pem
```
Note: The Bitcoin `secp256k1` curve which is what `go-libp2p-core/crypto`
actually generates and would be of interest to test against is not
recognized by the Go library:
```
Error: parsing PKCS8 format: x509: failed to parse EC private key embedded
 in PKCS#8: x509: unknown elliptic curve
```
We keep the `secp384r1` type instead from the original openssl example.
