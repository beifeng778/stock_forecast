package mail

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"
)

// SendMail 发送邮件
func SendMail(to, subject, body string) error {
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")

	if host == "" || user == "" || pass == "" {
		return fmt.Errorf("邮件配置不完整，请检查 SMTP_HOST, SMTP_USER, SMTP_PASS")
	}

	port := 465
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// 构建邮件内容
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		user, to, subject, body)

	// 使用 TLS 连接（163邮箱需要SSL）
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", host, port), tlsConfig)
	if err != nil {
		return fmt.Errorf("连接邮件服务器失败: %v", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %v", err)
	}
	defer client.Close()

	// 认证
	auth := smtp.PlainAuth("", user, pass, host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("邮件认证失败: %v", err)
	}

	// 设置发件人和收件人
	if err := client.Mail(user); err != nil {
		return fmt.Errorf("设置发件人失败: %v", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("设置收件人失败: %v", err)
	}

	// 发送邮件内容
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取写入器失败: %v", err)
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %v", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("关闭写入器失败: %v", err)
	}

	return client.Quit()
}

// SendInviteCodeNotification 发送验证码更新通知（支持多个邮箱，用逗号分隔）
func SendInviteCodeNotification(newCode string) error {
	emails := os.Getenv("NOTIFY_EMAILS")
	if emails == "" {
		return fmt.Errorf("未配置通知邮箱 NOTIFY_EMAILS")
	}

	subject := "【股票预测系统】验证码已更新"
	body := fmt.Sprintf(`
		<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
			<h2 style="color: #10b981;">股票预测系统 - 验证码更新通知</h2>
			<p>您的系统验证码已自动更新，新的验证码为：</p>
			<div style="background: #1e293b; color: #10b981; padding: 20px; border-radius: 8px; text-align: center; font-size: 24px; font-family: monospace; letter-spacing: 2px;">
				%s
			</div>
			<p style="color: #64748b; font-size: 12px; margin-top: 20px;">
				此邮件由系统自动发送，请勿回复。
			</p>
		</div>
	`, newCode)

	// 支持多个邮箱，用逗号分隔
	emailList := strings.Split(emails, ",")
	var lastErr error
	for _, email := range emailList {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		if err := SendMail(email, subject, body); err != nil {
			lastErr = err
			fmt.Printf("发送邮件到 %s 失败: %v\n", email, err)
		} else {
			fmt.Printf("验证码通知已发送到 %s\n", email)
		}
	}
	return lastErr
}
