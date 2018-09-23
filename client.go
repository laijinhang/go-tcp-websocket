package websocket

import (
	"net"
	"encoding/binary"
	"crypto/rand"
	"github.com/imroc/biu"
	"io"
)

type WebSocketClient struct {
	conn net.Conn
}

func createRequestHeader(host string) string {
	connRequest := `GET /ws HTTP/1.1
Host: ` + host + `
Connection: Upgrade
Pragma: no-cache
Cache-Control: no-cache
User-Agent: Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36
Upgrade: websocket
Origin: file://
Sec-WebSocket-Version: 13
Accept-Encoding: gzip, deflate
Accept-Language: zh-CN,zh;q=0.9
Sec-WebSocket-Key: VR+OReqwhymoQ21dBtoIMQ==
Sec-WebSocket-Extensions: permessage-deflate; client_max_window_bits

`
	return connRequest
}

func NewClient(address string) (*WebSocketClient, error) {
	connRequest := createRequestHeader(address)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	// 发送请求头给服务器
	_, err = conn.Write([]byte(connRequest))
	if err != nil {
		return nil, err
	}
	// 获取服务器发送的响应头
	buf := make([]byte, 1024)
	_, err = conn.Read(buf)
	if err != nil {
		return nil, err
	}

	return &WebSocketClient{conn:conn}, err
}

// 根据websocket协议对string数据进行封装，先mask编码，然后转二进制
func EnProtoc(v interface{}) []byte {
	var s string
	var Opcode byte
	switch v.(type) {	// 传输字符串文本
	case string:
		Opcode = Opcode | 0x81
		s = v.(string)
	case []byte:	// 传输二进制流
		Opcode = Opcode | 0x82
		s = string(v.([]byte))
	}

	length := uint64(len(s))
	var x uint64	// x is Payload length
	var data []byte
	maskingKey := make([]byte, 4)
	io.ReadFull(rand.Reader, maskingKey)

	data = append(data, Opcode)
	if length > 0 && length < 126 {	// 直接发送
		x = length
		data = append(data, byte(x + 128))	// 掩码加长度
	} else if length >= 126 && length < 65536 {	// 直接发送，大于125字节，小于64MB直接发送
		x = 126
		data = append(data, byte(x + 128))

		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, length)

		data = append(data, b[6:]...)
	} else if length >= 65536 {	// 大于等于64MB，小于16777216T直接发送
		x = 127
		data = append(data, byte(x + 128))

		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, length)

		data = append(data, b...)
	}
	// -------------------
	maskOctet := []byte(s)
	var i, j int
	for ;i < len(maskOctet);i++ {
		j = i % 4
		maskOctet[i] = maskOctet[i] ^ maskingKey[j]
	}
	data = append(data, maskingKey...)	// 加上Masking-Key
	data = append(data, maskOctet...)	// 加上数据
	if Opcode & 0x02 == 1 {
		data = append([]byte{}, []byte(biu.BytesToBinaryString(data))...)
	}

	return data
}

// 根据websocket协议对数据进行解封
func ParseProtocol(data []byte) string {
	PayloadLength := len(data)
	if PayloadLength < 3 {
		return ""
	} else if PayloadLength >= 3 && PayloadLength <= 127 {
		return string(data[2:PayloadLength])
	} else if PayloadLength >= 130 && PayloadLength <= 65539 {
		return string(data[4:PayloadLength])
	} else if PayloadLength >= 65544 {
		return string(data[10:PayloadLength])
	}
	return ""
}

func (w *WebSocketClient) Send(v interface{}) error {
	data := EnProtoc(v)	// 将数据封装成websocket数据包
	_, err := w.conn.Write(data)
	return err
}

func (w *WebSocketClient) Read() (string, error) {
	buf := make([]byte, 1024)
	n, err := w.conn.Read(buf)

	return ParseProtocol(buf[:n]), err
}

func generateMaskingKey() (maskingKey []byte, err error) {
	maskingKey = make([]byte, 4)
	if _, err = io.ReadFull(rand.Reader, maskingKey); err != nil {
		return
	}
	return
}

func (w *WebSocketClient) Close() {
	closeData := []byte{0x88, 0x80}
	maskingKey, _ := generateMaskingKey()

	closeData = append(closeData, maskingKey...)
	w.conn.Write(closeData)
	if w.conn != nil {
		w.conn.Close()
	}
}


