# go-multiaddr

[multiaddr](https://github.com/jbenet/multiaddr) implementation in Go.

## Example

### Simple

```go
import ma "github.com/jbenet/go-multiaddr"

// construct from a string (err signals parse failure)
m1, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234")

// construct from bytes (err signals parse failure)
m2, err := ma.NewMultiaddrBytes(m1.Bytes())

// true
strings.Equal(m1.String(), "/ip4/127.0.0.1/udp/1234")
strings.Equal(m1.String(), m2.String())
bytes.Equal(m1.Bytes(), m2.Bytes())
m1.Equal(m2)
m2.Equal(m1)
```

### Protocols

```go
// get the multiaddr protocol description objects
addr.Protocols()
// []Protocol{
//   Protocol{ Code: 4, Name: 'ip4', Size: 32},
//   Protocol{ Code: 17, Name: 'udp', Size: 16},
// }
```

### En/decapsulate

```go
m.Encapsulate(ma.NewMultiaddr("/sctp/5678"))
// <Multiaddr /ip4/127.0.0.1/udp/1234/sctp/5678>
m.Decapsulate(ma.NewMultiaddr("/udp")) // up to + inc last occurrence of subaddr
// <Multiaddr /ip4/127.0.0.1>
```

### Tunneling

Multiaddr allows expressing tunnels very nicely.

```js
printer, _ := ma.NewMultiaddr("/ip4/192.168.0.13/tcp/80")
proxy, _ := ma.NewMultiaddr("/ip4/10.20.30.40/tcp/443")
printerOverProxy := proxy.Encapsulate(printer)
// /ip4/10.20.30.40/tcp/443/ip4/192.168.0.13/tcp/80

proxyAgain := printerOverProxy.Decapsulate(printer)
// /ip4/10.20.30.40/tcp/443
```
