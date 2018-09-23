package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
)

func main() {
	// 1）监听TCP端口
	ln, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Panic(err)
	}
	// 2）处理客户端接入
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Accept err: ", err)
		}
		go func() {
			handleConnection(conn)
			ws := WsSocket{Conn: conn}
			for {
				d, err := ws.Read()
				if err != nil {
					log.Println(err)
					break
				}
				fmt.Println("接收到客户端的数据为:", string(d))
				err = ws.Send(d)
				if err != nil {
					log.Println(err)
					break
				}
			}
		}()
	}
}

// 处理客户连接
func handleConnection(conn net.Conn) {
	content := make([]byte, 1024)
	_, err := conn.Read(content) // 读取客户端发送过来的数据

	if err != nil {
		log.Println(err)
	}
	// 判断是否为HTTP请求
	isHttp := false
	if string(content[0:3]) == "GET" {
		isHttp = true
	}

	if isHttp {
		headers := parseHandshake(string(content)) // 解析请求头
		secWebsocketKey := headers["Sec-WebSocket-Key"]

		// 先不处理其他验证
		guid := "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

		// 计算Sec-WebSocket-Accept
		h := sha1.New()

		io.WriteString(h, secWebsocketKey+guid)
		accept := make([]byte, 28)
		base64.StdEncoding.Encode(accept, h.Sum(nil))

		var responseBuf bytes.Buffer
		responseBuf.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
		responseBuf.WriteString("Sec-WebSocket-Accept: " + string(accept) + "\r\n")
		responseBuf.WriteString("Connection: Upgrade\r\n")
		responseBuf.WriteString("Upgrade: websocket\r\n\r\n")

		if _, err := conn.Write(responseBuf.Bytes()); err != nil {
			log.Println(err)
		}
	}
}

type frameServerHeader struct {
	Fin           bool
	Rsv           [3]bool
	Opcode        int
	Mask          bool
	PayloadLength int
}

type WsSocket struct {
	MaskingKey []byte   // 掩码值
	Conn       net.Conn // 连接
}

func NewWsSocket(conn net.Conn) *WsSocket {
	return &WsSocket{Conn: conn}
}

// 发送
func (this *WsSocket) Send(data []byte) error {
	// 这里只处理data长度<125的
	if len(data) >= 125 {
		return errors.New("send iframe data error")
	}
	headerFrame := make([]byte, 2)
	headerFrame[0] = headerFrame[0] | 0x81 // 10000001 以文本的形式发送数据
	headerFrame[1] = headerFrame[1] | byte(len(data))

	_, err := this.Conn.Write(append(headerFrame, []byte(data)...))
	return err
}

// 读取
func (this *WsSocket) Read() (data []byte, err error) {
	buf := make([]byte, 1024)
	n, err := this.Conn.Read(buf)
	if n < 7 { // 客户端发送的websocket包长度至少为7，1+1+4+1
		return buf[:n], errors.New("bad data1")
	}
	fmt.Println(buf[1] & 0x80)
	if buf[1]&0x80 != 0x80 { // mask没有设置掩码值
		return buf[:n], errors.New("bad data2")
	}
	length := buf[1] & 0x7f // 0111 1111
	if length < 126 {
		maskingKey := buf[2:6] // mask key
		data = buf[6:]
		i, j := 0, 0
		for ; i < int(length); i++ {
			j = i % 4
			data[i] = data[i] ^ maskingKey[j]
		} // 解码
		data = data[:length]
	} else {

	}
	return
}

// 解析握手
func parseHandshake(content string) map[string]string {
	headers := make(map[string]string, 0)
	lines := strings.Split(content, "\r\n")

	for _, line := range lines {
		if len(line) > 0 {
			words := strings.Split(line, ":")
			if len(words) == 2 {
				headers[strings.TrimSpace(words[0])] = strings.TrimSpace(words[1])
			}
		}
	}
	return headers
}
