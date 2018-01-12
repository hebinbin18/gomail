package gomail

import (
	"crypto/tls"
	"encoding/base64"
	"net"
	"fmt"
	"errors"
	"time"
	"crypto/md5"
	"encoding/hex"
	"strings"
	"os"
	"net/http"
	"io/ioutil"
)

const (
	CRLF = "\r\n"
	LF   = "\n"
	TAB  = "\t"
)

type goSMTPConn struct {
	debug     bool
	uniqueId  string
	boundary1 string
	boundary2 string
	conn      net.Conn
	mClient   *client
}

func (c *goSMTPConn) base64Encode(src string) string {
	dst := base64.StdEncoding.EncodeToString([]byte(src))

	return dst
}

func (c *goSMTPConn) md5Encode(src string) string {
	h := md5.New()
	h.Write([]byte(src))
	dst := hex.EncodeToString(h.Sum(nil))

	return dst
}

func (c *goSMTPConn) getReply() (string, string, error) {
	buf := make([]byte, 728) // 515
	cnt, err := c.conn.Read(buf)
	data := string(buf)
	if c.debug {
		fmt.Println("GoMail Recv Data Length:", cnt)
		fmt.Println("GoMail Recv Data Content:")
		fmt.Println(data)
	}
	if err != nil {
		fmt.Println("GoMail Recv Error:", err.Error())
		return "", "", err
	}

	return data[0:3], data, nil
}

func (c *goSMTPConn) sendCMD(data string) error {
	data += CRLF
	cnt, err := c.conn.Write([]byte(data))
	if c.debug {
		fmt.Println("GoMail Send Data Length:", cnt)
	}
	if c.debug && cnt < 1000 {
		fmt.Println("GoMail Send Data Content:")
		fmt.Println(data)
	}
	if err != nil {
		fmt.Println("GoMail Send Error:", err.Error())
		return err
	}

	return nil
}

func (c *goSMTPConn) echoCMD(data string) (string, error) {
	if err := c.sendCMD(data); nil != err {
		return "", err
	}
	code, _, err := c.getReply()

	return code, err
}

// 真正的邮件内容: RFC 821 发送DATA以后的数据
func (c *goSMTPConn) getMailContent() string {
	msg := c.createHeader() + c.getAttachments()

	return msg
}

func (c *goSMTPConn) getSendToString() string {
	send := "To: " + strings.Join(c.mClient.addressSend, ";") + LF
	cc, bcc := "", ""
	if len(c.mClient.addressCC) > 0 {
		cc = "Cc: " + strings.Join(c.mClient.addressCC, ";") + LF
	} else {
		cc = ""
	}

	if len(c.mClient.addressBCC) > 0 {
		bcc = "Bcc: " + strings.Join(c.mClient.addressBCC, ";") + LF
	} else {
		bcc = ""
	}

	if "" == c.mClient.fromName {
		c.mClient.fromName = c.mClient.fromAddr
	}
	from := fmt.Sprintf("From: =?utf-8?B?%s?= <%s>%s", base64.StdEncoding.EncodeToString([]byte(c.mClient.fromName)), c.mClient.fromAddr, LF)
	reply := fmt.Sprintf("Reply-to: =?utf-8?B?%s?= <%s>%s", base64.StdEncoding.EncodeToString([]byte(c.mClient.replyName)), c.mClient.replyAddr, LF)

	return send + cc + bcc + from + reply
}

