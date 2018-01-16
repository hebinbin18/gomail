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
	debug   bool
	conn    net.Conn
	mClient *client
}

func (c *goSMTPConn) bEncode(src string) string {
	dst := mime.WordEncoder.Encode(mime.BEncoding, "utf-8", src)

	return dst
}

func (c *goSMTPConn) base64Encode(src []byte) string {
	dst := base64.StdEncoding.EncodeToString(src)

	return dst
}

func (c *goSMTPConn) getBoundary(key string) string {
	h := md5.New()
	h.Write([]byte(time.Now().String()))
	dst := hex.EncodeToString(h.Sum(nil))

	return "----=BOUNDARY_" + key + "_" + dst
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
	//fmt.Println("==> GoMail Send Data Content:", dat, cnt)
	if c.debug && cnt < 1000 {
		fmt.Println("==> GoMail Send Data Content:", dat)
	}
	if c.debug && cnt > 1000 {
		fmt.Println("==> GoMail Send Data Content:", dat[0:700], LF, CRLF, " . . . ", LF, LF, dat[cnt-200:])
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

	return data, nil
}

func (c *goSMTPConn) echoCMD(cmd string) (string, error) {
	err := c.sendData(cmd)
	if nil != err {
		return "", err
	}
	data, err := c.recvData()
	if nil != err {
		return "", err
	}

	return data[0:3], err
}

func (c *goSMTPConn) addMailAddress(buf *bytes.Buffer) {
	// 邮件头
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + LF)
	addrList1 := make([]string, len(c.mClient.address))
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
	buf.WriteString("To: " + strings.Join(addrList1, ", ") + LF)
	if len(addrList2) > 0 {
		buf.WriteString("Cc: " + strings.Join(addrList2, ", ") + LF)
	}
	if len(addrList3) > 0 {
		buf.WriteString("Bcc: " + strings.Join(addrList3, ", ") + LF)
	}
	buf.WriteString("From: " + c.addrEncode(c.mClient.fromName, c.mClient.fromAddr) + LF)
	if "" != c.mClient.replyAddr {
		buf.WriteString("Reply-to: " + c.addrEncode(c.mClient.replyName, c.mClient.replyAddr) + LF)
	}
}

// http://tools.ietf.org/html/rfc4021
func (c *goSMTPConn) addMailHeader(buf *bytes.Buffer, boundary string) {
	// 主题信息
	buf.WriteString("Subject: " + c.bEncode(c.mClient.subject) + LF)
	buf.WriteString(fmt.Sprintf("Message-ID: <54c3aee5da3b50b47a9ee09defb8c00e@%s>", c.mClient.getServerHostName()) + LF)
	buf.WriteString("X-Priority: " + c.mClient.priority + LF)
	buf.WriteString("X-Mailer: GoMail 1.0.0" + LF)
	// 已读回执
	if c.mClient.notification {
		buf.WriteString("Disposition-Notification-To: " + c.addrEncode(c.mClient.fromName, c.mClient.fromAddr) + LF)
	}
	buf.WriteString("Mime-Version: 1.0" + LF)
	if len(c.mClient.attachments) > 0 {
		buf.WriteString("Content-Type: multipart/mixed;" + LF)
		buf.WriteString(TAB + `boundary="` + boundary + `"` + LF)
		buf.WriteString(LF)
		buf.WriteString("This is a multi-part message in MIME format." + LF)
		buf.WriteString(LF)
		buf.WriteString("--" + boundary + LF)
	}
}

func (c *goSMTPConn) addMailBody(buf *bytes.Buffer) {
	// 正文
	boundary := c.getBoundary("body")
	buf.WriteString("Content-Type: multipart/alternative;" + LF)
	buf.WriteString(TAB + `boundary="` + boundary + `"` + LF)
	buf.WriteString(LF)

	cType := "plain"
	if "html" == c.mClient.mailType {
		cType = "html"
	}
	buf.WriteString("--" + boundary + LF)
	buf.WriteString("Content-Type: text/" + cType + ";" + LF)
	buf.WriteString(TAB + `charset="utf-8"` + LF)
	buf.WriteString("Content-Transfer-Encoding: base64" + LF)
	buf.WriteString(LF)
	buf.WriteString(c.base64Encode(c.mClient.getContent()) + LF)
	buf.WriteString(LF)
	buf.WriteString("--" + boundary + "--" + LF)
}

func (c *goSMTPConn) addMailAttachments(buf *bytes.Buffer, boundary string) {
	if 0 == len(c.mClient.attachments) {
		return
	}
	for _, file := range c.mClient.attachments {
		path := file["path"]
		buf.WriteString(LF + "--" + boundary + LF)
		buf.WriteString("Content-Type: application/octet-stream;" + LF)
		buf.WriteString(TAB + fmt.Sprintf(`name="%s";`, c.bEncode(file["name"])) + LF)
		buf.WriteString("Content-Disposition: attachment;" + LF)
		buf.WriteString(TAB + fmt.Sprintf(`filename="%s"`, c.bEncode(file["name"])) + LF)
		buf.WriteString("Content-Transfer-Encoding: base64" + LF + LF)

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
		buf.WriteString(fB64LF)
		buf.WriteString(LF + LF)
	}
	buf.WriteString("--" + boundary + "--" + LF)
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
		data, err := c.recvData()
		if err != nil {
			return err
		}
		code = data[0:3]
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
		return errors.New("CMD DATA return code not 354 is " + code)
	}

	// Step 4 开始发送正文 RFC 821
	boundary := c.getBoundary("mixed")
	buf := new(bytes.Buffer)
	c.addMailAddress(buf)
	c.addMailHeader(buf, boundary)
	c.addMailBody(buf)
	c.addMailAttachments(buf, boundary)
	buf.WriteString(CRLF + ".")
	err = c.sendData(buf.String())
	if nil != err {
		return err
	}
	data, err := c.recvData()
	if nil != err {
		return err
	}
	// 250 为 queued
	if "250" != data[0:3] {
		return errors.New("Mail Send return: " + data)
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
