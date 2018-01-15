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
	m.SetSSl()
	m.SetHost("smtp.163.com", 465)
	m.SetAuth("go_mail_test@163.com", "gomailtest123")

	m.SetFromName("hebin")
	m.SetReplyAddr("4171947@qq.com")
	//m.SetReplyName("re name")
	m.SetSubject("时间")

	m.AddAddress("hebin@zouke.com")
	//m.AddCC("go_mail_test@163.com")
	//m.AddCC("43171947@qq.com")

	m.SetHtmlMail()
	m.SetMailContent("今天下午见")
	//m.AddAttachment("/test.jpg", "test.jpg")

	err := m.SendMail()
	fmt.Println("Test Result:", err)
}
