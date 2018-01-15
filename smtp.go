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
	"mime"
	"bytes"
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

func (c *goSMTPConn) bEncode(src string) string {
	dst := mime.WordEncoder.Encode(mime.BEncoding, "utf-8", src)

	return dst
}

func (c *goSMTPConn) base64Encode(src []byte) string {
	dst := base64.StdEncoding.EncodeToString(src)

	return dst
}

func (c *goSMTPConn) md5Encode(src string) string {
	h := md5.New()
	h.Write([]byte(src))
	dst := hex.EncodeToString(h.Sum(nil))

	return dst
}

func (c *goSMTPConn) addrEncode(name, addr string) string {
	if "" == name {
		name = strings.Split(addr, "@")[0]
	}
	dst := fmt.Sprintf(`"=?utf-8?b?%s?="<%s>`, c.base64Encode([]byte(name)), addr)

	return dst
}

func (c *goSMTPConn) sendData(dat string) error {
	dat += CRLF
	cnt, err := c.conn.Write([]byte(dat))
	if c.debug && cnt < 1000 {
		fmt.Println("==> GoMail Send Data Content:", dat)
	}
	if err != nil {
		fmt.Println("==> GoMail Send Error:", err.Error())
		return err
	}

	return nil
}

func (c *goSMTPConn) recvData() (string, error) {
	buf := make([]byte, 1024) // 515
	_, err := c.conn.Read(buf)
	data := string(buf)
	if c.debug {
		fmt.Println("<== GoMail Recv Data Content:", data)
	}
	if err != nil {
		fmt.Println("<== GoMail Recv Error:", err.Error())
		return "", err
	}

	return data[0:3], nil
}

func (c *goSMTPConn) echoCMD(cmd string) (string, error) {
	err := c.sendData(cmd)
	if nil != err {
		return "", err
	}

	return c.recvData()
}

// http://tools.ietf.org/html/rfc4021
func (c *goSMTPConn) createHeader() string {
	c.uniqueId = c.md5Encode(time.Now().String())
	c.boundary1 = "----=BOUNDARY_1_" + c.uniqueId
	c.boundary2 = "----=BOUNDARY_2_" + c.uniqueId

	// 邮件头
	buf := new(bytes.Buffer)
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + LF)
	//buf.WriteString("Return-Path: " + c.bEncode(c.mClient.fromName) + LF)
	addrList1 := make([]string, len(c.mClient.addressSend))
	addrList2 := make([]string, len(c.mClient.addressCC))
	addrList3 := make([]string, len(c.mClient.addressBCC))
	for i, addr := range c.mClient.addressSend {
		addrList1[i] = c.addrEncode("", addr)
	}
	for i, addr := range c.mClient.addressCC {
		addrList2[i] = c.addrEncode("", addr)
	}
	for i, addr := range c.mClient.addressBCC {
		addrList3[i] = c.addrEncode("", addr)
	}
	buf.WriteString("To: " + strings.Join(addrList1, ";") + LF)
	if len(addrList2) > 0 {
		buf.WriteString("Cc: " + strings.Join(addrList2, ";") + LF)
	}
	if len(addrList3) > 0 {
		buf.WriteString("Bcc: " + strings.Join(addrList3, ";") + LF)
	}
	buf.WriteString("From: " + c.addrEncode(c.mClient.fromName, c.mClient.fromAddr) + LF)
	if "" != c.mClient.replyAddr {
		buf.WriteString("Reply-to: " + c.addrEncode(c.mClient.replyName, c.mClient.replyAddr) + LF)
	}

	// 主题信息
	buf.WriteString("Subject: " + c.bEncode(c.mClient.subject) + LF)
	buf.WriteString(fmt.Sprintf("Message-ID: <54c3aee5da3b50b47a9ee09defb8c00e@%s>", c.mClient.getServerHostName()) + LF)
	buf.WriteString("X-Priority: " + c.mClient.priority + LF)
	buf.WriteString("GoMail 1.0.0" + LF)
	//buf.WriteString(fmt.Sprintf("Disposition-Notification-To: <%s>", c.mClient.fromAddr) +  LF)
	buf.WriteString("Mime-Version: 1.0" + LF)

	// 正文
	buf.WriteString("Content-Type: multipart/alternative;" + LF)
	buf.WriteString(TAB + `boundary="` + c.boundary1 + `"` + LF)
	buf.WriteString(LF)
	if "html" == c.mClient.mailType {
		buf.WriteString("--" + c.boundary1 + LF)
		buf.WriteString("Content-Type: text/html;" + LF)
		buf.WriteString(TAB + `charset="utf-8"` + LF)
		buf.WriteString("Content-Transfer-Encoding: base64" + LF)
		buf.WriteString(LF)
		buf.WriteString(c.base64Encode(c.mClient.getContent()) + LF)
		buf.WriteString(LF)
	} else {
		buf.WriteString("--" + c.boundary1 + LF)
		buf.WriteString("Content-Type: text/plain;" + LF)
		buf.WriteString(TAB + `charset="utf-8"` + LF)
		buf.WriteString("Content-Transfer-Encoding: base64" + LF)
		buf.WriteString(LF)
		buf.WriteString(c.base64Encode(c.mClient.getContent()) + LF)
		buf.WriteString(LF)
	}
	buf.WriteString("--" + c.boundary1 + "--" + LF)

	return buf.String()
}

