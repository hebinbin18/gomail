package gomail_test

import (
	"github.com/hebinbin18/gomail"
	"testing"
	"fmt"
)

// go test -v example_test.go -test.run Mail
func TestMail(t *testing.T) {
	m := gomail.NewClient()
	m.SetDebug()
	m.SetTLS()
	m.SetHost("smtp.163.com", 465)
	m.SetAuth("go_mail_test@163.com", "gomailtest123")
	m.SetFromName("hebin")
	//m.SetReplyAddr("4171947@qq.com")
	//m.SetReplyName("Reply Name")
	m.SetSubject("this is subject")

	m.AddAddress("hebin@zouke.com")
	//m.AddCC("go_mail_test@163.com")
	//m.AddBCC("43171947@qq.com")

	m.SetHtmlMail()
	m.SetMailContent("this is mail content !")
	//m.AddAttachment("/test.jpg", "test.jpg")

	err := m.SendMail()
	fmt.Println("Test Result:", err)
}
