package WebSocket

// HTTP/1.1 101 Switching Protocols
// Upgrade: websocket
// Connection: Upgrade
// Sec-WebSocket-Accept: rG8wsswmHTJ85lJgAE3M5RTmcCE=

// Websocket Frame format:
//  0                   1                   2                   3
//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-------+-+-------------+-------------------------------+
// |F|R|R|R| opcode|M| Payload len |    Extended payload length    |
// |I|S|S|S|  (4)  |A|     (7)     |             (16/64)           |
// |N|V|V|V|       |S|             |   (if payload len==126/127)   |
// | |1|2|3|       |K|             |                               |
// +-+-+-+-+-------+-+-------------+ - - - - - - - - - - - - - - - +
// |     Extended payload length continued, if payload len == 127  |
// + - - - - - - - - - - - - - - - +-------------------------------+
// |                               |Masking-key, if MASK set to 1  |
// +-------------------------------+-------------------------------+
// | Masking-key (continued)       |          Payload Data         |
// +-------------------------------- - - - - - - - - - - - - - - - +
// :                     Payload Data continued ...                :
// + - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - +
// |                     Payload Data continued ...                |
// +---------------------------------------------------------------+
// 

import (
    http "livesv/HttpPackage"
    "crypto/sha1"
    "encoding/base64"
    "fmt"
    "os"
)

var (
    CLOSE_FRAME = []byte{0b10000000 | 0x8, 0}
)

func Build_websocket_accept(key string) http.HttpResponse {
    const magical_suffix = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
    var result http.HttpResponse
    result.Headers = make(map[string]string)
    accept_key_sha1 := sha1.Sum([]byte(key + magical_suffix))
    accept_key_base64 := base64.StdEncoding.EncodeToString(accept_key_sha1[:])
    result.Version = "HTTP/1.1"
    result.Code = "101"
    result.Msg = "Switching Protocols"
    result.Headers = map[string]string{
        "Upgrade": "websocket",
        "Connection": "Upgrade",
        "Sec-WebSocket-Accept": accept_key_base64,
    }
    return result
}

func Build_websocket_frame_msg(msg string) []byte {
    if len(msg) < 126 {
        return append([]byte{0x81, byte(len(msg))}, msg...)
    } else {
        fmt.Fprintf(os.Stderr, "[ERROR] Umimplemented\n")
        os.Exit(1)
    }
    return nil
}