func (c *goSMTPConn) getAttachments() string {
	if 0 == len(c.mClient.attachments) {
		return ""
	}

	text := ""
	for _, file := range c.mClient.attachments {
		path := file["path"]
		name := fmt.Sprintf("%s", c.bEncode(file["name"]))
		text += c.boundary1 + LF
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
			fBufferB64 = c.base64Encode(fBuffer)
		} else {
			fi, err := os.Open(path)
			if err != nil {
				panic(err)
			}
			fBuffer, err := ioutil.ReadAll(fi)
			fBufferB64 = c.base64Encode(fBuffer)
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
	text += fmt.Sprintf("%s--", c.boundary1)

	return text
}

func (c *goSMTPConn) authenticate() error {
	var err error
	var code string

	// Step 1
	err = c.sendData("AUTH LOGIN")
	if err != nil {
		return err
	}
	for "334" != code {
		code, err = c.recvData()
		if err != nil {
			return err
		}
	}
	// Step 2
	name := c.base64Encode([]byte(c.mClient.getUserName()))
	code, err = c.echoCMD(name)
	if err != nil {
		return err
	}
	if "334" != code {
		return errors.New("UserName Authenticate Failure " + code)
	}

	// Step 3
	password := c.base64Encode([]byte(c.mClient.getPassword()))
	code, err = c.echoCMD(password)
	if err != nil {
		return err
	}
	if "235" != code {
		return errors.New("PassWord Authenticate Failure " + code)
	}

	return nil
}

// TCP connect to Mail Server
func (c *goSMTPConn) dial() error {
	var err error
	var code string

	if c.mClient.SSL {
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
	_, err = c.recvData()
	if nil != err {
		return err
	}
	if c.mClient.SSL {

		//code, err = c.echoCMD("STARTTLS")
		//if nil != err {
		//	return err
		//}
		//if "220" != code {
		//	return errors.New("STARTTLS Return not 220 , Is " + code)
		//}
	}

	// Step 2 Send extended hello first (RFC 821)
	host := c.mClient.getServerHostName()
	code, err = c.echoCMD("EHLO" + " " + host)
	code, err = c.echoCMD("HELO" + " " + host)
	if nil != err {
		return err
	}
	if "250" != code {
		return errors.New("HELO Return not 250 , Is " + code)
	}

	return nil
}

func (c *goSMTPConn) sendContent() error {
	var err error
	var code string

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

	// Step 3 正文开始
	code, err = c.echoCMD("DATA")
	if nil != err {
		return err
	}
	if "354" != code {
		fmt.Println("------send 2", code)
	}

	// Step 4 开始发送正文 RFC 821
	content := c.createHeader() + c.getAttachments() + CRLF + "."
	code, err = c.echoCMD(content)
	if nil != err {
		return err
	}
	// 250 为 queued
	if "250" != code {
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