// http://tools.ietf.org/html/rfc4021
func (c *goSMTPConn) createHeader() string {
	c.uniqueId = c.md5Encode(time.Now().String())
	c.boundary1 = "b1_" + c.uniqueId
	c.boundary2 = "b2_" + c.uniqueId

	// 邮件头
	header := "Date: " + time.Now().Format(time.RFC1123Z) + LF
	header += "Return-Path: "
	if "" != c.mClient.fromName {
		header += c.mClient.fromName + LF
	} else {
		header += c.mClient.fromAddr + LF
	}

	// 收件人信息
	header += c.getSendToString()

	// 主题信息
	subject := fmt.Sprintf("Subject: =?utf-8?B?%s?=%s", base64.StdEncoding.EncodeToString([]byte(c.mClient.subject)), LF)
	subject += fmt.Sprintf("Message-ID: <54c3aee5da3b50b47a9ee09defb8c00e@%s>%s", c.mClient.getServerHostName(), LF)
	subject += "X-Priority: " + c.mClient.priority + LF
	subject += "X-Mailer: GoLang (phpmailer.sourceforge.net)" + LF
	//	subject += fmt.Sprintf("Disposition-Notification-To: <%s>%s", c.mClient.from.mailAddr, LF)
	subject += "MIME-Version: 1.0" + LF
	header += subject

	// mime type + content
	mime := ""
	if len(c.mClient.attachments) > 0 {
		mime += "Content-Type: multipart/mixed;" + LF
		mime += fmt.Sprintf(`%sboundary="%s"%s%s%s`, TAB, c.boundary1, LF, LF, LF)
		mime += fmt.Sprintf(`--%s%s`, c.boundary1, LF)
		mime += fmt.Sprintf(`Content-Type: multipart/alternative;%s%sboundary="%s"%s%s`, LF, TAB, c.boundary2, LF, LF)

		mime += "--" + c.boundary2 + LF
		mime += `Content-Type: text/plain; charset = "utf-8"` + LF
		mime += "Content-Transfer-Encoding: base64" + LF
		mime += LF + LF
		mime += base64.StdEncoding.EncodeToString([]byte("text/html")) + LF
		mime += LF + LF
		mime += "--" + c.boundary2 + LF
		mime += `Content-Type: text/html; charset = "utf-8"` + LF
		mime += "Content-Transfer-Encoding: base64" + LF
		mime += LF + LF
		mime += c.base64Encode(c.mClient.getContent()) + LF
		mime += LF + LF
		mime += "--" + c.boundary2 + "--" + LF
	} else {
		mime += fmt.Sprintf(`Content-Type: multipart/alternative;%s%sboundary="%s"%s%s`, LF, TAB, c.boundary1, LF, LF)

		mime += "--" + c.boundary1 + LF
		mime += `Content-Type: text/plain; charset = "utf-8"` + LF
		mime += "Content-Transfer-Encoding: base64" + LF
		mime += LF + LF
		mime += base64.StdEncoding.EncodeToString([]byte("text/html")) + LF
		mime += LF + LF
		mime += "--" + c.boundary1 + LF
		mime += `Content-Type: text/html; charset = "utf-8"` + LF
		mime += "Content-Transfer-Encoding: base64" + LF
		mime += LF + LF
		mime += c.base64Encode(c.mClient.getContent()) + LF
		mime += LF + LF
		mime += "--" + c.boundary1 + "--" + LF
	}
	header += mime

	return header
}

func (c *goSMTPConn) getAttachments() string {
	if 0 == len(c.mClient.attachments) {
		return ""
	}

	text := ""
	for _, file := range c.mClient.attachments {
		path := file["path"]
		name := fmt.Sprintf("=?utf-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(file["name"])))
		text += "--" + c.boundary1 + LF
		text += fmt.Sprintf(`Content-Type: application/octet-stream;%s%scharset="utf-8";%s%sname="%s"%s`, LF, TAB, LF, TAB, name, LF)
		text += fmt.Sprintf(`Content-Disposition: attachment; filename="%s"%s`, name, LF)
		text += fmt.Sprintf("Content-Transfer-Encoding: base64%s%s", LF, LF)

		fBufferB64 := ""
		if strings.HasPrefix(path, "http") {
			resp, err := http.Get(path)
			if err != nil {
				resp.Body.Close()
				panic("fetch attachments " + path + " error")
			}
			fBuffer, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				panic("fetch attachments " + path + " error2")
			}
			fBufferB64 = base64.StdEncoding.EncodeToString(fBuffer)
		} else {
			fi, err := os.Open(path)
			if err != nil {
				panic(err)
			}
			fBuffer, err := ioutil.ReadAll(fi)
			fBufferB64 = base64.StdEncoding.EncodeToString(fBuffer)
			fi.Close()
		}

		fB64LF := ""
		fLen := len(fBufferB64)
		for i := 0; i < fLen; i += 72 {
			end := i + 72
			if end > fLen {
				end = fLen
			}

			fB64LF += fBufferB64[i:end] + LF
		}
		text += fB64LF
		text += LF + LF
	}
	text += fmt.Sprintf("--%s--%s", c.boundary1, LF)

	return text
}

