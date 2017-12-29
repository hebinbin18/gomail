package gomail

import (
	"crypto/tls"
	"encoding/base64"
	"net"
	"time"
	"fmt"
	"errors"
)

type smtpClient struct {
	debug bool
	error string
	conn  net.Conn
	proxy *client
}

func newMailClient(proxy *client) *smtpClient {
	client := smtpClient{}
	client.proxy = proxy
	client.debug = proxy.debug

	return &client
}

func (c *smtpClient) SetDebug() {

}

func (c *smtpClient) getReplies() (code, replyData string) {
	data := make([]byte, 728) // 515
	cnt, err := c.conn.Read(data)
	if err != nil {
		panic(err)
	}
	replyStr := string(data)
	if c.debug {
		fmt.Println("*HOST:", cnt)
		fmt.Println(replyStr)
	}

	return replyStr[0:3], replyStr
}

func (c *smtpClient) sendCMD(data string) bool {
	data += CRLF
	cnt, err := c.conn.Write([]byte(data))
	if err != nil {
		panic(err)
	}
	if c.debug {
		fmt.Println("*Client:", cnt)
		fmt.Println(data)
	}

	return true
}

func (c *smtpClient) sendHello(hello, host string) bool {
	hello += " " + host
	c.sendCMD(hello)
	code, _ := c.getReplies()
	if "250" == code {
		return true
	}

	return false
}

func (c *smtpClient) authenticate() bool {
	data := "AUTH fmtIN"
	c.sendCMD(data)
	code, _ := c.getReplies()
	if "334" != code && "250" != code {
		fmt.Println(code, "---atuh 1")
		//return false
	}
	name := base64.StdEncoding.EncodeToString([]byte(c.proxy.getUserName()))
	c.sendCMD(name)
	code, _ = c.getReplies()
	if "334" != code {
		fmt.Println(code, "---atuh 2")
		//return false
	}
	password := base64.StdEncoding.EncodeToString([]byte(c.proxy.getPassword()))
	c.sendCMD(password)
	code, _ = c.getReplies()
	if "235" != code && "334" != code {
		fmt.Println(code, "---atuh 3")
		//return false
	}

	return true
}

// TCP connect to Mail Server
func (c *smtpClient) connectHost() bool {
	if c.proxy.tls {
		host, _, _ := net.SplitHostPort(c.proxy.getHostPort())
		c.conn, _ = tls.Dial("tcp", c.proxy.getHostPort(), &tls.Config{})

		data := "STARTTLS"
		c.sendCMD(data)
		code, _ := c.getReplies()
		if "220" != code {
			return false
		}
		// Send extended hello first (RFC 821)
		if !c.sendHello("EHLO", host) {
			if !c.sendHello("HELO", host) {
				panic("Send Hello Error")
				return false
			}
		}
		return true
	}

	conn, err := net.Dial("tcp", c.proxy.getHostPort())
	if err != nil {
		panic(err)
	}
	conn.SetWriteDeadline(time.Now().Add(time.Second * 30))
	conn.SetReadDeadline(time.Now().Add(time.Second * 30))
	c.conn = conn
	c.getReplies()

	// Send extended hello first (RFC 821)
	host := c.proxy.getServerHostName()
	if !c.sendHello("EHLO", host) {
		if !c.sendHello("HELO", host) {
			panic("Send Hello Error")
			return false
		}
	}

	return true
}

func (c *smtpClient) send() bool {
	// TODO TLS 发送存在BUG 发送成功 return 为false
	from := c.proxy.getFromAddr()
	sendTo := c.proxy.getSendTo()
	c.sendCMD("MAIL FROM:<" + from + ">")
	code, _ := c.getReplies()
	if "250" != code && "235" != code {
		fmt.Println("------send 1", code)
		//return false
	}

	//收件人列表
	for _, to := range sendTo {
		c.sendCMD("RCPT TO:<" + to + ">")
		code, _ = c.getReplies()
		if "250" != code && "251" != code {
			fmt.Println(to)
		}
	}

	c.sendCMD("DATA")
	code, _ = c.getReplies()
	if "354" != code {
		fmt.Println("------send 2", code)
		//return false
	}
	c.sendCMD(string(c.proxy.getMailContent()))
	c.sendCMD(CRLF + ".")
	code, _ = c.getReplies()

	// 250 为 queued
	if "354" != code && "250" != code {
		fmt.Println("------send 3", code)
		//return false
	}

	return true
}

// 发送邮件
func (c *smtpClient) SendMail() error {
	if false == c.connectHost() {
		return errors.New("邮件服务器网络连接错误")
	}
	if false == c.authenticate() {
		return errors.New("登录鉴权失败")
	}
	if false == c.send() {
		return errors.New("邮件服务器发送失败")
	}

	return nil
}