func (c *goSMTPConn) authenticate() error {
	var err error
	var code string

	// Step 1
	code, err = c.echoCMD("AUTH fmtIN")
	if err != nil {
		return err
	}
	if "334" != code && "250" != code {
		return errors.New("AUTH fmtIN Authenticate Failure " + code)
	}

	// Step 2
	name := c.base64Encode(c.mClient.getUserName())
	code, err = c.echoCMD(name)
	if err != nil {
		return err
	}
	if "334" != code {
		return errors.New("UserName Authenticate Failure " + code)
	}

	// Step 3
	password := c.base64Encode(c.mClient.getPassword())
	code, err = c.echoCMD(password)
	if err != nil {
		return err
	}
	if "235" != code && "334" != code {
		return errors.New("PassWord Authenticate Failure " + code)
	}

	return nil
}

// TCP connect to Mail Server
func (c *goSMTPConn) dial() error {
	var err error
	var code string

	if c.mClient.tls {
		c.conn, err = tls.Dial("tcp", c.mClient.getHostPort(), &tls.Config{})
	} else {
		c.conn, err = net.Dial("tcp", c.mClient.getHostPort())
	}
	if err != nil {
		return err
	}
	c.conn.SetWriteDeadline(time.Now().Add(time.Second * 30))
	c.conn.SetReadDeadline(time.Now().Add(time.Second * 30))

	// Step 1
	code, err = c.echoCMD("STARTTLS")
	if nil != err {
		return err
	}
	if "220" != code {
		return errors.New("STARTTLS Return not 220 , Is " + code)
	}

	// Step 2 Send extended hello first (RFC 821)
	host := c.mClient.getServerHostName()
	code, err = c.echoCMD("EHLO" + " " + host)
	if nil != err {
		return err
	}
	if "250" != code {
		return errors.New("EHLO Return not 250 , Is " + code)
	}

	return nil
}

func (c *goSMTPConn) sendContent() error {
	var err error
	var code string

	// TODO TLS 发送存在BUG 发送成功 return 为false
	// Step 1 发件人地址
	from := c.mClient.getFromAddr()
	code, err = c.echoCMD("MAIL FROM:<" + from + ">")
	if nil != err {
		return err
	}
	if "250" != code && "235" != code {
		fmt.Println("------send 1", code)
	}

	// Step 2 收件人地址
	sendTo := c.mClient.getSendTo()
	for _, to := range sendTo {
		code, err = c.echoCMD("RCPT TO:<" + to + ">")
		if nil != err {
			return err
		}
		if "250" != code && "251" != code {
			fmt.Println(to)
		}
	}

	// Step 3 发送正文
	code, err = c.echoCMD("DATA")
	if nil != err {
		return err
	}
	if "354" != code {
		fmt.Println("------send 2", code)
	}
	// 开始发送正文
	c.sendCMD(c.getMailContent())

	// Step 4
	code, err = c.echoCMD(CRLF + ".")
	if nil != err {
		return err
	}
	// 250 为 queued
	if "354" != code && "250" != code {
		fmt.Println("------send 3", code)
	}

	return nil
}

// 发送邮件
func (c *goSMTPConn) Send() error {
	if nil != c.dial() {
		return errors.New("邮件服务器网络连接错误")
	}
	if nil != c.authenticate() {
		return errors.New("登录鉴权失败")
	}
	if nil != c.sendContent() {
		return errors.New("邮件服务器发送失败")
	}

	return nil
}
